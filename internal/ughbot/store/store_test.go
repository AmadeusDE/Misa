package store

import (
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func open(t *testing.T) (*Store, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "sub", "ughbot.db")
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, p
}

func TestMigrationsIdempotent(t *testing.T) {
	s, p := open(t)
	v, _ := s.SchemaVersion()
	if v != len(migrations) {
		t.Fatalf("version %d", v)
	}
	s.Close()
	s2, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if v2, _ := s2.SchemaVersion(); v2 != v {
		t.Fatalf("reopen version %d != %d", v2, v)
	}
	var n int
	s2.db.QueryRow(`SELECT COUNT(*) FROM schema_version`).Scan(&n)
	if n != len(migrations) {
		t.Fatalf("schema_version rows %d", n)
	}
	var mode string
	s2.db.QueryRow(`PRAGMA journal_mode`).Scan(&mode)
	if mode != "wal" {
		t.Fatalf("journal mode %q", mode)
	}
	for _, tbl := range []string{"bots", "threads", "tasks", "ledger", "mail"} {
		if _, err := s2.db.Exec("SELECT 1 FROM " + tbl); err != nil {
			t.Fatalf("%s: %v", tbl, err)
		}
	}
}

func TestMailRoundTrip(t *testing.T) {
	s, _ := open(t)
	if _, ok, _ := s.Claim("alpha"); ok {
		t.Fatal("claim on empty")
	}
	a, _ := s.Enqueue(Mail{To: "alpha", From: "user", Body: "first"})
	b, _ := s.Enqueue(Mail{To: "alpha", From: "beta", Kind: KindReply, ReplyTo: a, Body: "second"})
	s.Enqueue(Mail{To: "other", From: "user", Body: "elsewhere"})
	m, ok, err := s.Claim("alpha")
	if err != nil || !ok || m.ID != a || m.Body != "first" || m.From != "user" || m.Kind != KindMail || m.State != MailClaimed {
		t.Fatalf("%+v %v %v", m, ok, err)
	}
	if err := s.Ack(a); err != nil {
		t.Fatal(err)
	}
	if err := s.Ack(a); !errors.Is(err, ErrNotClaimed) {
		t.Fatalf("double ack: %v", err)
	}
	m, ok, _ = s.Claim("alpha")
	if !ok || m.ID != b || m.ReplyTo != a || m.Kind != KindReply {
		t.Fatalf("%+v", m)
	}
	if got, ok, _ := s.MailByID(a); !ok || got.Body != "first" {
		t.Fatalf("by id: %+v", got)
	}
	if _, ok, err := s.MailByID(999); ok || err != nil {
		t.Fatalf("missing id: %v %v", ok, err)
	}
	if r, _ := s.Replied("beta", a); !r {
		t.Fatal("reply not seen")
	}
	if r, _ := s.Replied("user", a); r {
		t.Fatal("user never replied")
	}
	if n, _ := s.RequeueClaimed("alpha"); n != 1 {
		t.Fatalf("requeued %d", n)
	}
	if n, _ := s.Pending("alpha"); n != 1 {
		t.Fatalf("pending %d", n)
	}
}

