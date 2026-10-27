package api

import (
	"context"
	"net/http"
)

// BotAdmin creates, edits and deletes bots at runtime. A Backend that implements it
// gets the /v1/bots management endpoints; others answer 501.
type BotAdmin interface {
	BotDef(ctx context.Context, name string) (BotDef, error)
	CreateBot(ctx context.Context, def BotDef) error
	UpdateBot(ctx context.Context, def BotDef) error
	DeleteBot(ctx context.Context, name string) error
}

func (s *Server) botAdmin(w http.ResponseWriter) (BotAdmin, bool) {
	a, ok := s.backend.(BotAdmin)
	if !ok {
		writeJSON(w, http.StatusNotImplemented, ErrorBody{Error: "bot management is not available"})
	}
	return a, ok
}

func (s *Server) getBotDef(w http.ResponseWriter, r *http.Request) {
	a, ok := s.botAdmin(w)
	if !ok {
		return
	}
	def, err := a.BotDef(r.Context(), r.PathValue("bot"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, def)
}

func (s *Server) createBot(w http.ResponseWriter, r *http.Request) {
	a, ok := s.botAdmin(w)
	if !ok {
		return
	}
	var def BotDef
	if !decodeBody(w, r, &def) {
		return
	}
	if err := a.CreateBot(r.Context(), def); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) updateBot(w http.ResponseWriter, r *http.Request) {
	a, ok := s.botAdmin(w)
	if !ok {
		return
	}
	var def BotDef
	if !decodeBody(w, r, &def) {
		return
	}
	def.Name = r.PathValue("bot")
	if err := a.UpdateBot(r.Context(), def); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteBot(w http.ResponseWriter, r *http.Request) {
	a, ok := s.botAdmin(w)
	if !ok {
		return
	}
	if err := a.DeleteBot(r.Context(), r.PathValue("bot")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
