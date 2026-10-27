// Package sandbox runs a code session's tools and git on another machine that shares its
// working paths: a helper (Serve) at the far end of a stream, and a Client the daemon's
// agent uses as its tools.Executor and worktree.GitFunc.
package sandbox

import (
	"context"
	"encoding/json"
)

const (
	opTool   = "tool"
	opGit    = "git"
	opCancel = "cancel"
)

type request struct {
	ID uint64 `json:"id"`
	Op string `json:"op"`

	Name    string          `json:"name,omitempty"`
	Cwd     string          `json:"cwd,omitempty"`
	Args    json.RawMessage `json:"args,omitempty"`
	Scratch string          `json:"scratch,omitempty"`
	Blob    string          `json:"blob,omitempty"`
	Recall  bool            `json:"recall,omitempty"`

	Dir  string   `json:"dir,omitempty"`
	Env  []string `json:"env,omitempty"`
	Argv []string `json:"argv,omitempty"`
}

type response struct {
	ID       uint64 `json:"id"`
	Progress string `json:"progress,omitempty"`
	Done     bool   `json:"done,omitempty"`
	Out      string `json:"out,omitempty"`
	Err      string `json:"err,omitempty"`
	BlobText string `json:"blob_text,omitempty"`
	Stdout   []byte `json:"stdout,omitempty"`
	Stderr   []byte `json:"stderr,omitempty"`
}

// Machine is a running sandbox: built-in tools and git run there, on the shared paths.
type Machine interface {
	RunTool(ctx context.Context, name, cwd string, raw json.RawMessage) (string, error)
	Git(ctx context.Context, dir string, env []string, args ...string) ([]byte, []byte, error)
	// Describe is a line for status displays, such as the VM's address.
	Describe() string
	Close() error
}

// Sandbox starts machines that see shares, host directories, at the same paths.
type Sandbox interface {
	Start(ctx context.Context, shares []string) (Machine, error)
}
