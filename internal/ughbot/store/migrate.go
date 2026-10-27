package store

import "fmt"

var migrations = []string{
	`CREATE TABLE bots (
		name TEXT PRIMARY KEY,
		created INTEGER NOT NULL
	);
	CREATE TABLE threads (
		id TEXT PRIMARY KEY,
		bot TEXT NOT NULL REFERENCES bots(name),
		session TEXT NOT NULL DEFAULT '',
		created INTEGER NOT NULL
	);
	CREATE TABLE tasks (
		id TEXT PRIMARY KEY,
		bot TEXT NOT NULL REFERENCES bots(name),
		thread TEXT NOT NULL DEFAULT '',
		title TEXT NOT NULL DEFAULT '',
		state TEXT NOT NULL,
		created INTEGER NOT NULL,
		finished INTEGER NOT NULL DEFAULT 0
	);
	CREATE TABLE ledger (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		bot TEXT NOT NULL,
		task TEXT NOT NULL DEFAULT '',
		model TEXT NOT NULL,
		reserved REAL NOT NULL,
		settled REAL NOT NULL DEFAULT 0,
		state TEXT NOT NULL,
		ts INTEGER NOT NULL
	);
	CREATE INDEX ledger_bot_ts ON ledger(bot, ts);
	CREATE TABLE mail (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		"to" TEXT NOT NULL,
		"from" TEXT NOT NULL,
		reply_to INTEGER NOT NULL DEFAULT 0,
		body TEXT NOT NULL,
		state TEXT NOT NULL,
		ts INTEGER NOT NULL
	);
	CREATE INDEX mail_to_state ON mail("to", state, id);`,
	`CREATE TABLE bot_defs (
		name TEXT PRIMARY KEY,
		def TEXT NOT NULL,
		updated INTEGER NOT NULL
	);`,
	`ALTER TABLE mail ADD COLUMN kind TEXT NOT NULL DEFAULT 'mail';`,
	`CREATE TABLE approvals (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		bot TEXT NOT NULL,
		action TEXT NOT NULL,
		args TEXT NOT NULL,
		hash TEXT NOT NULL,
		summary TEXT NOT NULL,
		state TEXT NOT NULL,
		created INTEGER NOT NULL,
		expires INTEGER NOT NULL,
		decided INTEGER NOT NULL DEFAULT 0,
		decided_by TEXT NOT NULL DEFAULT '',
		result TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX approvals_state ON approvals(state, id);`,
	`CREATE TABLE routines (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		owner TEXT NOT NULL,
		bot TEXT NOT NULL,
		prompt TEXT NOT NULL,
		cron TEXT NOT NULL DEFAULT '',
		at INTEGER NOT NULL DEFAULT 0,
		tz TEXT NOT NULL DEFAULT '',
		missed TEXT NOT NULL,
		state TEXT NOT NULL,
		next INTEGER NOT NULL,
		last INTEGER NOT NULL DEFAULT 0,
		fired INTEGER NOT NULL DEFAULT 0,
		created INTEGER NOT NULL
	);
	CREATE INDEX routines_due ON routines(state, next);`,
	`CREATE TABLE code_grants (
		bot TEXT NOT NULL,
		dir TEXT NOT NULL,
		source TEXT NOT NULL DEFAULT '',
		approval INTEGER NOT NULL DEFAULT 0,
		created INTEGER NOT NULL,
		PRIMARY KEY (bot, dir)
	);`,
}

func (s *Store) migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return err
	}
	return s.tx(func(tx dbtx) error {
		var cur int
		err := tx.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&cur)
		if err != nil {
			return err
		}
		for i := cur; i < len(migrations); i++ {
			if _, err := tx.Exec(migrations[i]); err != nil {
				return fmt.Errorf("migration %d: %w", i+1, err)
			}
			if _, err := tx.Exec(`INSERT INTO schema_version (version) VALUES (?)`, i+1); err != nil {
				return err
			}
		}
		return nil
	})
}

// SchemaVersion reports the highest applied migration.
func (s *Store) SchemaVersion() (int, error) {
	var v int
	err := s.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&v)
	return v, err
}
