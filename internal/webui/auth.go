package webui

import (
	"crypto/ed25519"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

var (
	errBadCode   = errors.New("invalid code")
	errNoTOTP    = errors.New("enrollment is not initialised")
	errDuplicate = errors.New("key already enrolled")
)

const maxBody = 8 << 10

func (s *Server) registerBuiltins() {
	s.MountPublic("GET /auth/challenge", http.HandlerFunc(s.handleChallenge))
	s.MountPublic("POST /auth/enroll", http.HandlerFunc(s.handleEnroll))
	s.MountPublic("POST /auth/login", http.HandlerFunc(s.handleLogin))
	s.Mount("POST /auth/logout", http.HandlerFunc(s.handleLogout))
	s.Mount("GET /auth/session", http.HandlerFunc(s.handleSessionInfo))
	s.Mount("POST /auth/notices/dismiss", http.HandlerFunc(s.handleDismiss))
	s.MountPublic("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticFS())))
	s.MountPublic("GET /login", http.HandlerFunc(s.pageLogin))
	s.MountPublic("GET /enroll", http.HandlerFunc(s.pageEnroll))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		jsonError(w, http.StatusUnsupportedMediaType, "application/json required")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		jsonError(w, http.StatusBadRequest, "bad request")
		return false
	}
	return true
}

type challenge struct {
	expires time.Time
	client  string
}

func (s *Server) handleChallenge(w http.ResponseWriter, r *http.Request) {
	now, client := s.opts.Now(), clientKey(clientIP(r))
	s.mu.Lock()
	mine := 0
	for n, c := range s.challenges {
		switch {
		case !now.Before(c.expires):
			delete(s.challenges, n)
		case c.client == client:
			mine++
		}
	}
	if mine >= maxClientChallenges || len(s.challenges) >= maxChallenges {
		s.mu.Unlock()
		jsonError(w, http.StatusTooManyRequests, "too many challenges")
		return
	}
	nonce := randomHex(32)
	s.challenges[nonce] = challenge{now.Add(challengeTTL), client}
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"nonce": nonce, "expires_in": int(challengeTTL.Seconds())})
}

func (s *Server) consumeChallenge(nonce string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.challenges[nonce]
	delete(s.challenges, nonce)
	return ok && s.opts.Now().Before(c.expires)
}

type enrollRequest struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	PubKey string `json:"pubkey"`
}

