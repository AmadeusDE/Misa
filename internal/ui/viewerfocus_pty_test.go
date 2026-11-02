//go:build unix && !android

package ui

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/AmadeusDE/Misa/internal/imgview"
	"github.com/AmadeusDE/Misa/internal/term"
)

// focusState is the root's focus and the visibility it pushed, read on the
// program's own goroutine.
type focusState struct{ windowFocused, visible bool }

// focusRoot runs the main model under a real Bubble Tea program and records what
// each host focus report did to it.
type focusRoot struct {
	m       *Model
	ready   chan struct{}
	blurred chan struct{}
	blur    focusState
	resume  focusState
}

func (f *focusRoot) Init() tea.Cmd { return f.m.Init() }

func (f *focusRoot) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := f.m.Update(msg)
	if _, ok := msg.(tea.WindowSizeMsg); ok {
		select {
		case <-f.ready:
		default:
			close(f.ready)
		}
	}
	switch msg.(type) {
	case tea.BlurMsg:
		f.blur = focusState{f.m.windowFocused, f.m.procVisible}
		close(f.blurred)
	case tea.FocusMsg:
		f.resume = focusState{f.m.windowFocused, f.m.procVisible}
		return f, tea.Quit
	}
	return f, cmd
}

func (f *focusRoot) View() tea.View { return f.m.View() }

// TestMainUIFocusReportsPauseTheImageViewer follows a host focus report from the
// terminal bytes Bubble Tea decodes to the visibility the root pushes into the
// in-process image viewer.
func TestMainUIFocusReportsPauseTheImageViewer(t *testing.T) {
	forceImageViewerFallback(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := newRoot(t, 80, 24)
	m.muxAutoPane = false // a pane would replace the root's own screen
	if ok, _ := m.handleBuiltinCmd("imgview " + writeFocusPNG(t)); !ok {
		t.Fatal("did not enter the image viewer")
	}
	if _, ok := m.procViewer.(*imgview.Viewer); !ok {
		t.Fatalf("viewer %T is not the real image viewer", m.procViewer)
	}
	if !m.procVisible {
		t.Fatal("a visible root viewer started hidden")
	}

	f := &focusRoot{m: m, ready: make(chan struct{}), blurred: make(chan struct{})}
	tm := term.New(80, 24)
	done := make(chan error, 1)
	progs := make(chan *tea.Program, 1)
	if err := tm.StartFunc(func(tty *os.File) int {
		out, in := io.Pipe()
		go func() {
			defer out.Close()
			buf := make([]byte, 4096)
			for {
				n, err := out.Read(buf)
				if n > 0 {
					tm.ProcessOutput(buf[:n])
				}
				if err != nil {
					return
				}
			}
		}()
		prog := tea.NewProgram(f,
			tea.WithInput(tty), tea.WithOutput(in),
			tea.WithoutSignalHandler(), tea.WithWindowSize(80, 24))
		progs <- prog
		_, err := prog.Run()
		in.Close() // end the host reader once the program stops writing
		done <- err
		return 0
	}); err != nil {
		t.Fatalf("PTY session: %v", err)
	}
	defer tm.Close()

	waitSignal(t, f.ready, "the program never resized the root")
	prog := <-progs
	defer func() {
		select {
		case <-done:
		default:
			prog.Kill()
		}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for !tm.FocusReporting() {
		if time.Now().After(deadline) {
			t.Fatal("the root view never asked the terminal for focus reports")
		}
		time.Sleep(time.Millisecond)
	}

	tm.SetFocused(false)
	waitSignal(t, f.blurred, "focus-out never reached the root")
	tm.SetFocused(true)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("program: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the program did not quit after focus-in")
	}

	if f.blur != (focusState{windowFocused: false, visible: false}) {
		t.Fatalf("blurred root = %+v, want an unfocused window and a paused viewer", f.blur)
	}
	if f.resume != (focusState{windowFocused: true, visible: true}) {
		t.Fatalf("focused root = %+v, want a focused window and a resumed viewer", f.resume)
	}
}

func waitSignal(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal(what)
	}
}

func writeFocusPNG(t *testing.T) string {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, 4, 3))); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "focus.png")
	if err := os.WriteFile(p, b.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}
