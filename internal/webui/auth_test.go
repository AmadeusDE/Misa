package webui

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEnrollThenLoginAndCSRF(t *testing.T) {
	e := newEnv(t, nil)
	c := e.enrolled(t, "laptop")
	if err := c.Login(t.Context()); err != nil {
		t.Fatal(err)
	}
	get, _ := http.NewRequest("GET", e.ts.URL+"/echo", nil)
	res, err := c.Do(get)
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("GET: %v %v", res, err)
	}
	post, _ := http.NewRequest("POST", e.ts.URL+"/echo", nil)
	res, err = c.Do(post)
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("POST with csrf: %v %v", res, err)
	}
	noCSRF, _ := http.NewRequest("POST", e.ts.URL+"/echo", nil)
	res, err = c.HTTP.Do(noCSRF)
	if err != nil || res.StatusCode != http.StatusForbidden {
		t.Fatalf("POST without csrf should be 403, got %v %v", res, err)
	}
	wrong, _ := http.NewRequest("POST", e.ts.URL+"/echo", nil)
	wrong.Header.Set(csrfHeader, "nope")
	if res, _ = c.HTTP.Do(wrong); res.StatusCode != http.StatusForbidden {
		t.Fatalf("wrong csrf token accepted: %d", res.StatusCode)
	}
}

func TestSessionCookieAttributes(t *testing.T) {
	e := newEnv(t, nil)
	c := e.enrolled(t, "laptop")
	req, _ := http.NewRequest("GET", e.ts.URL+"/auth/challenge", nil)
	res, _ := c.HTTP.Do(req)
	var ch struct{ Nonce string }
	_ = json.NewDecoder(res.Body).Decode(&ch)
	body := `{"device_id":"` + c.DeviceID + `","nonce":"` + ch.Nonce + `","signature":"` +
		base64.StdEncoding.EncodeToString(SignChallenge(c.Key, ch.Nonce)) + `"}`
	res, _ = http.Post(e.ts.URL+"/auth/login", "application/json", strings.NewReader(body))
	var found *http.Cookie
	for _, ck := range res.Cookies() {
		if ck.Name == cookieName {
			found = ck
		}
	}
	if found == nil || !found.HttpOnly || found.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie = %+v", found)
	}
}

func TestUnauthenticatedMountedRoute(t *testing.T) {
	e := newEnv(t, nil)
	res, _ := http.Get(e.ts.URL + "/echo")
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", res.StatusCode)
	}
	req, _ := http.NewRequest("GET", e.ts.URL+"/echo", nil)
	req.Header.Set("Accept", "text/html")
	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, _ = noFollow.Do(req)
	if res.StatusCode != http.StatusSeeOther || !strings.HasPrefix(res.Header.Get("Location"), "/login") {
		t.Fatalf("browser redirect = %d %q", res.StatusCode, res.Header.Get("Location"))
	}
}

func TestTOTPReplayRejected(t *testing.T) {
	e := newEnv(t, nil)
	code := e.code(t)
	if err := e.client(t).Enroll(t.Context(), code, "one"); err != nil {
		t.Fatal(err)
	}
	if err := e.client(t).Enroll(t.Context(), code, "two"); err == nil {
		t.Fatal("same TOTP code accepted twice")
	}
	e.clock.Advance(totpPeriod * time.Second)
	if err := e.client(t).Enroll(t.Context(), e.code(t), "two"); err != nil {
		t.Fatalf("next step code rejected: %v", err)
	}
}

