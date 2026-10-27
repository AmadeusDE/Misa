package store

import (
	"database/sql"
	"errors"
	"time"
)

// Approval states. Expired counts as denied; failed means approved but the action errored.
const (
	ApprovalPending  = "pending"
	ApprovalApproved = "approved"
	ApprovalDenied   = "denied"
	ApprovalExpired  = "expired"
	ApprovalFailed   = "failed"
)

// ErrNotPending is returned when deciding an approval that is decided, expired or absent.
var ErrNotPending = errors.New("approval is not pending")

type Approval struct {
	ID        int64
	Bot       string
	Action    string
	Args      string
	Hash      string
	Summary   string
	State     string
	Created   time.Time
	Expires   time.Time
	Decided   time.Time
	DecidedBy string
	Result    string
}

const approvalColumns = `id,bot,action,args,hash,summary,state,created,expires,decided,decided_by,result`

func scanApproval(row interface{ Scan(...any) error }) (Approval, error) {
	var a Approval
	var created, expires, decided int64
	err := row.Scan(&a.ID, &a.Bot, &a.Action, &a.Args, &a.Hash, &a.Summary, &a.State, &created, &expires, &decided, &a.DecidedBy, &a.Result)
	a.Created, a.Expires = time.Unix(0, created), time.Unix(0, expires)
	if decided != 0 {
		a.Decided = time.Unix(0, decided)
	}
	return a, err
}

// AddApproval stores a pending request (Bot, Action, Args, Hash, Summary, Expires) and returns its id.
func (s *Store) AddApproval(a Approval) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO approvals (bot,action,args,hash,summary,state,created,expires) VALUES (?,?,?,?,?,?,?,?)`,
		a.Bot, a.Action, a.Args, a.Hash, a.Summary, ApprovalPending, time.Now().UnixNano(), a.Expires.UnixNano())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Approval returns one request; ok is false when there is none with that id.
func (s *Store) Approval(id int64) (a Approval, ok bool, err error) {
	a, err = scanApproval(s.db.QueryRow(`SELECT `+approvalColumns+` FROM approvals WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Approval{}, false, nil
	}
	return a, err == nil, err
}

// Approvals lists requests in the given state, or all of them for "", newest first.
func (s *Store) Approvals(state string, limit int) ([]Approval, error) {
	rows, err := s.db.Query(`SELECT `+approvalColumns+` FROM approvals WHERE ?='' OR state=? ORDER BY id DESC LIMIT ?`, state, state, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Approval
	for rows.Next() {
		a, err := scanApproval(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Decide moves a pending, unexpired request to state; exactly one caller can win.
func (s *Store) Decide(id int64, state, by string, now time.Time) (Approval, error) {
	a, err := scanApproval(s.db.QueryRow(`UPDATE approvals SET state=?, decided=?, decided_by=?
		WHERE id=? AND state=? AND expires>? RETURNING `+approvalColumns,
		state, now.UnixNano(), by, id, ApprovalPending, now.UnixNano()))
	if errors.Is(err, sql.ErrNoRows) {
		return Approval{}, ErrNotPending
	}
	return a, err
}

// SetApprovalResult records what carrying out an approved request produced; failed marks it failed.
func (s *Store) SetApprovalResult(id int64, result string, failed bool) error {
	state := ApprovalApproved
	if failed {
		state = ApprovalFailed
	}
	_, err := s.db.Exec(`UPDATE approvals SET result=?, state=? WHERE id=?`, result, state, id)
	return err
}

// ExpireApprovals marks pending requests past their expiry as expired and returns them.
func (s *Store) ExpireApprovals(now time.Time) ([]Approval, error) {
	rows, err := s.db.Query(`UPDATE approvals SET state=?, decided=? WHERE state=? AND expires<=? RETURNING `+approvalColumns,
		ApprovalExpired, now.UnixNano(), ApprovalPending, now.UnixNano())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Approval
	for rows.Next() {
		a, err := scanApproval(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
