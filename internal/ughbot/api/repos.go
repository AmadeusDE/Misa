package api

import (
	"context"
	"errors"
	"net/http"
	"time"
)

var ErrNoGrant = errors.New("no such code grant")

// RepoGrant is a code directory a person approved for a bot, on top of those in its definition.
type RepoGrant struct {
	Bot      string    `json:"bot"`
	Dir      string    `json:"dir"`
	Source   string    `json:"source,omitempty"` // the URL it was cloned from
	Approval int64     `json:"approval,omitempty"`
	Created  time.Time `json:"created"`
}

// Repos are the code directories bots were granted through approvals. A Backend that implements
// it gets the /v1/repos endpoints; others answer 501.
type Repos interface {
	// RepoGrants lists every grant, or those of bot.
	RepoGrants(ctx context.Context, bot string) ([]RepoGrant, error)
	// RevokeRepo takes a grant back and stops the bot's code tasks in it; a clone stays on disk.
	RevokeRepo(ctx context.Context, bot, dir string) error
}

func (s *Server) repos(w http.ResponseWriter) (Repos, bool) {
	r, ok := s.backend.(Repos)
	if !ok {
		writeJSON(w, http.StatusNotImplemented, ErrorBody{Error: "repo grants are not available"})
	}
	return r, ok
}

func (s *Server) listRepos(w http.ResponseWriter, r *http.Request) {
	rp, ok := s.repos(w)
	if !ok {
		return
	}
	list, err := rp.RepoGrants(r.Context(), r.URL.Query().Get("bot"))
	if err != nil {
		writeError(w, err)
		return
	}
	if list == nil {
		list = []RepoGrant{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) revokeRepo(w http.ResponseWriter, r *http.Request) {
	rp, ok := s.repos(w)
	if !ok {
		return
	}
	q := r.URL.Query()
	if err := rp.RevokeRepo(r.Context(), q.Get("bot"), q.Get("dir")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
