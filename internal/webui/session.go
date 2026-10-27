package webui

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"strings"
	"unicode"
)

const (
	cookieName = "webui_session"
	csrfHeader = "X-CSRF-Token"
)

type ctxKey int

const (
	serverKey ctxKey = iota
	sessionKey
)

// SessionInfo describes the authenticated caller inside a Mount'ed handler.
type SessionInfo struct {
	DeviceID   string
	DeviceName string
	CSRF       string
}

func SessionFrom(r *http.Request) (SessionInfo, bool) {
	si, ok := r.Context().Value(sessionKey).(SessionInfo)
	return si, ok
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) inContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), serverKey, s)
		if si, ok := s.authenticate(r); ok {
			ctx = context.WithValue(ctx, sessionKey, si)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// authenticate resolves the session cookie. A session is valid only while it
// is unexpired and its device is neither revoked nor removed.
func (s *Server) authenticate(r *http.Request) (SessionInfo, bool) {
	c, err := r.Cookie(cookieName)
	if err != nil || c.Value == "" {
		return SessionInfo{}, false
	}
	id := hashToken(c.Value)
	now := s.opts.Now()
	var found Session
	var name string
	var ok bool
	_ = s.store.View(func(st *State) {
		for _, x := range st.Sessions {
			if x.ID == id && now.Before(x.Expires) {
				found, ok = x, true
				break
			}
		}
		if !ok {
			return
		}
		if d := st.device(found.DeviceID); d != nil {
			name, ok = d.Name, !d.Revoked
		} else {
			ok = false
		}
	})
	if !ok && found.ID != "" && strings.HasPrefix(found.DeviceID, "ak-") {
		for _, k := range loadAuthorizedKeys(s.opts.AuthorizedKeys) {
			if k.ID() == found.DeviceID {
				return SessionInfo{DeviceID: found.DeviceID, DeviceName: k.Comment, CSRF: found.CSRF}, true
			}
		}
	}
	if !ok {
		return SessionInfo{}, false
	}
	return SessionInfo{DeviceID: found.DeviceID, DeviceName: name, CSRF: found.CSRF}, true
}

func isMutating(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return false
	}
	return true
}

func (s *Server) protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		si, ok := SessionFrom(r)
		if !ok {
			s.denyAnonymous(w, r)
			return
		}
		if isMutating(r.Method) && !csrfOK(r, si.CSRF) {
			http.Error(w, "csrf token missing or wrong", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func csrfOK(r *http.Request, want string) bool {
	got := r.Header.Get(csrfHeader)
	if got == "" {
		got = r.PostFormValue("csrf_token")
	}
	return want != "" && subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func (s *Server) denyAnonymous(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && strings.Contains(r.Header.Get("Accept"), "text/html") {
		http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
		return
	}
	http.Error(w, "login required", http.StatusUnauthorized)
}

func safeNext(next string) string {
	// Browsers drop tabs and newlines from URLs, so "/\t/evil.com" would become "//evil.com".
	if strings.HasPrefix(next, "/") && !strings.HasPrefix(next, "//") && !strings.Contains(next, "\\") && !strings.ContainsFunc(next, unicode.IsControl) {
		return next
	}
	return "/"
}
