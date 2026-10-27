package tools

import (
	"context"
	"encoding/json"
)

// Executor runs the built-in tools somewhere else, such as a VM that shares the working
// paths. Tools added by the embedder still run here.
type Executor interface {
	RunTool(ctx context.Context, name, cwd string, raw json.RawMessage) (string, error)
}

type executorKey struct{}

// WithExecutor sends the built-in tool calls made under ctx to ex.
func WithExecutor(ctx context.Context, ex Executor) context.Context {
	return context.WithValue(ctx, executorKey{}, ex)
}

// Run runs t, through the executor in ctx when t is a built-in tool.
func Run(ctx context.Context, t Tool, cwd string, raw json.RawMessage) (string, error) {
	if ex, ok := ctx.Value(executorKey{}).(Executor); ok && ex != nil && builtin(t) {
		return ex.RunTool(ctx, t.Spec().Name, cwd, raw)
	}
	return t.Run(ctx, cwd, raw)
}

func builtin(t Tool) bool {
	switch t.(type) {
	case readTool, writeTool, editTool, lsTool, grepTool, findTool, bashTool:
		return true
	}
	return false
}

// Scratch is the scratch directory in ctx, without creating it.
func Scratch(ctx context.Context) string {
	dir, _ := ctx.Value(scratchKey{}).(string)
	return dir
}

// BlobOf is the blob destination in ctx.
func BlobOf(ctx context.Context) Blob {
	b, _ := ctx.Value(blobKey{}).(Blob)
	return b
}

// ProgressOf is the progress callback in ctx, or nil.
func ProgressOf(ctx context.Context) func(string) {
	fn, _ := ctx.Value(progressKey{}).(func(string))
	return fn
}
