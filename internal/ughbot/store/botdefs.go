package store

import "time"

// PutBotDef stores the JSON definition of a bot made in the UI, replacing any earlier one.
func (s *Store) PutBotDef(name, def string) error {
	_, err := s.db.Exec(`INSERT INTO bot_defs (name, def, updated) VALUES (?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET def = excluded.def, updated = excluded.updated`, name, def, time.Now().Unix())
	return err
}

func (s *Store) DeleteBotDef(name string) error {
	_, err := s.db.Exec(`DELETE FROM bot_defs WHERE name = ?`, name)
	return err
}

// BotDefs returns every stored definition keyed by bot name.
func (s *Store) BotDefs() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT name, def FROM bot_defs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var n, d string
		if err := rows.Scan(&n, &d); err != nil {
			return nil, err
		}
		out[n] = d
	}
	return out, rows.Err()
}