func (s *Server) handleEnroll(w http.ResponseWriter, r *http.Request) {
	ip, now := clientIP(r), s.opts.Now()
	client := clientKey(ip)
	if !s.enrollLimit.allow(client, now) {
		jsonError(w, http.StatusTooManyRequests, "locked out, try later")
		return
	}
	var req enrollRequest
	if !readJSON(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	pub, err := base64.StdEncoding.DecodeString(req.PubKey)
	if !validDeviceName(name) || err != nil || len(pub) != ed25519.PublicKeySize {
		jsonError(w, http.StatusBadRequest, "a name (1-64 printable characters) and a base64 ed25519 public key are required")
		return
	}
	id := randomHex(8)
	err = s.store.Update(func(st *State) error {
		if st.TOTP.Secret == "" {
			return errNoTOTP
		}
		for _, d := range st.Devices {
			if subtle.ConstantTimeCompare(d.PubKey, pub) == 1 {
				return errDuplicate
			}
		}
		counter, ok := matchTOTP(st.TOTP.Secret, req.Code, st.TOTP.LastCounter, now)
		if !ok {
			return errBadCode
		}
		st.TOTP.LastCounter = counter
		st.Devices = append(st.Devices, Device{ID: id, Name: name, PubKey: pub, Created: now})
		st.Notices = append(st.Notices, Notice{ID: randomHex(6), At: now, Text: "New device enrolled: " + name})
		st.log(Event{At: now, Kind: "enroll", DeviceID: id, Name: name, IP: ip})
		return nil
	})
	switch {
	case errors.Is(err, errBadCode):
		s.enrollLimit.fail(client, now)
		s.recordFailure("enroll_failed", ip, "")
		jsonError(w, http.StatusUnauthorized, "invalid code")
	case errors.Is(err, errNoTOTP):
		jsonError(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, errDuplicate):
		jsonError(w, http.StatusConflict, err.Error())
	case err != nil:
		jsonError(w, http.StatusInternalServerError, "store error")
	default:
		s.enrollLimit.reset(client)
		writeJSON(w, http.StatusOK, map[string]string{"device_id": id})
	}
}

func (s *Server) recordFailure(kind, ip, deviceID string) {
	_ = s.store.Update(func(st *State) error {
		st.log(Event{At: s.opts.Now(), Kind: kind, IP: ip, DeviceID: deviceID})
		return nil
	})
}

type loginRequest struct {
	DeviceID  string `json:"device_id"`
	Nonce     string `json:"nonce"`
	Signature string `json:"signature"`
}

func (s *Server) devicePubKey(id string) (ed25519.PublicKey, string, bool) {
	var pub ed25519.PublicKey
	var name string
	if strings.HasPrefix(id, "ak-") {
		for _, k := range loadAuthorizedKeys(s.opts.AuthorizedKeys) {
			if k.ID() == id {
				return k.PubKey, k.Comment, true
			}
		}
		return nil, "", false
	}
	ok := false
	_ = s.store.View(func(st *State) {
		if d := st.device(id); d != nil && !d.Revoked && len(d.PubKey) == ed25519.PublicKeySize {
			pub, name, ok = ed25519.PublicKey(append([]byte(nil), d.PubKey...)), d.Name, true
		}
	})
	return pub, name, ok
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip, now := clientIP(r), s.opts.Now()
	client := clientKey(ip)
	if !s.loginLimit.allow(client, now) {
		jsonError(w, http.StatusTooManyRequests, "locked out, try later")
		return
	}
	var req loginRequest
	if !readJSON(w, r, &req) {
		return
	}
	fresh := s.consumeChallenge(req.Nonce)
	sig, err := base64.StdEncoding.DecodeString(req.Signature)
	pub, name, known := s.devicePubKey(req.DeviceID)
	if !fresh || err != nil || !known || !ed25519.Verify(pub, ChallengeMessage(req.Nonce), sig) {
		s.loginLimit.fail(client, now)
		s.recordFailure("login_failed", ip, req.DeviceID)
		jsonError(w, http.StatusUnauthorized, "login failed")
		return
	}
	token, csrf := randomHex(32), randomHex(16)
	sess := Session{ID: hashToken(token), DeviceID: req.DeviceID, CSRF: csrf, IP: ip,
		Created: now, Expires: now.Add(s.opts.SessionTTL)}
	err = s.store.Update(func(st *State) error {
		st.dropSessions(func(x Session) bool { return !now.Before(x.Expires) })
		st.Sessions = append(st.Sessions, sess)
		if d := st.device(req.DeviceID); d != nil {
			d.LastSeen = now
		}
		st.log(Event{At: now, Kind: "login", DeviceID: req.DeviceID, Name: name, IP: ip})
		return nil
	})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "store error")
		return
	}
	s.loginLimit.reset(client)
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: token, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteStrictMode, Expires: sess.Expires})
	writeJSON(w, http.StatusOK, map[string]any{"csrf_token": csrf, "expires": sess.Expires.Format(time.RFC3339)})
}

func (s *Server) sessionID(r *http.Request) string {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return ""
	}
	return hashToken(c.Value)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	id := s.sessionID(r)
	_ = s.store.Update(func(st *State) error {
		st.dropSessions(func(x Session) bool { return x.ID == id })
		return nil
	})
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, SameSite: http.SameSiteStrictMode})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSessionInfo(w http.ResponseWriter, r *http.Request) {
	si, _ := SessionFrom(r)
	writeJSON(w, http.StatusOK, map[string]string{"device_id": si.DeviceID, "device_name": si.DeviceName, "csrf_token": si.CSRF})
}

func (s *Server) handleDismiss(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	si, _ := SessionFrom(r)
	_ = s.store.Update(func(st *State) error {
		for i := range st.Notices {
			if st.Notices[i].ID == req.ID {
				st.Notices[i].Dismissed = append(st.Notices[i].Dismissed, si.DeviceID)
			}
		}
		return nil
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
