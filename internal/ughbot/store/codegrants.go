package store

import (
	"errors"
	"time"
)

// CodeGrant is a code directory a person approved for a bot, on top of its definition's.
type CodeGrant struct {
	Bot      string
	Dir      string
	Source   string // the URL it was cloned from; empty for a directory that already existed
	Approval int64
	Created  time.Time
}

var ErrNoGrant = errors.New("no such code grant")

func (s *Store) AddCodeGrant(g CodeGrant) error {
	_, err := s.db.Exec(`INSERT INTO code_grants (bot, dir, source, approval, created) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(bot, dir) DO NOTHING`, g.Bot, g.Dir, g.Source, g.Approval, time.Now().Unix())
	return err
}

// CodeGrants lists a bot's grants, or every grant when bot is empty, oldest first.
func (s *Store) CodeGrants(bot string) ([]CodeGrant, error) {
	rows, err := s.db.Query(`SELECT bot, dir, source, approval, created FROM code_grants
		WHERE ? = '' OR bot = ? ORDER BY created, bot, dir`, bot, bot)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CodeGrant
	for rows.Next() {
		var g CodeGrant
		var created int64
		if err := rows.Scan(&g.Bot, &g.Dir, &g.Source, &g.Approval, &created); err != nil {
			return nil, err
		}
		g.Created = time.Unix(created, 0)
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) DeleteCodeGrant(bot, dir string) error {
	res, err := s.db.Exec(`DELETE FROM code_grants WHERE bot = ? AND dir = ?`, bot, dir)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNoGrant
	}
	return nil
}

// DeleteCodeGrants drops every grant of a deleted bot.
func (s *Store) DeleteCodeGrants(bot string) error {
	_, err := s.db.Exec(`DELETE FROM code_grants WHERE bot = ?`, bot)
	return err
}
