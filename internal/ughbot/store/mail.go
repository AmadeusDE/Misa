package store

import (
	"database/sql"
	"errors"
	"time"
)

const (
	MailQueued  = "queued"
	MailClaimed = "claimed"
	MailDone    = "done"
)

// Mail kinds: plain mail, a question expecting a reply, the reply (ReplyTo names the
// question) and steering from a managing bot.
const (
	KindMail  = "mail"
	KindAsk   = "ask"
	KindReply = "reply"
	KindSteer = "steer"
)

type Mail struct {
	ID      int64
	To      string
	From    string
	Kind    string
	ReplyTo int64
	Body    string
	State   string
	TS      time.Time
}

const mailColumns = `id,"to","from",kind,reply_to,body,state,ts`

func scanMail(row interface{ Scan(...any) error }) (Mail, error) {
	var m Mail
	var ts int64
	err := row.Scan(&m.ID, &m.To, &m.From, &m.Kind, &m.ReplyTo, &m.Body, &m.State, &ts)
	m.TS = time.Unix(0, ts)
	return m, err
}

// Enqueue stores m (To, From, Kind, ReplyTo, Body) as queued mail and returns its id.
func (s *Store) Enqueue(m Mail) (int64, error) {
	if m.Kind == "" {
		m.Kind = KindMail
	}
	res, err := s.db.Exec(`INSERT INTO mail ("to","from",kind,reply_to,body,state,ts) VALUES (?,?,?,?,?,?,?)`,
		m.To, m.From, m.Kind, m.ReplyTo, m.Body, MailQueued, time.Now().UnixNano())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// MailByID returns one message; ok is false when there is none with that id.
func (s *Store) MailByID(id int64) (m Mail, ok bool, err error) {
	m, err = scanMail(s.db.QueryRow(`SELECT `+mailColumns+` FROM mail WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Mail{}, false, nil
	}
	return m, err == nil, err
}

// Replied reports whether from has already replied to the question with that id.
func (s *Store) Replied(from string, question int64) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM mail WHERE "from"=? AND kind=? AND reply_to=?`, from, KindReply, question).Scan(&n)
	return n > 0, err
}

// Claim hands the oldest queued mail for a bot to one consumer; ok is false when none is waiting.
func (s *Store) Claim(to string) (m Mail, ok bool, err error) {
	err = s.tx(func(tx dbtx) error {
		var serr error
		m, serr = scanMail(tx.QueryRow(`UPDATE mail SET state=? WHERE id=(
			SELECT id FROM mail WHERE "to"=? AND state=? ORDER BY id LIMIT 1)
			RETURNING `+mailColumns, MailClaimed, to, MailQueued))
		if errors.Is(serr, sql.ErrNoRows) {
			m = Mail{}
			return nil
		}
		ok = serr == nil
		return serr
	})
	return
}

// Ack marks claimed mail as handled.
func (s *Store) Ack(id int64) error {
	res, err := s.db.Exec(`UPDATE mail SET state=? WHERE id=? AND state=?`, MailDone, id, MailClaimed)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotClaimed
	}
	return nil
}

// Requeue returns claimed mail to the queue (a consumer crashed or restarted); it reports how many.
func (s *Store) RequeueClaimed(to string) (int64, error) {
	res, err := s.db.Exec(`UPDATE mail SET state=? WHERE "to"=? AND state=?`, MailQueued, to, MailClaimed)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Pending counts queued mail for a bot.
func (s *Store) Pending(to string) (n int, err error) {
	err = s.db.QueryRow(`SELECT COUNT(*) FROM mail WHERE "to"=? AND state=?`, to, MailQueued).Scan(&n)
	return
}

// MailTo returns the latest mail for a bot in any state, newest first.
func (s *Store) MailTo(to string, limit int) ([]Mail, error) {
	rows, err := s.db.Query(`SELECT `+mailColumns+` FROM mail WHERE "to"=? ORDER BY id DESC LIMIT ?`, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Mail
	for rows.Next() {
		m, err := scanMail(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
