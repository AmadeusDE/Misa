package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

type sharedTestBackend struct {
	path, barrier, id string
	first             bool
}

func (b *sharedTestBackend) Name() string { return "shared test" }
func (b *sharedTestBackend) Load() (*Credentials, error) {
	data, err := os.ReadFile(b.path)
	if err != nil {
		return nil, err
	}
	var c Credentials
	if err = json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	if !b.first {
		b.first = true
		if err = os.WriteFile(filepath.Join(b.barrier, b.id), nil, 0600); err != nil {
			return nil, err
		}
		deadline := time.Now().Add(10 * time.Second)
		for {
			if _, err = os.Stat(filepath.Join(b.barrier, "one")); err == nil {
				if _, err = os.Stat(filepath.Join(b.barrier, "two")); err == nil {
					break
				}
			}
			if time.Now().After(deadline) {
				return nil, fmt.Errorf("barrier timed out")
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	return &c, nil
}
func (b *sharedTestBackend) Save(c *Credentials) error {
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(b.path, data, 0600)
}

type redirectTestTransport struct{ target *url.URL }

func (r redirectTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = r.target.Scheme
	clone.URL.Host = r.target.Host
	return http.DefaultTransport.RoundTrip(clone)
}

func TestRefreshAcrossProcesses(t *testing.T) {
	if id := os.Getenv("UGH_AUTH_REFRESH_HELPER"); id != "" {
		target, err := url.Parse(os.Getenv("UGH_AUTH_SERVER"))
		if err != nil {
			t.Fatal(err)
		}
		codexHTTP = &http.Client{Transport: redirectTestTransport{target}, Timeout: 10 * time.Second}
		dir := os.Getenv("UGH_AUTH_TEST_DIR")
		st := NewStore(&sharedTestBackend{path: filepath.Join(dir, "credentials"), barrier: dir, id: id})
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		token, account, err := CodexTokens(ctx, st)
		if err != nil || token != "fresh" || account != "account" {
			t.Fatalf("token=%q account=%q error=%v", token, account, err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "credentials"))
		if err != nil {
			t.Fatal(err)
		}
		var persisted Credentials
		if err := json.Unmarshal(data, &persisted); err != nil {
			t.Fatal(err)
		}
		if c := persisted.Providers[codexProvider]; c.Access != "fresh" || c.Refresh != "rotated" {
			t.Fatalf("returned before rotated credentials were persisted: %+v", c)
		}
		return
	}
	dir := t.TempDir()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		time.Sleep(150 * time.Millisecond)
		fmt.Fprint(w, `{"access_token":"fresh","refresh_token":"rotated","expires_in":3600}`)
	}))
	defer server.Close()
	initial := Credentials{Version: 1, Providers: map[string]Credential{"codex": {Type: "oauth", Access: "expired", Refresh: "old", AccountID: "account"}}}
	data, _ := json.Marshal(initial)
	if err := os.WriteFile(filepath.Join(dir, "credentials"), data, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var cmds []*exec.Cmd
	defer func() {
		for _, cmd := range cmds {
			if cmd.ProcessState == nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
		}
	}()
	for _, id := range []string{"one", "two"} {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRefreshAcrossProcesses$")
		cmd.Env = append(os.Environ(), "UGH_AUTH_REFRESH_HELPER="+id, "UGH_AUTH_TEST_DIR="+dir, "UGH_AUTH_SERVER="+server.URL, "XDG_RUNTIME_DIR="+dir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		cmds = append(cmds, cmd)
	}
	for _, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			t.Error(err)
		}
	}
	if n := requests.Load(); n != 1 {
		t.Fatalf("refresh requests = %d, want 1", n)
	}
}

func TestRefreshLockRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)
	lock := filepath.Join(RuntimeDir(), "auth.lock")
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("safe"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, lock); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	unlock, err := lockAuth(context.Background())
	if err == nil {
		unlock()
		t.Fatal("accepted symlink")
	}
}

func TestRefreshLockCancellation(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	unlock, err := lockAuth(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	second, err := lockAuth(ctx)
	if err == nil {
		second()
		t.Fatal("contended lock accepted cancelled context")
	}
	if err.Error() != "auth: refresh lock busy" {
		t.Fatalf("error = %v", err)
	}
}
