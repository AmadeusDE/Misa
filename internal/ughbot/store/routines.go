package store

import (
	"database/sql"
	"errors"
	"time"
)

// Routine states. A one-shot routine is done after it fires.
const (
	RoutineActive = "active"
	RoutinePaused = "paused"
	RoutineDone   = "done"
)

// Missed-fire policies: fire once however many were missed, or skip to the next time.
const (
	MissedOnce = "once"
	MissedSkip = "skip"
)

// ErrNoRoutine is returned for an id that does not exist.
var ErrNoRoutine = errors.New("no such routine")

// Routine mails Prompt to Bot on its schedule: Cron (in TZ, empty = local) or the one-shot At.
type Routine struct {
	ID      int64
	Owner   string // who made it: "user", "cli", "tui", "web:<device>" or a bot name
	Bot     string
	Prompt  string
	Cron    string
	At      time.Time
	TZ      string
	Missed  string
	State   string
	Next    time.Time
	Last    time.Time
	Fired   int
	Created time.Time
}

const routineColumns = `id,owner,bot,prompt,cron,at,tz,missed,state,next,last,fired,created`

func unixOrZero(n int64) time.Time {
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(0, n)
}

func nanosOrZero(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixNano()
}

func scanRoutine(row interface{ Scan(...any) error }) (Routine, error) {
	var r Routine
	var at, next, last, created int64
	err := row.Scan(&r.ID, &r.Owner, &r.Bot, &r.Prompt, &r.Cron, &at, &r.TZ, &r.Missed, &r.State, &next, &last, &r.Fired, &created)
	r.At, r.Next, r.Last, r.Created = unixOrZero(at), unixOrZero(next), unixOrZero(last), unixOrZero(created)
	return r, err
}

// AddRoutine stores an active routine (Owner, Bot, Prompt, Cron or At, TZ, Missed, Next) and returns its id.
func (s *Store) AddRoutine(r Routine) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO routines (owner,bot,prompt,cron,at,tz,missed,state,next,created) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		r.Owner, r.Bot, r.Prompt, r.Cron, nanosOrZero(r.At), r.TZ, r.Missed, RoutineActive, r.Next.UnixNano(), time.Now().UnixNano())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) Routine(id int64) (Routine, error) {
	r, err := scanRoutine(s.db.QueryRow(`SELECT `+routineColumns+` FROM routines WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Routine{}, ErrNoRoutine
	}
	return r, err
}

// Routines lists every routine, or only those mailing bot when bot is set, by id.
func (s *Store) Routines(bot string) ([]Routine, error) {
	return s.routines(`SELECT `+routineColumns+` FROM routines WHERE ?='' OR bot=? ORDER BY id`, bot, bot)
}

// DueRoutines lists active routines whose next fire is at or before now.
func (s *Store) DueRoutines(now time.Time) ([]Routine, error) {
	return s.routines(`SELECT `+routineColumns+` FROM routines WHERE state=? AND next<=? ORDER BY next`, RoutineActive, now.UnixNano())
}

func (s *Store) routines(q string, args ...any) ([]Routine, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Routine
	for rows.Next() {
		r, err := scanRoutine(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// FireRoutine advances a due routine from the next time it was read with to next (zero ends a
// one-shot) and, when mail is set, queues it, in one transaction: a routine fires at most once
// per time even if two callers race. ok is false when the routine had already moved on.
func (s *Store) FireRoutine(r Routine, next time.Time, mail *Mail, now time.Time) (ok bool, mailID int64, err error) {
	err = s.tx(func(tx dbtx) error {
		state, nextN := RoutineActive, nanosOrZero(next)
		if next.IsZero() {
			state = RoutineDone
		}
		lastExpr, firedExpr := "last", "fired"
		if mail != nil {
			lastExpr, firedExpr = "?", "fired+1"
		}
		q := `UPDATE routines SET next=?, state=?, last=` + lastExpr + `, fired=` + firedExpr + ` WHERE id=? AND next=? AND state=?`
		args := []any{nextN, state}
		if mail != nil {
			args = append(args, now.UnixNano())
		}
		args = append(args, r.ID, r.Next.UnixNano(), RoutineActive)
		res, err := tx.Exec(q, args...)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil
		}
		ok = true
		if mail == nil {
			return nil
		}
		m := *mail
		if m.Kind == "" {
			m.Kind = KindMail
		}
		ins, err := tx.Exec(`INSERT INTO mail ("to","from",kind,reply_to,body,state,ts) VALUES (?,?,?,?,?,?,?)`,
			m.To, m.From, m.Kind, m.ReplyTo, m.Body, MailQueued, now.UnixNano())
		if err != nil {
			return err
		}
		mailID, err = ins.LastInsertId()
		return err
	})
	return
}

// SetRoutineState pauses or resumes a routine; resuming sets its next fire.
func (s *Store) SetRoutineState(id int64, state string, next time.Time) error {
	q, args := `UPDATE routines SET state=? WHERE id=?`, []any{state, id}
	if !next.IsZero() {
		q, args = `UPDATE routines SET state=?, next=? WHERE id=?`, []any{state, next.UnixNano(), id}
	}
	res, err := s.db.Exec(q, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNoRoutine
	}
	return nil
}

func (s *Store) DeleteRoutine(id int64) error {
	res, err := s.db.Exec(`DELETE FROM routines WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNoRoutine
	}
	return nil
}
