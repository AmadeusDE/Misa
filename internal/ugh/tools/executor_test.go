package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
)

type recordingExecutor struct{ calls []string }

func (r *recordingExecutor) RunTool(_ context.Context, name, cwd string, _ json.RawMessage) (string, error) {
	r.calls = append(r.calls, name+"@"+cwd)
	return "remote", nil
}

type ownTool struct{}

func (ownTool) Spec() llm.ToolSpec { return llm.ToolSpec{Name: "own"} }
func (ownTool) Run(context.Context, string, json.RawMessage) (string, error) {
	return "local", nil
}

func TestExecutorTakesOnlyBuiltinTools(t *testing.T) {
	ex := &recordingExecutor{}
	ctx := WithExecutor(context.Background(), ex)
	if out, _ := Run(ctx, bashTool{}, "/repo", json.RawMessage(`{"command":"true"}`)); out != "remote" {
		t.Fatalf("bash ran %q, want the executor", out)
	}
	if out, _ := Run(ctx, ownTool{}, "/repo", nil); out != "local" {
		t.Fatalf("an embedder's tool ran %q, want here", out)
	}
	if len(ex.calls) != 1 || ex.calls[0] != "bash@/repo" {
		t.Fatalf("calls = %v", ex.calls)
	}
	if out, _ := Run(context.Background(), ownTool{}, "/repo", nil); out != "local" {
		t.Fatal("no executor runs here")
	}
}