func TestEnrollLockout(t *testing.T) {
	e := newEnv(t, nil)
	for i := 0; i < 5; i++ {
		if err := e.client(t).Enroll(t.Context(), "000000", "x"); err == nil || strings.Contains(err.Error(), "locked") {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	err := e.client(t).Enroll(t.Context(), e.code(t), "x")
	if err == nil || !strings.Contains(err.Error(), "locked out") {
		t.Fatalf("correct code during lockout should be refused, got %v", err)
	}
	e.clock.Advance(failWindow + time.Second)
	if err := e.client(t).Enroll(t.Context(), e.code(t), "x"); err != nil {
		t.Fatalf("lockout should expire: %v", err)
	}
}

func TestEnrollGlobalLimit(t *testing.T) {
	e := newEnv(t, nil)
	e.srv.enrollLimit = newLimiter(100, 3, failWindow)
	for i := 0; i < 3; i++ {
		_ = e.client(t).Enroll(t.Context(), "000000", "x")
	}
	if err := e.client(t).Enroll(t.Context(), e.code(t), "x"); err == nil || !strings.Contains(err.Error(), "locked out") {
		t.Fatalf("global limit not applied: %v", err)
	}
}

func TestEnrollWithoutTOTPInit(t *testing.T) {
	store, _ := NewFileStore(t.TempDir())
	srv, _ := New(Options{Store: store, ListenAddr: "127.0.0.1:0", Logf: func(string, ...any) {}})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	c, _ := NewClient(ts.URL, priv)
	if err := c.Enroll(t.Context(), "123456", "x"); err == nil {
		t.Fatal("enrolled without a TOTP secret")
	}
}

func TestRevokedDeviceSessionDies(t *testing.T) {
	e := newEnv(t, nil)
	c := e.enrolled(t, "phone")
	if err := c.Login(t.Context()); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", e.ts.URL+"/echo", nil)
	if res, _ := c.Do(req); res.StatusCode != 200 {
		t.Fatalf("before revoke: %d", res.StatusCode)
	}
	hub := e.mountTerm(t, "cat")
	v, _ := openStream(t, e, c, "rows=5&cols=30")
	v.waitFor(func() bool { return len(v.lines) > 0 }, "terminal frame")
	if err := e.admin.RevokeDevice("phone"); err != nil {
		t.Fatal(err)
	}
	req, _ = http.NewRequest("GET", e.ts.URL+"/echo", nil)
	if res, _ := c.Do(req); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("after revoke: %d", res.StatusCode)
	}
	waitUntil(t, func() bool { return hub.lookup(c.DeviceID) == nil }, "revoked device's terminal closed")
	if err := c.Login(t.Context()); err == nil {
		t.Fatal("revoked device can log in")
	}
}

func TestSessionExpires(t *testing.T) {
	e := newEnv(t, func(o *Options) { o.SessionTTL = time.Minute })
	c := e.enrolled(t, "phone")
	_ = c.Login(t.Context())
	e.clock.Advance(2 * time.Minute)
	req, _ := http.NewRequest("GET", e.ts.URL+"/echo", nil)
	if res, _ := c.HTTP.Do(req); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expired session status = %d", res.StatusCode)
	}
}

