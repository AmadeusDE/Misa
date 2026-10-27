package store

import "time"

// DayCost is what a bot settled on one local calendar day.
type DayCost struct {
	Bot string
	Day string
	USD float64
}

// CostByDay sums settled spend per bot and local day; empty bot, from or to ("YYYY-MM-DD") mean no filter.
func (s *Store) CostByDay(bot, from, to string) ([]DayCost, error) {
	rows, err := s.db.Query(`SELECT bot, day, SUM(settled) FROM (
			SELECT bot, settled, strftime('%Y-%m-%d', ts/1000000000, 'unixepoch', 'localtime') AS day
			FROM ledger WHERE state=? AND (?='' OR bot=?))
		WHERE (?='' OR day>=?) AND (?='' OR day<=?)
		GROUP BY bot, day ORDER BY day, bot`, LedgerSettled, bot, bot, from, from, to, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DayCost
	for rows.Next() {
		var c DayCost
		if err := rows.Scan(&c.Bot, &c.Day, &c.USD); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Today is the local calendar day as CostByDay formats it.
func Today() string { return time.Now().Format("2006-01-02") }