func TestReserveSettleRelease(t *testing.T) {
	s, _ := open(t)
	id, err := s.Reserve("alpha", "t1", "m", 0.5, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reserve("alpha", "t1", "m", 0.6, 1, 10); !errors.Is(err, ErrOverCap) {
		t.Fatalf("want over cap: %v", err)
	}
	if err := s.Settle(id, 0.2); err != nil {
		t.Fatal(err)
	}
	if sp, _ := s.SpendSince("alpha", time.Now().Add(-time.Hour)); sp != 0.2 {
		t.Fatalf("spend %v", sp)
	}
	id2, err := s.Reserve("alpha", "t1", "m", 0.8, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Release(id2); err != nil {
		t.Fatal(err)
	}
	if err := s.Release(id2); !errors.Is(err, ErrNoEntry) {
		t.Fatalf("%v", err)
	}
	if _, err := s.Reserve("alpha", "", "m", 0.9, 1, 0.5); !errors.Is(err, ErrOverCap) {
		t.Fatalf("monthly cap: %v", err)
	}
	if _, err := s.Reserve("alpha", "", "m", 100, 0, 0); err != nil {
		t.Fatalf("uncapped: %v", err)
	}
	if _, err := s.Reserve("bravo", "", "m", 1, 1, 1); err != nil {
		t.Fatalf("other bot has own budget: %v", err)
	}
}

func TestDailyWindowIgnoresYesterday(t *testing.T) {
	s, _ := open(t)
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	if _, err := s.reserveAt(yesterday, "alpha", "", "m", 0.9, 1, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.reserveAt(now, "alpha", "", "m", 0.9, 1, 0); err != nil {
		t.Fatalf("yesterday must not count today: %v", err)
	}
}

func TestConcurrentReserveNeverExceedsCap(t *testing.T) {
	s, _ := open(t)
	const cap = 5.0
	var ok atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Reserve("alpha", "", "m", 0.5, cap, 0)
			switch {
			case err == nil:
				ok.Add(1)
			case !errors.Is(err, ErrOverCap):
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != 10 {
		t.Fatalf("granted %d reservations, want 10", ok.Load())
	}
	if sp, _ := s.SpendSince("alpha", time.Now().Add(-time.Hour)); sp > cap+1e-9 {
		t.Fatalf("spent %v > cap", sp)
	}
}

func TestCostByDayGroupsSettledSpend(t *testing.T) {
	s, _ := open(t)
	for _, c := range []struct {
		bot string
		usd float64
	}{{"a", 0.5}, {"a", 0.25}, {"b", 1}} {
		id, err := s.Reserve(c.bot, "", "m", c.usd, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Settle(id, c.usd); err != nil {
			t.Fatal(err)
		}
	}
	open, _ := s.Reserve("a", "", "m", 9, 0, 0)
	_ = open
	rows, err := s.CostByDay("", "", "")
	if err != nil || len(rows) != 2 {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
	if rows[0].Bot != "a" || rows[0].USD != 0.75 || rows[0].Day != Today() {
		t.Fatalf("a row = %+v", rows[0])
	}
	if one, _ := s.CostByDay("b", "", ""); len(one) != 1 || one[0].USD != 1 {
		t.Fatalf("filtered = %v", one)
	}
	if none, _ := s.CostByDay("", "2999-01-01", ""); len(none) != 0 {
		t.Fatalf("from filter = %v", none)
	}
}

func TestApprovalLifecycle(t *testing.T) {
	s, _ := open(t)
	now := time.Now()
	id, err := s.AddApproval(Approval{Bot: "a", Action: "create_bot", Args: "{}", Hash: "h", Summary: "make b", Expires: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	old, _ := s.AddApproval(Approval{Bot: "a", Action: "delete_bot", Args: "{}", Hash: "h2", Summary: "old", Expires: now.Add(-time.Second)})
	if p, _ := s.Approvals(ApprovalPending, 10); len(p) != 2 || p[0].ID != old {
		t.Fatalf("pending newest first: %+v", p)
	}
	if _, err := s.Decide(old, ApprovalApproved, "cli", now); !errors.Is(err, ErrNotPending) {
		t.Fatalf("an expired request cannot be approved: %v", err)
	}
	exp, _ := s.ExpireApprovals(now)
	if len(exp) != 1 || exp[0].ID != old || exp[0].State != ApprovalExpired {
		t.Fatalf("expired: %+v", exp)
	}
	a, err := s.Decide(id, ApprovalApproved, "cli", now)
	if err != nil || a.State != ApprovalApproved || a.DecidedBy != "cli" || a.Summary != "make b" {
		t.Fatalf("%+v %v", a, err)
	}
	if _, err := s.Decide(id, ApprovalDenied, "web", now); !errors.Is(err, ErrNotPending) {
		t.Fatalf("decided twice: %v", err)
	}
	s.SetApprovalResult(id, "boom", true)
	if a, ok, _ := s.Approval(id); !ok || a.State != ApprovalFailed || a.Result != "boom" {
		t.Fatalf("%+v", a)
	}
	if all, _ := s.Approvals("", 10); len(all) != 2 {
		t.Fatalf("all: %d", len(all))
	}
}

func TestRoutineFiresOncePerTime(t *testing.T) {
	s, _ := open(t)
	now := time.Now()
	id, err := s.AddRoutine(Routine{Owner: "user", Bot: "alpha", Prompt: "check", Cron: "* * * * *", Missed: MissedOnce, Next: now.Add(-time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	due, _ := s.DueRoutines(now)
	if len(due) != 1 || due[0].ID != id {
		t.Fatalf("due: %+v", due)
	}
	r := due[0]
	mail := &Mail{To: "alpha", From: "routine #1", Body: "check"}
	ok, mid, err := s.FireRoutine(r, now.Add(time.Minute), mail, now)
	if err != nil || !ok || mid == 0 {
		t.Fatalf("%v %v %v", ok, mid, err)
	}
	if ok, _, _ := s.FireRoutine(r, now.Add(time.Minute), mail, now); ok {
		t.Fatal("fired twice for the same time")
	}
	if n, _ := s.Pending("alpha"); n != 1 {
		t.Fatalf("mail queued %d", n)
	}
	got, _ := s.Routine(id)
	if got.Fired != 1 || got.Last.IsZero() || !got.Next.Equal(time.Unix(0, now.Add(time.Minute).UnixNano())) {
		t.Fatalf("%+v", got)
	}
	// skipping advances without mail; a zero next ends the routine
	if ok, _, _ := s.FireRoutine(got, time.Time{}, nil, now); !ok {
		t.Fatal("skip")
	}
	if got, _ = s.Routine(id); got.State != RoutineDone || got.Fired != 1 {
		t.Fatalf("%+v", got)
	}
	if err := s.SetRoutineState(id, RoutinePaused, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteRoutine(id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Routine(id); !errors.Is(err, ErrNoRoutine) {
		t.Fatal(err)
	}
}
