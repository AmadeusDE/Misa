package api

import (
	"context"
	"net/http"
	"strconv"
)

// Routines are scheduled mail to bots. A Backend that implements it gets the /v1/routines
// endpoints; others answer 501.
type Routines interface {
	// Routines lists every routine, or those mailing bot.
	Routines(ctx context.Context, bot string) ([]RoutineInfo, error)
	AddRoutine(ctx context.Context, req RoutineRequest) (RoutineInfo, error)
	PauseRoutine(ctx context.Context, id int64, paused bool) (RoutineInfo, error)
	DeleteRoutine(ctx context.Context, id int64) error
}

func (s *Server) routines(w http.ResponseWriter) (Routines, bool) {
	r, ok := s.backend.(Routines)
	if !ok {
		writeJSON(w, http.StatusNotImplemented, ErrorBody{Error: "routines are not available"})
	}
	return r, ok
}

func routineID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorBody{Error: "routine id must be a number"})
		return 0, false
	}
	return id, true
}

func (s *Server) listRoutines(w http.ResponseWriter, r *http.Request) {
	rt, ok := s.routines(w)
	if !ok {
		return
	}
	list, err := rt.Routines(r.Context(), r.URL.Query().Get("bot"))
	if err != nil {
		writeError(w, err)
		return
	}
	if list == nil {
		list = []RoutineInfo{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) addRoutine(w http.ResponseWriter, r *http.Request) {
	rt, ok := s.routines(w)
	if !ok {
		return
	}
	var req RoutineRequest
	if !decodeBody(w, r, &req) {
		return
	}
	info, err := rt.AddRoutine(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, info)
}

func (s *Server) pauseRoutine(paused bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rt, ok := s.routines(w)
		if !ok {
			return
		}
		id, ok := routineID(w, r)
		if !ok {
			return
		}
		info, err := rt.PauseRoutine(r.Context(), id, paused)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, info)
	}
}

func (s *Server) deleteRoutine(w http.ResponseWriter, r *http.Request) {
	rt, ok := s.routines(w)
	if !ok {
		return
	}
	id, ok := routineID(w, r)
	if !ok {
		return
	}
	if err := rt.DeleteRoutine(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
