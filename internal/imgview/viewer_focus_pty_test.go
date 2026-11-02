//go:build unix && !android

package imgview

import (
	"io"
	"os"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/config"
	"github.com/AmadeusDE/Misa/internal/term"
)

// focusSnapshot is the viewer state observed on the program's own goroutine.
type focusSnapshot struct {
	visible, armed, playing bool
	focused                 bool
	frame, committed        int
	plays, loop             uint64
	pending                 int
}

// focusProgram is the standalone viewer under a real Bubble Tea program whose
// input is a PTY, so host focus reports take the production path.
type focusProgram struct {
	inner   tea.Model
	viewer  *Viewer
	clock   *ManualClock
	started chan struct{}
	blurred chan struct{}
	blur    focusSnapshot
	resume  focusSnapshot
}

func (p *focusProgram) Init() tea.Cmd { return p.inner.Init() }

func (p *focusProgram) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := p.inner.Update(msg)
	if a := p.viewer.anim; a != nil && a.armed {
		select {
		case <-p.started:
		default:
			close(p.started)
		}
	}
	switch msg.(type) {
	case tea.BlurMsg:
		p.blur = p.snapshot()
		close(p.blurred)
	case tea.FocusMsg:
		p.resume = p.snapshot()
		return p, tea.Quit
	}
	return p, cmd
}

func (p *focusProgram) View() tea.View { return p.inner.View() }

func (p *focusProgram) snapshot() focusSnapshot {
	s := focusSnapshot{focused: p.viewer.focused, pending: p.clock.Pending()}
	if a := p.viewer.anim; a != nil {
		s.visible, s.armed, s.committed = a.visible, a.armed, a.index
		if a.ctrl != nil {
			s.playing, s.frame = a.ctrl.Playing(), a.ctrl.Frame()
			s.plays, s.loop = a.ctrl.Plays(), a.ctrl.Loop()
		}
	}
	return s
}

// TestStandaloneFocusReportsPauseAndResume drives the standalone viewer from the
// bytes a host terminal sends: DECSET 1004 enables reporting, CSI O and CSI I
// reach the viewer as BlurMsg/FocusMsg through Bubble Tea's input decoder.
func TestStandaloneFocusReportsPauseAndResume(t *testing.T) {
	cfg := config.DefaultImageViewer()
	cfg.AnimationAutoplay = true
	cfg.HalfblockAnimationFPS = 120
	v, err := NewConfigured([]string{writeAnimatedGIF(t, 3, 0)}, false, "", cfg)
	if err != nil {
		t.Fatal(err)
	}
	v.Resize(40, 12)
	clock := NewManualClock()
	v.clock = clock

	if view := (&standalone{viewer: v}).View(); !view.ReportFocus {
		t.Fatal("standalone View does not request focus reporting")
	}

	p := &focusProgram{inner: &standalone{viewer: v}, viewer: v, clock: clock,
		started: make(chan struct{}), blurred: make(chan struct{})}
	tm := term.New(40, 12)
	done := make(chan error, 1)
	progs := make(chan *tea.Program, 1)
	if err := tm.StartFunc(func(tty *os.File) int {
		// The program's output is the host: everything it writes, including the
		// DECSET 1004 that View.ReportFocus asks the renderer for, reaches the
		// terminal emulator, whose focus reports come back on the PTY.
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
		prog := tea.NewProgram(p,
			tea.WithInput(tty), tea.WithOutput(in),
			tea.WithoutSignalHandler(), tea.WithWindowSize(40, 12))
		progs <- prog
		_, err := prog.Run()
		in.Close() // end the host reader once the program stops writing
		done <- err
		return 0
	}); err != nil {
		t.Fatalf("PTY session: %v", err)
	}
	defer tm.Close()

	waitFor(t, p.started, "playback never started")
	prog := <-progs
	defer func() {
		select {
		case <-done:
		default:
			prog.Kill()
		}
	}()

	tm.SetFocused(false)
	waitFor(t, p.blurred, "focus-out never reached the viewer")
	tm.SetFocused(true)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("program: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the program did not quit after focus-in")
	}

	if b := p.blur; b.visible || b.armed || b.playing || b.focused {
		t.Fatalf("blurred viewer kept working: %+v", b)
	}
	if p.blur.frame != 0 || p.blur.committed != 0 || p.blur.plays != 0 || p.blur.loop != 1 {
		t.Fatalf("blur moved the sequence: %+v", p.blur)
	}
	if r := p.resume; !r.visible || !r.armed || !r.playing || !r.focused || r.pending != 1 {
		t.Fatalf("focused viewer did not resume: %+v", r)
	}
	if p.resume.frame != 0 || p.resume.plays != 0 || p.resume.loop != 1 {
		t.Fatalf("resume restarted the sequence: %+v", p.resume)
	}
}

func waitFor(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal(what)
	}
}
