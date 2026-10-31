//go:build unix

package wayland

import (
	"net"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func socketPair(t *testing.T) (local, peer *net.UnixConn) {
	t.Helper()
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	conn := func(fd int) *net.UnixConn {
		f := os.NewFile(uintptr(fd), "wayland-test")
		defer f.Close()
		c, err := net.FileConn(f)
		if err != nil {
			t.Fatal(err)
		}
		return c.(*net.UnixConn)
	}
	return conn(fds[0]), conn(fds[1])
}

func TestCompositorResetEndsEvents(t *testing.T) {
	local, peer := socketPair(t)
	c := newConn(local)
	defer c.Close()
	if _, err := local.Write(make([]byte, 64)); err != nil {
		t.Fatal(err)
	}
	peer.Close()

	select {
	case _, open := <-c.Events():
		if open {
			t.Fatal("unexpected event")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Events did not close after the compositor went away")
	}
	if c.readErr == nil {
		t.Fatal("readErr not set")
	}
}
