package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"
)

const (
	defaultHeartbeat    = 15 * time.Second
	defaultWriteTimeout = 10 * time.Second
	maxBodyBytes        = 1 << 20
)

type Server struct {
	backend      Backend
	Heartbeat    time.Duration
	WriteTimeout time.Duration
	// Web serves /v1/web/*; when nil the backend is asked, else those endpoints say the web UI is not enabled.
	Web  WebAdmin
	done chan struct{}
}

func NewServer(b Backend) *Server {
	return &Server{backend: b, Heartbeat: defaultHeartbeat, WriteTimeout: defaultWriteTimeout, done: make(chan struct{})}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/bots", s.listBots)
	mux.HandleFunc("GET /v1/status", s.status)
	mux.HandleFunc("GET /v1/cost", s.cost)
	mux.HandleFunc("POST /v1/stop", s.stop)
	mux.HandleFunc("POST /v1/bots/{bot}/send", s.send)
	mux.HandleFunc("GET /v1/bots/{bot}/events", s.events)
	mux.HandleFunc("POST /v1/bots", s.createBot)
	mux.HandleFunc("GET /v1/bots/{bot}", s.getBotDef)
	mux.HandleFunc("PUT /v1/bots/{bot}", s.updateBot)
	mux.HandleFunc("DELETE /v1/bots/{bot}", s.deleteBot)
	mux.HandleFunc("GET /v1/approvals", s.listApprovals)
	mux.HandleFunc("POST /v1/approvals/{id}/approve", s.decide(true))
	mux.HandleFunc("POST /v1/approvals/{id}/deny", s.decide(false))
	mux.HandleFunc("GET /v1/routines", s.listRoutines)
	mux.HandleFunc("POST /v1/routines", s.addRoutine)
	mux.HandleFunc("POST /v1/routines/{id}/pause", s.pauseRoutine(true))
	mux.HandleFunc("POST /v1/routines/{id}/resume", s.pauseRoutine(false))
	mux.HandleFunc("DELETE /v1/routines/{id}", s.deleteRoutine)
	mux.HandleFunc("GET /v1/repos", s.listRepos)
	mux.HandleFunc("DELETE /v1/repos", s.revokeRepo)
	s.registerWebAdmin(mux)
	return mux
}

// Serve runs until ctx ends, then ends open event streams and shuts down.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	srv := &http.Server{Handler: s.Handler()}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	close(s.done)
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		srv.Close()
	}
	<-errc
	return nil
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrUnknownBot), errors.Is(err, ErrNoApproval), errors.Is(err, ErrNoRoutine), errors.Is(err, ErrNoGrant):
		code = http.StatusNotFound
	case errors.Is(err, ErrBotExists), errors.Is(err, ErrNotPending):
		code = http.StatusConflict
	case errors.Is(err, ErrReadOnly):
		code = http.StatusForbidden
	case errors.Is(err, ErrInvalid), errors.Is(err, ErrBadRoutine):
		code = http.StatusBadRequest
	case errors.Is(err, ErrWebDisabled):
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, ErrorBody{Error: err.Error()})
}

func (s *Server) listBots(w http.ResponseWriter, r *http.Request) {
	bots, err := s.backend.ListBots(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if bots == nil {
		bots = []BotInfo{}
	}
	writeJSON(w, http.StatusOK, bots)
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	st, err := s.backend.Status(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) cost(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	rows, err := s.backend.Cost(r.Context(), CostQuery{Bot: q.Get("bot"), From: q.Get("from"), To: q.Get("to")})
	if err != nil {
		writeError(w, err)
		return
	}
	if rows == nil {
		rows = []CostRow{}
	}
	writeJSON(w, http.StatusOK, CostReport{Rows: rows})
}

func (s *Server) stop(w http.ResponseWriter, r *http.Request) {
	if err := s.backend.Stop(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) send(w http.ResponseWriter, r *http.Request) {
	var req SendRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Text == "" {
		writeJSON(w, http.StatusBadRequest, ErrorBody{Error: "text is required"})
		return
	}
	ticket, err := s.backend.Send(r.Context(), r.PathValue("bot"), req.Text)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, SendResponse{Ticket: ticket})
}

func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorBody{Error: "invalid JSON body"})
		return false
	}
	return true
}

func resumeSeq(r *http.Request) (uint64, error) {
	raw := r.Header.Get("Last-Event-ID")
	if raw == "" {
		raw = r.URL.Query().Get("after")
	}
	if raw == "" {
		return 0, nil
	}
	return strconv.ParseUint(raw, 10, 64)
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	after, err := resumeSeq(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorBody{Error: "invalid Last-Event-ID"})
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	go func() {
		select {
		case <-s.done:
			cancel()
		case <-ctx.Done():
		}
	}()
	events, err := s.backend.Subscribe(ctx, r.PathValue("bot"), after)
	if err != nil {
		writeError(w, err)
		return
	}
	rc := http.NewResponseController(w)
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if !s.flush(rc, w, ": open\n\n") {
		return
	}
	tick := time.NewTicker(s.Heartbeat)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if !s.flush(rc, w, ": ping\n\n") {
				return
			}
		case ev, ok := <-events:
			if !ok {
				return
			}
			data, _ := json.Marshal(ev)
			if !s.flush(rc, w, fmt.Sprintf("id: %d\nevent: %s\ndata: %s\n\n", ev.Seq, ev.Type, data)) {
				return
			}
		}
	}
}

// flush drops a client that stops reading, so one stuck stream holds nothing else.
func (s *Server) flush(rc *http.ResponseController, w http.ResponseWriter, frame string) bool {
	rc.SetWriteDeadline(time.Now().Add(s.WriteTimeout))
	if _, err := fmt.Fprint(w, frame); err != nil {
		return false
	}
	return rc.Flush() == nil
}
