package api

import (
	"context"
	"net/http"
	"strconv"
)

// Approvals is the queue of actions bots asked for that wait on a person. A Backend that
// implements it gets the /v1/approvals endpoints; others answer 501.
type Approvals interface {
	// Approvals lists requests in state ("pending", ...), or every request for "".
	Approvals(ctx context.Context, state string) ([]ApprovalInfo, error)
	Approve(ctx context.Context, id int64, by string) (ApprovalInfo, error)
	Deny(ctx context.Context, id int64, by string) (ApprovalInfo, error)
}

func (s *Server) approvals(w http.ResponseWriter) (Approvals, bool) {
	a, ok := s.backend.(Approvals)
	if !ok {
		writeJSON(w, http.StatusNotImplemented, ErrorBody{Error: "approvals are not available"})
	}
	return a, ok
}

func (s *Server) listApprovals(w http.ResponseWriter, r *http.Request) {
	a, ok := s.approvals(w)
	if !ok {
		return
	}
	list, err := a.Approvals(r.Context(), r.URL.Query().Get("state"))
	if err != nil {
		writeError(w, err)
		return
	}
	if list == nil {
		list = []ApprovalInfo{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) decide(approve bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := s.approvals(w)
		if !ok {
			return
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorBody{Error: "approval id must be a number"})
			return
		}
		var req DecideRequest
		if !decodeBody(w, r, &req) {
			return
		}
		decide := a.Deny
		if approve {
			decide = a.Approve
		}
		info, err := decide(r.Context(), id, req.By)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, info)
	}
}
