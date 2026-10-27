package store

import (
	"database/sql"
	"errors"
	"time"
)

var (
	ErrOverCap    = errors.New("store: spend cap exceeded")
	ErrNotClaimed = errors.New("store: mail is not claimed")
	ErrNoEntry    = errors.New("store: no open reservation with that id")
)

const (
	LedgerReserved = "reserved"
	LedgerSettled  = "settled"
	LedgerReleased = "released"
)

// SpendSince is what a bot has spent or has reserved (not released) since the given time.
func (s *Store) SpendSince(bot string, since time.Time) (float64, error) {
	return spendSince(s.db, bot, since)
}

type rower interface {
	QueryRow(query string, args ...any) *sql.Row
}

func spendSince(q rower, bot string, since time.Time) (float64, error) {
	var sum float64
	err := q.QueryRow(`SELECT COALESCE(SUM(CASE state WHEN ? THEN settled WHEN ? THEN reserved ELSE 0 END),0)
		FROM ledger WHERE bot=? AND ts>=?`, LedgerSettled, LedgerReserved, bot, since.UnixNano()).Scan(&sum)
	return sum, err
}

// Reserve atomically checks the bot's daily and monthly spend (a cap <= 0 means unlimited)
// and records an estimate; day and month start in the local zone.
func (s *Store) Reserve(bot, task, model string, est, capDaily, capMonthly float64) (int64, error) {
	return s.reserveAt(time.Now(), bot, task, model, est, capDaily, capMonthly)
}

func (s *Store) reserveAt(now time.Time, bot, task, model string, est, capDaily, capMonthly float64) (id int64, err error) {
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	err = s.tx(func(tx dbtx) error {
		for _, c := range []struct {
			cap   float64
			since time.Time
		}{{capDaily, day}, {capMonthly, month}} {
			if c.cap <= 0 {
				continue
			}
			spent, err := spendSince(tx, bot, c.since)
			if err != nil {
				return err
			}
			if spent+est > c.cap {
				return ErrOverCap
			}
		}
		res, err := tx.Exec(`INSERT INTO ledger (bot,task,model,reserved,state,ts) VALUES (?,?,?,?,?,?)`,
			bot, task, model, est, LedgerReserved, now.UnixNano())
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		return err
	})
	return
}

// Settle records the actual cost of a reservation.
func (s *Store) Settle(id int64, actual float64) error {
	return s.close(id, LedgerSettled, actual)
}

// Release drops a reservation that was never used.
func (s *Store) Release(id int64) error {
	return s.close(id, LedgerReleased, 0)
}

func (s *Store) close(id int64, state string, actual float64) error {
	res, err := s.db.Exec(`UPDATE ledger SET state=?, settled=? WHERE id=? AND state=?`,
		state, actual, id, LedgerReserved)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNoEntry
	}
	return nil
}
