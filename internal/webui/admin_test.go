package webui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdminDeviceLifecycle(t *testing.T) {
	e := newEnv(t, nil)
	c := e.enrolled(t, "old name")
	if err := e.admin.RenameDevice("old name", "new name"); err != nil {
		t.Fatal(err)
	}
	devs, _ := e.admin.Devices()
	if len(devs) != 1 || devs[0].Name != "new name" || devs[0].ID != c.DeviceID {
		t.Fatalf("devices = %+v", devs)
	}
	if err := e.admin.RevokeDevice("nobody"); err == nil {
		t.Fatal("revoking an unknown device succeeded")
	}
	if err := e.admin.RevokeDevice(c.DeviceID); err != nil {
		t.Fatal(err)
	}
	kinds := []string{}
	evs, _ := e.admin.Events(0)
	for _, ev := range evs {
		kinds = append(kinds, ev.Kind)
	}
	if got := strings.Join(kinds, ","); !strings.Contains(got, "enroll") || !strings.HasSuffix(got, "revoke") {
		t.Fatalf("events = %s", got)
	}
}

func TestAdminSessionsListAndKill(t *testing.T) {
	e := newEnv(t, nil)
	c := e.enrolled(t, "a")
	if err := c.Login(t.Context()); err != nil {
		t.Fatal(err)
	}
	rows, _ := e.admin.Sessions()
	if len(rows) != 1 || rows[0].DeviceName != "a" {
		t.Fatalf("sessions = %+v", rows)
	}
	if err := e.admin.KillSession("abc"); err == nil {
		t.Fatal("short prefix accepted")
	}
	if err := e.admin.KillSession(rows[0].ID); err != nil {
		t.Fatal(err)
	}
	if rows, _ = e.admin.Sessions(); len(rows) != 0 {
		t.Fatalf("session survived kill: %+v", rows)
	}
}

func TestAdminTOTPInitAndReset(t *testing.T) {
	e := newEnv(t, nil)
	if _, _, err := e.admin.TOTPInit("Test", "me"); err == nil {
		t.Fatal("init replaced an existing secret")
	}
	secret, uri, err := e.admin.TOTPReset("Test", "me")
	if err != nil || secret == e.secret || !strings.HasPrefix(uri, "otpauth://totp/Test:me?") || !strings.Contains(uri, "secret="+secret) {
		t.Fatalf("reset = %q %q %v", secret, uri, err)
	}
	e.clock.Advance(totpPeriod * 1e9)
	old, _ := TOTPCode(e.secret, e.clock.Now())
	if err := e.client(t).Enroll(t.Context(), old, "x"); err == nil {
		t.Fatal("code from the old secret still works")
	}
}

func TestTOTPKnownVector(t *testing.T) {
	// RFC 6238 appendix B, SHA-1, secret "12345678901234567890", T=59 -> 94287082 (6 digits: 287082).
	secret := b32.EncodeToString([]byte("12345678901234567890"))
	got, err := hotp(secret, 59/30)
	if err != nil || got != "287082" {
		t.Fatalf("hotp = %q %v", got, err)
	}
}

func TestFileStorePersistsPrivately(t *testing.T) {
	dir := t.TempDir()
	st, _ := NewFileStore(dir)
	if err := st.Update(func(s *State) error { s.TOTP.Secret = "X"; return nil }); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "webui.json"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %v %v", info, err)
	}
	again, _ := NewFileStore(dir)
	var got string
	_ = again.View(func(s *State) { got = s.TOTP.Secret })
	if got != "X" {
		t.Fatalf("reloaded secret = %q", got)
	}
}

func TestFileStoreRollsBackOnError(t *testing.T) {
	st, _ := NewFileStore(t.TempDir())
	_ = st.Update(func(s *State) error { s.TOTP.Secret = "X"; return nil })
	err := st.Update(func(s *State) error { s.TOTP.Secret = "Y"; return os.ErrInvalid })
	var got string
	_ = st.View(func(s *State) { got = s.TOTP.Secret })
	if err == nil || got != "X" {
		t.Fatalf("err=%v secret=%q", err, got)
	}
}

func TestFileStoreSeesChangesFromAnotherProcess(t *testing.T) {
	dir := t.TempDir()
	daemon, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	cli, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := NewAdmin(cli, nil).TOTPInit("t", "me"); err != nil {
		t.Fatal(err)
	}
	var secret string
	_ = daemon.View(func(st *State) { secret = st.TOTP.Secret })
	if secret == "" {
		t.Fatal("running store did not notice the secret written by the admin CLI")
	}
	if err := daemon.Update(func(st *State) error { st.log(Event{Kind: "x"}); return nil }); err != nil {
		t.Fatal(err)
	}
	_ = cli.View(func(st *State) { secret = st.TOTP.Secret })
	if secret == "" {
		t.Fatal("update by the running store lost the admin's change")
	}
}

func TestAdminNoticesDismissOnlyForAdmin(t *testing.T) {
	e := newEnv(t, nil)
	c := e.enrolled(t, "phone")
	list, _ := e.admin.Notices()
	if len(list) != 1 {
		t.Fatalf("notices = %+v", list)
	}
	if err := e.admin.DismissNotice("nope"); err == nil {
		t.Fatal("unknown notice accepted")
	}
	if err := e.admin.DismissNotice(list[0].ID); err != nil {
		t.Fatal(err)
	}
	if list, _ = e.admin.Notices(); len(list) != 0 {
		t.Fatalf("still listed: %+v", list)
	}
	if got := e.srv.noticesFor(c.DeviceID); len(got) != 1 {
		t.Fatalf("browser copy lost: %+v", got)
	}
}
