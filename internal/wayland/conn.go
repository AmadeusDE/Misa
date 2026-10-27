package wayland

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"golang.org/x/sys/unix"
)

// Display is the wl_display singleton, object 1.
const Display uint32 = 1

const (
	displaySync        = 0
	displayGetRegistry = 1
	displayEventError  = 0
	displayEventDelete = 1
	registryBind       = 0
	registryGlobal     = 0
	callbackDone       = 0
	maxFDsPerRead      = 28
)

// Handler receives the events of one object.
type Handler func(ev *Event)

// Conn is a client connection. Requests may be sent from any goroutine;
// events are delivered on Events and must be passed to Dispatch by one
// goroutine.
type Conn struct {
	sock     *net.UnixConn
	wmu      sync.Mutex
	next     uint32
	free     []uint32
	handlers map[uint32]Handler
	fdmu     sync.Mutex
	fdq      []int
	events   chan Event
	err      error
	readErr  error
}

// Connect opens $WAYLAND_SOCKET or the socket named by $WAYLAND_DISPLAY.
func Connect() (*Conn, error) {
	if fd, err := strconv.Atoi(os.Getenv("WAYLAND_SOCKET")); err == nil {
		os.Unsetenv("WAYLAND_SOCKET")
		f := os.NewFile(uintptr(fd), "wayland")
		nc, err := net.FileConn(f) // dups the fd
		f.Close()
		if err != nil {
			return nil, err
		}
		return newConn(nc.(*net.UnixConn)), nil
	}
	path, err := socketPath()
	if err != nil {
		return nil, err
	}
	nc, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, err
	}
	return newConn(nc), nil
}

func socketPath() (string, error) {
	name := os.Getenv("WAYLAND_DISPLAY")
	if name == "" {
		name = "wayland-0"
	}
	if filepath.IsAbs(name) {
		return name, nil
	}
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		return "", errors.New("wayland: XDG_RUNTIME_DIR is not set")
	}
	return filepath.Join(dir, name), nil
}

func newConn(sock *net.UnixConn) *Conn {
	c := &Conn{
		sock:     sock,
		next:     Display + 1,
		handlers: map[uint32]Handler{},
		events:   make(chan Event, 256),
	}
	c.handlers[Display] = c.displayEvent
	go c.readLoop()
	return c
}

func (c *Conn) Close() error { return c.sock.Close() }

// Events delivers incoming messages; it is closed when the socket fails.
func (c *Conn) Events() <-chan Event { return c.events }

// Err is the protocol error the compositor reported, or the read error that
// closed Events.
func (c *Conn) Err() error {
	if c.err != nil {
		return c.err
	}
	return c.readErr
}

// Send writes one request.
func (c *Conn) Send(obj uint32, op uint16, args ...any) error {
	b, fds := encode(obj, op, args)
	var oob []byte
	if len(fds) > 0 {
		oob = unix.UnixRights(fds...)
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_, _, err := c.sock.WriteMsgUnix(b, oob, nil)
	return err
}

// NewObject allocates a client object id whose events go to h.
func (c *Conn) NewObject(h Handler) uint32 {
	var id uint32
	if n := len(c.free); n > 0 {
		id, c.free = c.free[n-1], c.free[:n-1]
	} else {
		id = c.next
		c.next++
	}
	c.handlers[id] = h
	return id
}

// SetHandler routes events of id, such as an object the compositor created.
// A nil h ignores them.
func (c *Conn) SetHandler(id uint32, h Handler) {
	if h == nil {
		delete(c.handlers, id)
		return
	}
	c.handlers[id] = h
}

// Dispatch hands ev to its object's handler.
func (c *Conn) Dispatch(ev Event) {
	ev.fds = c.popFD
	if h := c.handlers[ev.Obj]; h != nil {
		h(&ev)
	}
}

func (c *Conn) displayEvent(ev *Event) {
	switch ev.Op {
	case displayEventError:
		obj, code, msg := ev.Object(), ev.Uint(), ev.Text()
		c.err = fmt.Errorf("wayland: protocol error on object %d (code %d): %s", obj, code, msg)
	case displayEventDelete:
		id := ev.Uint()
		delete(c.handlers, id)
		c.free = append(c.free, id)
	}
}

func (c *Conn) readLoop() {
	defer close(c.events)
	buf := make([]byte, 0, 64*1024)
	chunk := make([]byte, 32*1024)
	oob := make([]byte, unix.CmsgSpace(maxFDsPerRead*4))
	for {
		n, oobn, _, _, err := c.sock.ReadMsgUnix(chunk, oob)
		n, oobn = max(n, 0), max(oobn, 0)
		if oobn > 0 {
			c.pushFDs(oob[:oobn])
		}
		buf = append(buf, chunk[:n]...)
		for {
			ev, size, ok := splitMessage(buf)
			if !ok {
				break
			}
			buf = buf[size:]
			c.events <- ev
		}
		buf = append(buf[:0:0], buf...)
		if err != nil {
			c.readErr = err
			return
		}
		if n == 0 && oobn == 0 {
			c.readErr = errors.New("wayland: compositor closed the connection")
			return
		}
	}
}

func (c *Conn) pushFDs(oob []byte) {
	msgs, err := unix.ParseSocketControlMessage(oob)
	if err != nil {
		return
	}
	c.fdmu.Lock()
	defer c.fdmu.Unlock()
	for i := range msgs {
		if fds, err := unix.ParseUnixRights(&msgs[i]); err == nil {
			c.fdq = append(c.fdq, fds...)
		}
	}
}

func (c *Conn) popFD() (int, bool) {
	c.fdmu.Lock()
	defer c.fdmu.Unlock()
	if len(c.fdq) == 0 {
		return -1, false
	}
	fd := c.fdq[0]
	c.fdq = c.fdq[1:]
	return fd, true
}

// Global is an interface the compositor advertises.
type Global struct {
	Name    uint32
	Version uint32
}

// Registry is the compositor's global list, filled by Roundtrip.
type Registry struct {
	c       *Conn
	id      uint32
	Globals map[string]Global
}

// GetRegistry requests the global list; call Roundtrip to receive it.
func (c *Conn) GetRegistry() *Registry {
	r := &Registry{c: c, Globals: map[string]Global{}}
	r.id = c.NewObject(func(ev *Event) {
		if ev.Op == registryGlobal {
			name, iface, ver := ev.Uint(), ev.Text(), ev.Uint()
			r.Globals[iface] = Global{name, ver}
		}
	})
	c.Send(Display, displayGetRegistry, r.id)
	return r
}

// Bind binds iface at min(version, advertised), returning the object id and
// the bound version, or 0 if the compositor lacks it.
func (r *Registry) Bind(iface string, version uint32, h Handler) (uint32, uint32) {
	g, ok := r.Globals[iface]
	if !ok {
		return 0, 0
	}
	version = min(version, g.Version)
	id := r.c.NewObject(h)
	r.c.Send(r.id, registryBind, g.Name, iface, version, id)
	return id, version
}

// Roundtrip dispatches events until the compositor has handled every request
// sent so far.
func (c *Conn) Roundtrip() error {
	done := false
	cb := c.NewObject(func(ev *Event) { done = ev.Op == callbackDone })
	if err := c.Send(Display, displaySync, cb); err != nil {
		return err
	}
	for !done {
		ev, ok := <-c.events
		if !ok {
			return c.Err()
		}
		c.Dispatch(ev)
		if c.err != nil {
			return c.err
		}
	}
	return nil
}