func rawLogin(t *testing.T, e *env, deviceID, nonce string, sig []byte) int {
	t.Helper()
	b, _ := json.Marshal(loginRequest{DeviceID: deviceID, Nonce: nonce, Signature: base64.StdEncoding.EncodeToString(sig)})
	res, err := http.Post(e.ts.URL+"/auth/login", "application/json", strings.NewReader(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode
}

func nonce(t *testing.T, e *env) string {
	t.Helper()
	res, err := http.Get(e.ts.URL + "/auth/challenge")
	if err != nil {
		t.Fatal(err)
	}
	var ch struct{ Nonce string }
	_ = json.NewDecoder(res.Body).Decode(&ch)
	return ch.Nonce
}

func TestChallengeReplayRejected(t *testing.T) {
	e := newEnv(t, nil)
	c := e.enrolled(t, "a")
	n := nonce(t, e)
	sig := SignChallenge(c.Key, n)
	if code := rawLogin(t, e, c.DeviceID, n, sig); code != 200 {
		t.Fatalf("first login = %d", code)
	}
	if code := rawLogin(t, e, c.DeviceID, n, sig); code != http.StatusUnauthorized {
		t.Fatalf("replayed challenge = %d", code)
	}
}

func TestChallengeExpires(t *testing.T) {
	e := newEnv(t, nil)
	c := e.enrolled(t, "a")
	n := nonce(t, e)
	e.clock.Advance(challengeTTL + time.Second)
	if code := rawLogin(t, e, c.DeviceID, n, SignChallenge(c.Key, n)); code != http.StatusUnauthorized {
		t.Fatalf("expired challenge = %d", code)
	}
}

func TestLoginWrongSignature(t *testing.T) {
	e := newEnv(t, nil)
	c := e.enrolled(t, "a")
	_, other, _ := ed25519.GenerateKey(rand.Reader)
	n := nonce(t, e)
	if code := rawLogin(t, e, c.DeviceID, n, SignChallenge(other, n)); code != http.StatusUnauthorized {
		t.Fatalf("wrong key = %d", code)
	}
	n = nonce(t, e)
	if code := rawLogin(t, e, c.DeviceID, n, SignChallenge(c.Key, "different")); code != http.StatusUnauthorized {
		t.Fatalf("signature over another nonce = %d", code)
	}
	if code := rawLogin(t, e, "nosuchdevice", "forged", []byte("x")); code != http.StatusUnauthorized {
		t.Fatalf("unknown nonce = %d", code)
	}
}

func TestAuthorizedKeysLogin(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	pub := priv.Public().(ed25519.PublicKey)
	path := filepath.Join(t.TempDir(), "authorized_keys")
	line := "# comment\n" + MarshalAuthorizedKey(pub, "desk pane")
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	e := newEnv(t, func(o *Options) { o.AuthorizedKeys = path })
	c, _ := NewClient(e.ts.URL, priv)
	req, _ := http.NewRequest("POST", e.ts.URL+"/echo", nil)
	if res, err := c.Do(req); err != nil || res.StatusCode != 200 {
		t.Fatalf("authorized key request: %v %v", res, err)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	req, _ = http.NewRequest("GET", e.ts.URL+"/echo", nil)
	if res, _ := c.Do(req); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("key removed from file but session lives: %d", res.StatusCode)
	}
}

func TestParseAuthorizedKeysRoundTrip(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	keys, err := ParseAuthorizedKeys(strings.NewReader("ssh-rsa AAAA x\n" + MarshalAuthorizedKey(pub, "a b")))
	if err != nil || len(keys) != 1 || !keys[0].PubKey.Equal(pub) || keys[0].Comment != "a b" {
		t.Fatalf("keys = %+v err = %v", keys, err)
	}
}

func TestEnrollRaisesNoticeAndDismiss(t *testing.T) {
	e := newEnv(t, nil)
	a := e.enrolled(t, "first")
	_ = e.enrolled(t, "second")
	if err := a.Login(t.Context()); err != nil {
		t.Fatal(err)
	}
	var id string
	_ = e.store.View(func(st *State) { id = st.Notices[0].ID })
	if got := e.srv.noticesFor(a.DeviceID); len(got) != 2 {
		t.Fatalf("notices before dismiss = %+v", got)
	}
	req, _ := http.NewRequest("POST", e.ts.URL+"/auth/notices/dismiss", strings.NewReader(`{"id":"`+id+`"}`))
	req.Header.Set("Content-Type", "application/json")
	if res, _ := a.Do(req); res.StatusCode != 200 {
		t.Fatalf("dismiss = %d", res.StatusCode)
	}
	got := e.srv.noticesFor(a.DeviceID)
	if len(got) != 1 || got[0].Text != "New device enrolled: second" {
		t.Fatalf("notices after dismiss = %+v", got)
	}
}

func TestPublicPagesRender(t *testing.T) {
	e := newEnv(t, nil)
	for _, p := range []string{"/login", "/enroll", "/static/ed25519.js", "/static/app.css"} {
		res, _ := http.Get(e.ts.URL + p)
		if res.StatusCode != 200 {
			t.Errorf("%s = %d", p, res.StatusCode)
		}
		if p == "/login" && !strings.Contains(readAll(res), "/static/auth.js") {
			t.Error("login page not in layout")
		}
	}
	res, _ := http.Get(e.ts.URL + "/login")
	if res.Header.Get("Content-Security-Policy") == "" {
		t.Error("no CSP header")
	}
}

func TestSafeNext(t *testing.T) {
	for in, want := range map[string]string{"/a?b=1": "/a?b=1", "//evil.com": "/", "https://evil.com": "/", "": "/", `/\evil`: "/",
		"/\t/evil.com": "/", "/\n/evil.com": "/", "/a\rb": "/"} {
		if got := safeNext(in); got != want {
			t.Errorf("safeNext(%q) = %q want %q", in, got, want)
		}
	}
}

func TestBindsOnlyToInterfaceAddress(t *testing.T) {
	store, _ := NewFileStore(t.TempDir())
	addrs := func(name string) ([]net.Addr, error) {
		return []net.Addr{&net.IPNet{IP: net.ParseIP("fe80::1")}, &net.IPNet{IP: net.ParseIP("10.8.0.2"), Mask: net.CIDRMask(24, 32)}}, nil
	}
	s, err := New(Options{Store: store, ListenInterface: "wg0", ListenAddr: "127.0.0.1:1", Port: 9000, InterfaceAddrs: addrs})
	if err != nil {
		t.Fatal(err)
	}
	if got := s.ListenAddress(); got != "10.8.0.2:9000" {
		t.Fatalf("addr = %s (ListenAddr must be ignored when an interface is set)", got)
	}
}

func TestRefusesToStartWithoutInterfaceAddress(t *testing.T) {
	store, _ := NewFileStore(t.TempDir())
	empty := func(string) ([]net.Addr, error) { return nil, nil }
	if _, err := New(Options{Store: store, ListenInterface: "wg0", ListenAddr: "127.0.0.1:8787", InterfaceAddrs: empty}); err == nil {
		t.Fatal("started with an address-less interface")
	}
	missing := func(string) ([]net.Addr, error) { return nil, net.UnknownNetworkError("no such interface") }
	if _, err := New(Options{Store: store, ListenInterface: "wg0", InterfaceAddrs: missing}); err == nil {
		t.Fatal("started with a missing interface")
	}
	s, err := New(Options{Store: store})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := s.resolveListen(); got != "0.0.0.0:8787" {
		t.Fatalf("default listen = %q, want 0.0.0.0:8787", got)
	}
}

func TestServeStopsOnCancel(t *testing.T) {
	e := newEnv(t, nil)
	ln, err := e.srv.Listen()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	cctx, stop := context.WithCancel(t.Context())
	go func() { done <- e.srv.ServeListener(cctx, ln) }()
	stop()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return")
	}
}

func TestDevicesPageAndRevokeEndpointAreGone(t *testing.T) {
	e := newEnv(t, nil)
	a := e.enrolled(t, "first")
	if err := a.Login(t.Context()); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ method, path, body string }{
		{"GET", "/devices", ""},
		{"POST", "/devices/revoke", `{"id":"x"}`},
	} {
		req, _ := http.NewRequest(c.method, e.ts.URL+c.path, strings.NewReader(c.body))
		req.Header.Set("Content-Type", "application/json")
		if res, _ := a.Do(req); res.StatusCode != http.StatusNotFound && res.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("%s %s = %d, want gone", c.method, c.path, res.StatusCode)
		}
	}
	for _, n := range e.srv.navItems() {
		if n.Path == "/devices" {
			t.Error("devices nav entry still registered")
		}
	}
}

