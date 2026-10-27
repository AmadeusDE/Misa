package webui

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultPort       = 8787
	defaultSessionTTL = 8 * time.Hour
	challengeTTL      = time.Minute
	// Challenges outstanding per client and in all (the latter only bounds memory).
	maxClientChallenges = 16
	maxChallenges       = 1 << 16
	failWindow          = 15 * time.Minute
)

type Options struct {
	Store DeviceStore
	Title string
	// ListenInterface (e.g. "wg0") binds only to that interface's address on
	// Port and makes New fail when it has none. ListenAddr is used only when
	// ListenInterface is empty (development, tests).
	ListenInterface string
	ListenAddr      string
	Port            int // default 8787, used with ListenInterface
	Now             func() time.Time
	// InterfaceAddrs replaces net.InterfaceByName(...).Addrs() (tests).
	InterfaceAddrs func(name string) ([]net.Addr, error)
	// AuthorizedKeys is an authorized_keys-style file whose ssh-ed25519 keys
	// may log in as devices named by AuthorizedKeyID. Read on every login.
	AuthorizedKeys string
	SessionTTL     time.Duration
	Logf           func(format string, args ...any)
	// MaxTerminals caps browser terminals across all mounts (default 8);
	// TerminalGrace is how long one survives without a stream (default 60s).
	MaxTerminals  int
	TerminalGrace time.Duration
}

type NavItem struct{ Title, Path string }

type Server struct {
	opts   Options
	store  DeviceStore
	listen string
	mux    *http.ServeMux

	mu         sync.Mutex
	nav        []NavItem
	challenges map[string]challenge
	terminals  []*terminalHub
	termActive atomic.Int32

	enrollLimit *limiter
	loginLimit  *limiter
}

func New(o Options) (*Server, error) {
	if o.Store == nil {
		return nil, errors.New("webui: Store is required")
	}
	if o.Title == "" {
		o.Title = "webui"
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Port == 0 {
		o.Port = defaultPort
	}
	if o.SessionTTL == 0 {
		o.SessionTTL = defaultSessionTTL
	}
	if o.Logf == nil {
		o.Logf = log.Printf
	}
	s := &Server{
		opts: o, store: o.Store, mux: http.NewServeMux(),
		challenges: map[string]challenge{},
		// Enrollment codes are 6 digits, so failures also count globally against
		// slow distributed guessing (an attacker can delay enrollments, not logins).
		// Logins are signatures and cannot be guessed: per client only.
		enrollLimit: newLimiter(5, 20, failWindow),
		loginLimit:  newLimiter(10, 0, failWindow),
	}
	addr, err := s.resolveListen()
	if err != nil {
		return nil, err
	}
	s.listen = addr
	s.registerBuiltins()
	return s, nil
}

func (s *Server) resolveListen() (string, error) {
	o := s.opts
	if o.ListenInterface == "" {
		if o.ListenAddr == "" {
			return net.JoinHostPort("0.0.0.0", strconv.Itoa(o.Port)), nil
		}
		return o.ListenAddr, nil
	}
	lookup := o.InterfaceAddrs
	if lookup == nil {
		lookup = interfaceAddrs
	}
	addrs, err := lookup(o.ListenInterface)
	if err != nil {
		return "", fmt.Errorf("webui: interface %q: %w", o.ListenInterface, err)
	}
	ip := pickAddress(addrs)
	if ip == nil {
		return "", fmt.Errorf("webui: interface %q has no usable address", o.ListenInterface)
	}
	return net.JoinHostPort(ip.String(), strconv.Itoa(o.Port)), nil
}

func interfaceAddrs(name string) ([]net.Addr, error) {
	ifc, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}
	return ifc.Addrs()
}

func pickAddress(addrs []net.Addr) net.IP {
	var v6 net.IP
	for _, a := range addrs {
		var ip net.IP
		switch v := a.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip == nil || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
			continue
		}
		if ip.To4() != nil {
			return ip
		}
		if v6 == nil {
			v6 = ip
		}
	}
	return v6
}

// ListenAddress is the resolved host:port Serve binds.
func (s *Server) ListenAddress() string { return s.listen }

func (s *Server) AddNav(title, path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nav = append(s.nav, NavItem{title, path})
}

func (s *Server) navItems() []NavItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]NavItem(nil), s.nav...)
}

// Mount serves h for pattern (http.ServeMux syntax) behind login. Requests
// with a mutating method must carry the session's CSRF token in the
// X-CSRF-Token header (or a csrf_token form field).
func (s *Server) Mount(pattern string, h http.Handler) {
	s.mux.Handle(pattern, s.inContext(s.protect(h)))
}

// MountPublic serves h for pattern without authentication.
func (s *Server) MountPublic(pattern string, h http.Handler) {
	s.mux.Handle(pattern, s.inContext(h))
}

func (s *Server) Handler() http.Handler { return securityHeaders(s.mux) }

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) Listen() (net.Listener, error) { return net.Listen("tcp", s.listen) }

func (s *Server) Serve(ctx context.Context) error {
	ln, err := s.Listen()
	if err != nil {
		return err
	}
	return s.ServeListener(ctx, ln)
}

func (s *Server) ServeListener(ctx context.Context, ln net.Listener) error {
	srv := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}
	s.opts.Logf("webui: %s listening on http://%s/", s.opts.Title, ln.Addr())
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-ctx.Done()
		s.CloseTerminals()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()
	err := srv.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		<-done
		return nil
	}
	return err
}
