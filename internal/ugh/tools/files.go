// read, write, edit: bounded file tools relative to the agent cwd.
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
)

const maxFileBytes = 4 << 20

// ---- read ----

type readTool struct{}

func (readTool) Spec() llm.ToolSpec {
	return spec("read", "Read a text file. Paths are relative to the working directory.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":   map[string]any{"type": "string", "description": "file to read"},
				"offset": map[string]any{"type": "integer", "description": "1-based start line"},
				"limit":  map[string]any{"type": "integer", "description": "max lines (default 2000)"},
			},
			"required": []string{"path"},
		})
}

type readArgs struct {
	Path   string `json:"path"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
}

func (readTool) Run(_ context.Context, cwd string, raw json.RawMessage) (string, error) {
	var a readArgs
	if err := decode(raw, &a); err != nil {
		return "", err
	}
	if strings.TrimSpace(a.Path) == "" {
		return "", fmt.Errorf("read: path must not be empty (args: %.200s); pass the file to read as {\"path\": \"<file>\"}", string(raw))
	}
	if fi, err := os.Stat(resolve(cwd, a.Path)); err == nil && fi.IsDir() {
		return "", fmt.Errorf("read %s: is a directory, not a file; use the ls tool to list its entries or read a file inside it", a.Path)
	}
	data, err := readRegular(resolve(cwd, a.Path))
	if err != nil {
		return "", fmt.Errorf("read %s: %v", a.Path, err)
	}
	if len(data) > maxFileBytes {
		data = data[:maxFileBytes]
	}
	if a.Offset < 1 {
		a.Offset = 1
	}
	if a.Limit < 1 {
		a.Limit = 2000
	}
	if !isText(data) {
		return fmt.Sprintf("%s: binary file skipped", a.Path), nil
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	total := len(lines)
	if total == 0 {
		return fmt.Sprintf("%s: empty file", a.Path), nil
	}
	if a.Offset > total {
		return "", fmt.Errorf("read: offset %d beyond end of file (%d lines)", a.Offset, total)
	}
	end := a.Offset - 1 + a.Limit
	if end > total {
		end = total
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s (lines %d-%d of %d)", a.Path, a.Offset, end, total)
	if end < total {
		fmt.Fprintf(&b, " [truncated; read again with offset=%d]", end+1)
	}
	for i := a.Offset - 1; i < end; i++ {
		fmt.Fprintf(&b, "\n%4d  %s", i+1, lines[i])
	}
	return b.String(), nil
}

func isText(b []byte) bool {
	if bytes.IndexByte(b, 0) >= 0 {
		return false
	}
	// Check only a prefix; invalid UTF-8 in a large binary tail is fine.
	return utf8Prefix(b)
}

func utf8Prefix(b []byte) bool {
	p := b
	if len(p) > 8192 {
		p = p[:8192]
	}
	return utf8.Valid(p)
}

// ---- write ----

type writeTool struct{}

func (writeTool) Spec() llm.ToolSpec {
	return spec("write", "Create or overwrite a text file with the given content. Parent directories are created.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":    map[string]any{"type": "string"},
				"content": map[string]any{"type": "string"},
			},
			"required": []string{"path", "content"},
		})
}

type writeArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (writeTool) Run(_ context.Context, cwd string, raw json.RawMessage) (string, error) {
	var a writeArgs
	if err := decode(raw, &a); err != nil {
		return "", err
	}
	path := resolve(cwd, a.Path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("write: %v", err)
	}
	if err := writeIn(path, []byte(a.Content)); err != nil {
		return "", fmt.Errorf("write %s: %v", a.Path, err)
	}
	return fmt.Sprintf("wrote %s (%d bytes)", a.Path, len(a.Content)), nil
}

// ---- edit ----

type editTool struct{}

func (editTool) Spec() llm.ToolSpec {
	return spec("edit", "Replace exactly one occurrence of old_text in a file with new_text.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":     map[string]any{"type": "string"},
				"old_text": map[string]any{"type": "string", "description": "text to replace (must occur exactly once)"},
				"new_text": map[string]any{"type": "string"},
			},
			"required": []string{"path", "old_text", "new_text"},
		})
}

type editArgs struct {
	Path    string `json:"path"`
	OldText string `json:"old_text"`
	NewText string `json:"new_text"`
}

func (editTool) Run(_ context.Context, cwd string, raw json.RawMessage) (string, error) {
	var a editArgs
	if err := decode(raw, &a); err != nil {
		return "", err
	}
	path := resolve(cwd, a.Path)
	data, err := readRegular(path)
	if err != nil {
		return "", fmt.Errorf("edit %s: %v", a.Path, err)
	}
	if len(data) > maxFileBytes {
		return "", fmt.Errorf("edit %s: file exceeds the %d-byte edit bound", a.Path, maxFileBytes)
	}
	if a.OldText == "" {
		return "", fmt.Errorf("edit: old_text must not be empty")
	}
	count := bytes.Count(data, []byte(a.OldText))
	switch {
	case count == 0:
		return "", fmt.Errorf("edit %s: old_text not found; no change applied", a.Path)
	case count > 1:
		return "", fmt.Errorf("edit %s: old_text matches %d times; provide a longer unique string; no change applied", a.Path, count)
	}
	updated := bytes.Replace(data, []byte(a.OldText), []byte(a.NewText), 1)
	if err := writeIn(path, updated); err != nil {
		return "", fmt.Errorf("edit %s: %v", a.Path, err)
	}
	return fmt.Sprintf("edited %s (%d -> %d bytes)", a.Path, len(data), len(updated)), nil
}

// readRegular reads at most maxFileBytes+1 bytes of a regular file. Devices
// and FIFOs are refused before they are opened: /dev/zero never ends and a
// FIFO blocks until someone writes.
func readRegular(path string) ([]byte, error) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer root.Close()
	name := filepath.Base(path)
	if err := regular(root.Stat(name)); err != nil {
		return nil, err
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if err := regular(f.Stat()); err != nil {
		return nil, err
	}
	return limitRead(f, maxFileBytes+1)
}

func regular(fi os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("not a regular file (%s)", fi.Mode().Type())
	}
	return nil
}

// writeIn writes a file in the directory resolve chose without following a
// symlink out of it: a link planted or swapped in after the permission check
// can't redirect the write.
func writeIn(path string, data []byte) error {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer root.Close()
	return root.WriteFile(filepath.Base(path), data, 0o644)
}
