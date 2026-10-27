package vm

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/AmadeusDE/Misa/internal/ughbot/sandbox"
)

const (
	bootTimeout = 3 * time.Minute
	guestHelper = "/home/" + guestUser + "/.local/bin/ughbot"
)

// Start boots a fresh overlay of the base that sees shares at the same paths, and starts the
// tool helper in it. It waits for a free slot when max_running machines are already up.
func (m *Manager) Start(ctx context.Context, shares []string) (sandbox.Machine, error) {
	if _, err := os.Stat(m.basePath()); err != nil {
		return nil, errors.New("no prepared VM base yet: run `ughbot vm prepare`")
	}
	select {
	case m.slot <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	mc, err := m.start(ctx, shares)
	if err != nil {
		<-m.slot
		return nil, err
	}
	return mc, nil
}

func (m *Manager) start(ctx context.Context, shares []string) (_ *machine, err error) {
	k, err := loadKeys(m.dir)
	if err != nil {
		return nil, err
	}
	idb := make([]byte, 4)
	_, _ = rand.Read(idb)
	mc := &machine{m: m, id: hex.EncodeToString(idb), shares: shares}
	mc.dir = filepath.Join(m.runDir(), mc.id)
	if err := os.MkdirAll(mc.dir, 0o700); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			mc.teardown()
		}
	}()
	overlay := filepath.Join(mc.dir, "disk.qcow2")
	if err := qemuImg("create", "-q", "-f", "qcow2", "-F", "qcow2", "-b", m.basePath(), overlay); err != nil {
		return nil, err
	}
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	mc.addr = "127.0.0.1:" + strconv.Itoa(port)
	serial := filepath.Join(mc.dir, "serial.log")
	if mc.q, err = startQEMU(qemuArgs(m.cfg, boot{disk: overlay, serial: serial, sshPort: port, shares: shares}), filepath.Join(mc.dir, "qemu.log")); err != nil {
		return nil, err
	}
	_ = os.WriteFile(filepath.Join(mc.dir, "pid"), []byte(strconv.Itoa(mc.q.cmd.Process.Pid)), 0o600)
	if mc.conn, err = dial(ctx, mc.addr, k, mc.q.done); err != nil {
		return nil, fmt.Errorf("vm %s: %w (serial log: %s)", mc.id, err, serial)
	}
	if err := mc.install(); err != nil {
		return nil, fmt.Errorf("vm %s: copying the helper: %w", mc.id, err)
	}
	for i, dir := range shares {
		if err := mc.mount(i, dir); err != nil {
			return nil, fmt.Errorf("vm %s: sharing %s: %w", mc.id, dir, err)
		}
	}
	mc.gitIdentity(shares)
	if err := mc.startHelper(); err != nil {
		return nil, fmt.Errorf("vm %s: starting the helper: %w", mc.id, err)
	}
	m.mu.Lock()
	m.running[mc.id] = strings.Join(shares, ", ")
	m.mu.Unlock()
	fmt.Fprintf(m.log, "vm %s: up at %s sharing %s\n", mc.id, mc.addr, strings.Join(shares, ", "))
	return mc, nil
}

func dial(ctx context.Context, addr string, k *keys, exited <-chan struct{}) (*ssh.Client, error) {
	cfg := &ssh.ClientConfig{User: guestUser, Auth: []ssh.AuthMethod{ssh.PublicKeys(k.login)},
		HostKeyCallback: ssh.FixedHostKey(k.host.PublicKey()), HostKeyAlgorithms: []string{ssh.KeyAlgoED25519}, Timeout: 15 * time.Second}
	deadline := time.Now().Add(bootTimeout)
	var last error
	for time.Now().Before(deadline) {
		var d net.Dialer
		conn, err := d.DialContext(ctx, "tcp", addr)
		if err == nil {
			_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
			c, chans, reqs, herr := ssh.NewClientConn(conn, addr, cfg)
			if herr == nil {
				_ = conn.SetDeadline(time.Time{})
				return ssh.NewClient(c, chans, reqs), nil
			}
			conn.Close()
			err = herr
		}
		last = err
		select {
		case <-exited:
			return nil, errors.New("qemu exited while booting")
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return nil, fmt.Errorf("no SSH after %s: %v", bootTimeout, last)
}

type machine struct {
	m      *Manager
	id     string
	dir    string
	addr   string
	shares []string
	q      *qemuProc
	conn   *ssh.Client
	helper *ssh.Session
	stdin  io.WriteCloser
	*sandbox.Client
	once sync.Once
}

func (mc *machine) run(cmd string, stdin io.Reader) error {
	s, err := mc.conn.NewSession()
	if err != nil {
		return err
	}
	defer s.Close()
	s.Stdin = stdin
	out, err := s.CombinedOutput(cmd)
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (mc *machine) install() error {
	path := mc.m.cfg.Helper
	if path == "" {
		var err error
		if path, err = os.Executable(); err != nil {
			return err
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return mc.run("mkdir -p ~/.local/bin && cat > "+guestHelper+".new && chmod 755 "+guestHelper+".new && mv "+guestHelper+".new "+guestHelper, f)
}

func (mc *machine) mount(i int, dir string) error {
	q := quote(dir)
	base := "sudo mkdir -p " + q + " && sudo mount -t 9p -o trans=virtio,version=9p2000.L"
	err := mc.run(base+",msize=1048576 "+shareTag(i)+" "+q, nil)
	if err != nil {
		err = mc.run(base+" "+shareTag(i)+" "+q, nil)
	}
	return err
}

// gitIdentity gives commits made in the VM the host user's name and email from the first share.
func (mc *machine) gitIdentity(shares []string) {
	if len(shares) == 0 {
		return
	}
	for _, key := range []string{"user.name", "user.email"} {
		if v, err := exec.Command("git", "-C", shares[0], "config", key).Output(); err == nil && len(strings.TrimSpace(string(v))) > 0 {
			_ = mc.run("git config --global "+key+" "+quote(strings.TrimSpace(string(v))), nil)
		}
	}
}

func (mc *machine) startHelper() error {
	s, err := mc.conn.NewSession()
	if err != nil {
		return err
	}
	stdin, err := s.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := s.StdoutPipe()
	if err != nil {
		return err
	}
	logf, err := os.Create(filepath.Join(mc.dir, "helper.log"))
	if err == nil {
		s.Stderr = logf
	}
	if err := s.Start("exec " + guestHelper + " vm-agent"); err != nil {
		return err
	}
	mc.helper, mc.stdin = s, stdin
	mc.Client = sandbox.NewClient(stdout, stdin)
	return nil
}

func (mc *machine) Describe() string { return "vm " + mc.id + " (" + mc.addr + ")" }

// Close throws the machine away: nothing in it outlives the task except what it wrote to
// the shared directories.
func (mc *machine) Close() error {
	mc.once.Do(func() {
		mc.teardown()
		<-mc.m.slot
		fmt.Fprintf(mc.m.log, "vm %s: discarded\n", mc.id)
	})
	return nil
}

func (mc *machine) teardown() {
	if mc.stdin != nil {
		mc.stdin.Close()
	}
	if mc.helper != nil {
		mc.helper.Close()
	}
	if mc.conn != nil {
		mc.conn.Close()
	}
	if mc.q != nil {
		mc.q.kill()
	}
	mc.m.mu.Lock()
	delete(mc.m.running, mc.id)
	mc.m.mu.Unlock()
	_ = os.RemoveAll(mc.dir)
}

func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
