package ui

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/AmadeusDE/Misa/internal/imgview"
)

func TestImageViewerWords(t *testing.T) {
	for _, tt := range []struct {
		line  string
		words []string
		ok    bool
	}{
		{`imgview --thumbnail 'a b.png' "c|d.png"`, []string{"imgview", "--thumbnail", "a b.png", "c|d.png"}, true},
		{`imgview a\ b.png "a\z.png" ''`, []string{"imgview", "a b.png", `a\z.png`, ""}, true},
		{`imgview "$HOME/a"`, nil, false},
		{`imgview a | cat`, nil, false},
		{`imgview 'unfinished`, nil, false},
		{`imgview *.png`, nil, false},
	} {
		got, ok := imageViewerWords(tt.line)
		if ok != tt.ok || !reflect.DeepEqual(got, tt.words) {
			t.Errorf("%q: got %#v %v", tt.line, got, ok)
		}
	}
}

func forceImageViewerFallback(t *testing.T) {
	t.Helper()
	old := imageViewerExecutable
	imageViewerExecutable = func() (string, error) { return "", errors.New("test unavailable") }
	t.Cleanup(func() { imageViewerExecutable = old })
}

func TestImageViewerDirectoryOperand(t *testing.T) {
	forceImageViewerFallback(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	for _, name := range []string{"b.png", "a.png", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "c.png"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	m := New(dir)
	m.width, m.height = 80, 24
	if ok, _ := m.handleBuiltinCmd("imgview ."); !ok {
		t.Fatal("directory operand did not dispatch")
	}
	v, ok := m.procViewer.(*imgview.Viewer)
	if !ok {
		t.Fatalf("not an image viewer: %T", m.procViewer)
	}
	if got := v.Paths(); !reflect.DeepEqual(got, []string{filepath.Join(dir, "a.png"), filepath.Join(dir, "b.png")}) {
		t.Fatalf("directory expansion: %v", got)
	}
	recursive := New(dir)
	recursive.width, recursive.height = 80, 24
	if ok, _ := recursive.handleBuiltinCmd("imgview -r ."); !ok {
		t.Fatal("recursive operand did not dispatch")
	}
	v, _ = recursive.procViewer.(*imgview.Viewer)
	if v == nil {
		t.Fatalf("not an image viewer: %T", recursive.procViewer)
	}
	if got := v.Paths(); len(got) != 3 || got[2] != filepath.Join(dir, "sub", "c.png") {
		t.Fatalf("recursive expansion: %v", got)
	}
}

func TestImageViewerDispatch(t *testing.T) {
	forceImageViewerFallback(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := New(t.TempDir())
	m.width, m.height = 80, 24
	m.cmdInput.SetValue(`imgview 'a b.png'`)
	if ok, _ := m.handleBuiltinCmd(`imgview 'a b.png'`); !ok || m.procViewer == nil {
		t.Fatal("did not enter image viewer")
	}
	if m.cmdInput.Value() != "" {
		t.Error("input not cleared")
	}
	for _, line := range []string{"imgview --help", "imgview x | cat"} {
		other := New(t.TempDir())
		if ok, _ := other.handleBuiltinCmd(line); ok {
			t.Errorf("%q should fall through", line)
		}
	}
	other := New(t.TempDir())
	if ok, _ := other.handleBuiltinCmd("imgview --renderer=kitty 'x y'"); !ok {
		t.Fatal("kitty did not dispatch")
	}
	assertKittyFallback(t, other)
	bad := New(t.TempDir())
	if ok, _ := bad.handleBuiltinCmd("imgview --renderer=bad x"); !ok || bad.lastExitCode != 1 || !bufferHas(bad, "unknown renderer") {
		t.Fatal("invalid arguments should show an in-process diagnostic")
	}
}

func TestImageViewerDispatchConfig(t *testing.T) {
	forceImageViewerFallback(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "misa", "imageviewer.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("initial_mode: thumbnails\nrenderer: kitty\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m := New(t.TempDir())
	if ok, _ := m.handleBuiltinCmd("imgview --renderer=halfblock a.png"); !ok {
		t.Fatal("did not dispatch")
	}
	v, ok := m.procViewer.(*imgview.Viewer)
	if !ok || !v.Thumbnail() {
		t.Fatal("configured thumbnail mode not applied in UI")
	}
	other := New(t.TempDir())
	if ok, _ := other.handleBuiltinCmd("imgview a.png"); !ok {
		t.Fatal("configured renderer did not dispatch")
	}
	assertKittyFallback(t, other)
}

func assertKittyFallback(t *testing.T, m *Model) {
	t.Helper()
	v, ok := m.procViewer.(*imgview.Viewer)
	if !ok {
		t.Fatal("expected image viewer")
	}
	v.Resize(100, 4)
	if !strings.Contains(v.View(), "kitty unavailable; using halfblock") {
		t.Fatal("fallback warning missing")
	}
}

func TestImageViewerSelfExec(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	exe := filepath.Join(dir, "libmisa.so")
	if err := os.WriteFile(exe, []byte("test"), 0755); err != nil {
		t.Fatal(err)
	}
	oldExe, oldExec := imageViewerExecutable, imageViewerExecProcess
	t.Cleanup(func() { imageViewerExecutable, imageViewerExecProcess = oldExe, oldExec })
	imageViewerExecutable = func() (string, error) { return exe, nil }
	var got *exec.Cmd
	var callback tea.ExecCallback
	imageViewerExecProcess = func(c *exec.Cmd, cb tea.ExecCallback) tea.Cmd {
		got, callback = c, cb
		return func() tea.Msg { return nil }
	}
	m := New(dir)
	m.cmdInput.SetValue(`imgview 'a; b.png'`)
	if ok, cmd := m.tryImageViewer(`imgview 'a; b.png'`); !ok || cmd == nil {
		t.Fatal("no launch")
	}
	if got == nil || got.Dir != dir || !reflect.DeepEqual(got.Args, []string{exe, "imgview", "a; b.png"}) {
		t.Fatalf("bad command: %#v", got)
	}
	if !containsImageViewerEnv(got.Env, "MISA_IMGVIEW_EMBEDDED=1") {
		t.Fatal("embedded child marker missing")
	}
	if m.cmdInput.Value() != "" || m.procViewer != nil {
		t.Fatal("prompt or viewer state changed incorrectly")
	}
	got.Process = &os.Process{Pid: 42} // the child started; a nonzero exit must not reopen it
	exit := callback(errors.New("exit status 2")).(imageViewerFinished)
	if exit.startFailed {
		t.Fatal("a child exit was classified as a start failure")
	}
	other := New(dir)
	_, _ = other.updateImageViewerFinished(exit)
	if other.procViewer != nil || !bufferHas(other, "exit status 2") {
		t.Fatal("child exit was not reported")
	}
	got.Process = nil
	msg := callback(&os.PathError{Op: "fork/exec", Path: exe, Err: os.ErrPermission}).(imageViewerFinished)
	_, _ = m.updateImageViewerFinished(msg)
	if m.procViewer == nil {
		t.Fatal("start failure did not fall back")
	}
	v := m.procViewer.(*imgview.Viewer)
	v.Resize(160, 8)
	if !strings.Contains(v.View(), "in-UI") {
		t.Fatal("fallback notice missing")
	}
}

func TestImageViewerNonExecutableAndExit(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	exe := filepath.Join(dir, "misa")
	if err := os.WriteFile(exe, nil, 0644); err != nil {
		t.Fatal(err)
	}
	oldExe, oldExec := imageViewerExecutable, imageViewerExecProcess
	t.Cleanup(func() { imageViewerExecutable, imageViewerExecProcess = oldExe, oldExec })
	imageViewerExecutable = func() (string, error) { return exe, nil }
	imageViewerExecProcess = func(*exec.Cmd, tea.ExecCallback) tea.Cmd { t.Fatal("executed non-executable"); return nil }
	m := New(dir)
	m.width, m.height = 160, 8
	m.tryImageViewer("imgview a.png")
	if m.procViewer == nil || !strings.Contains(m.procViewer.View(), "in-UI") {
		t.Fatal("non-executable fallback missing notice")
	}
	other := New(dir)
	_, _ = other.updateImageViewerFinished(imageViewerFinished{err: errors.New("exit status 1")})
	if other.procViewer != nil || !bufferHas(other, "exit status 1") {
		t.Fatal("exit failure not reported correctly")
	}
}

func TestImageViewerFinishedRoutesToOwner(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := newRoot(t, 80, 24)
	m.addMuxPane()
	owner := m.mux.panes[0].misa
	if owner == nil {
		t.Fatal("missing nested pane")
	}
	m.Update(imageViewerFinished{owner: owner, err: errors.New("failed to restore terminal")})
	if !bufferHas(owner, "failed to restore terminal") || bufferHas(m, "failed to restore terminal") {
		t.Fatal("result not delivered to owner")
	}
}

func containsImageViewerEnv(env []string, want string) bool {
	for _, entry := range env {
		if entry == want {
			return true
		}
	}
	return false
}
