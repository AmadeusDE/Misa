package webui

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// secondLogin is another login of c's device (a re-login, a second tab).
func secondLogin(t *testing.T, e *env, c *Client) *Client {
	t.Helper()
	c2, err := NewClient(e.ts.URL, c.Key)
	if err != nil {
		t.Fatal(err)
	}
	c2.DeviceID = c.DeviceID
	if err := c2.Login(t.Context()); err != nil {
		t.Fatal(err)
	}
	return c2
}

func logout(t *testing.T, e *env, c *Client) {
	t.Helper()
	req, _ := http.NewRequest("POST", e.ts.URL+"/auth/logout", strings.NewReader("{}"))
	if res, err := c.Do(req); err != nil || res.StatusCode != 200 {
		t.Fatalf("logout: %v", err)
	}
}

func TestTerminalFollowsTheLoginThatAttachedLast(t *testing.T) {
	e := newEnv(t, nil)
	h := e.mountTerm(t, "cat")
	a := e.loggedIn(t, "phone")
	v1, _ := openStream(t, e, a, "rows=5&cols=30")
	v1.waitFor(func() bool { return len(v1.lines) > 0 }, "frame")
	b := secondLogin(t, e, a)
	v2, _ := openStream(t, e, b, "rows=5&cols=30")
	v2.waitFor(func() bool { return len(v2.lines) > 0 }, "frame")

	logout(t, e, a)
	time.Sleep(10 * h.authCheck)
	if h.lookup(a.DeviceID) == nil {
		t.Fatal("signing out the replaced login closed the terminal the new login uses")
	}

	logout(t, e, b)
	waitUntil(t, func() bool { return h.lookup(a.DeviceID) == nil }, "terminal closed when its current login signed out")
}