func TestDeviceNamesRefuseControlCharacters(t *testing.T) {
	for name, want := range map[string]bool{"phone": true, "Ana's laptop ✓": true, "": false,
		"evil\x1b]0;pwned\a": false, "a\nb": false, strings.Repeat("x", 65): false, "bad\xff": false} {
		if got := validDeviceName(name); got != want {
			t.Errorf("validDeviceName(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestOneClientCannotExhaustChallengesOrLogins(t *testing.T) {
	e := newEnv(t, nil)
	h := e.srv.Handler()
	post := func(path, ip, body string) int {
		method := "POST"
		if path == "/auth/challenge" {
			method = "GET"
		}
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.RemoteAddr = ip + ":1234"
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	for range maxClientChallenges + 5 {
		post("/auth/challenge", "2001:db8::1", "")
	}
	if code := post("/auth/challenge", "2001:db8::2", ""); code != http.StatusTooManyRequests {
		t.Errorf("same /64: challenge = %d, want 429", code)
	}
	if code := post("/auth/challenge", "192.0.2.7", ""); code != http.StatusOK {
		t.Errorf("another client's challenge = %d, want 200", code)
	}
	for range 200 {
		post("/auth/login", "198.51.100.9", `{"device_id":"x","nonce":"n","signature":"AA=="}`)
	}
	if code := post("/auth/login", "192.0.2.7", `{"device_id":"x","nonce":"n","signature":"AA=="}`); code == http.StatusTooManyRequests {
		t.Error("one client's failed logins locked out another")
	}
}

func TestClientKeyGroupsIPv6By64(t *testing.T) {
	for in, want := range map[string]string{"192.0.2.1": "192.0.2.1", "2001:db8::1": "2001:db8::/64",
		"2001:db8:0:0:ffff::9": "2001:db8::/64", "::ffff:192.0.2.1": "::ffff:192.0.2.1", "bogus": "bogus"} {
		if got := clientKey(in); got != want {
			t.Errorf("clientKey(%q) = %q, want %q", in, got, want)
		}
	}
}
