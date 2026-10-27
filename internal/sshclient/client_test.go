package sshclient

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

// testServer accepts user alice with password "pw" or the key in authorized,
// and answers exec requests with "ran: <command>"; command "fail" exits 3.
type testServer struct {
	port       string
	authorized ssh.PublicKey
}

func startServer(t *testing.T) *testServer {
	t.Helper()
	_, hostPriv, _ := ed25519.GenerateKey(rand.Reader)
	hostKey, err := ssh.NewSignerFromKey(hostPriv)
	if err != nil {
		t.Fatal(err)
	}
	s := &testServer{}
	cfg := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pw []byte) (*ssh.Permissions, error) {
			if c.User() == "alice" && string(pw) == "pw" {
				return nil, nil
			}
			return nil, fmt.Errorf("denied")
		},
		PublicKeyCallback: func(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if s.authorized != nil && bytes.Equal(key.Marshal(), s.authorized.Marshal()) {
				return nil, nil
			}
			return nil, fmt.Errorf("unknown key")
		},
	}
	cfg.AddHostKey(hostKey)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	s.port = strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveConn(conn, cfg)
		}
	}()
	return s
}

func serveConn(conn net.Conn, cfg *ssh.ServerConfig) {
	_, chans, reqs, err := ssh.NewServerConn(conn, cfg)
	if err != nil {
		conn.Close()
		return
	}
	go ssh.DiscardRequests(reqs)
	for nc := range chans {
		if nc.ChannelType() != "session" {
			nc.Reject(ssh.UnknownChannelType, "")
			continue
		}
		ch, requests, err := nc.Accept()
		if err != nil {
			continue
		}
		go func() {
			for req := range requests {
				if req.Type != "exec" {
					req.Reply(false, nil)
					continue
				}
				var exec struct{ Command string }
				ssh.Unmarshal(req.Payload, &exec)
				req.Reply(true, nil)
				fmt.Fprintf(ch, "ran: %s\n", exec.Command)
				status := uint32(0)
				if exec.Command == "fail" {
					status = 3
				}
				ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
				ch.Close()
			}
		}()
	}
}

func clientEnv(t *testing.T, answers ...string) (env, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	e := testEnv(t, "")
	var out, errOut bytes.Buffer
	e.Stdin = strings.NewReader("")
	e.Stdout, e.Stderr = &out, &errOut
	e.Prompt = &scripted{answers: answers}
	return e, &out, &errOut
}

func (s *testServer) options(command ...string) Options {
	return Options{
		Destination: "alice@127.0.0.1",
		Port:        s.port,
		Command:     command,
		Settings:    []string{"StrictHostKeyChecking=accept-new"},
	}
}

func TestRunCommandWithPassword(t *testing.T) {
	s := startServer(t)
	e, out, errOut := clientEnv(t, "pw")
	if code := run(s.options("echo", "hi"), e); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if out.String() != "ran: echo hi\n" {
		t.Errorf("out = %q", out)
	}
	known, _ := os.ReadFile(filepath.Join(e.Home, ".ssh", "known_hosts"))
	if !strings.HasPrefix(string(known), "[127.0.0.1]:"+s.port+" ssh-ed25519 ") {
		t.Errorf("known_hosts = %q", known)
	}
	if !strings.Contains(e.Prompt.(*scripted).asked[0], "alice@127.0.0.1's password") {
		t.Errorf("asked %q", e.Prompt.(*scripted).asked)
	}
}

func TestRemoteExitStatus(t *testing.T) {
	s := startServer(t)
	e, _, _ := clientEnv(t, "pw")
	if code := run(s.options("fail"), e); code != 3 {
		t.Errorf("exit = %d, want 3", code)
	}
}

func TestRunWithIdentityFile(t *testing.T) {
	s := startServer(t)
	e, out, errOut := clientEnv(t)
	s.authorized = writeKey(t, filepath.Join(e.Home, ".ssh"), "id_ed25519", "")
	if code := run(s.options("whoami"), e); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if out.String() != "ran: whoami\n" {
		t.Errorf("out = %q", out)
	}
}

func TestWrongPasswordFails(t *testing.T) {
	s := startServer(t)
	e, _, errOut := clientEnv(t, "a", "b", "c")
	if code := run(s.options("x"), e); code != exitSSHError {
		t.Errorf("exit = %d", code)
	}
	if !strings.Contains(errOut.String(), "sshc: ") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestShellWithoutTerminalSkipsPTY(t *testing.T) {
	s := startServer(t)
	e, _, errOut := clientEnv(t, "pw")
	run(s.options(), e)
	if !strings.Contains(errOut.String(), "Pseudo-terminal will not be allocated") {
		t.Errorf("stderr = %q", errOut)
	}
}
