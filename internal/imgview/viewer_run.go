package imgview

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/AmadeusDE/Misa/internal/config"
	"github.com/AmadeusDE/Misa/internal/imageout"
	"github.com/muesli/cancelreader"
	"golang.org/x/term"
)

const standaloneEnter = "\x1b[?1049h\x1b[?25l\x1b[?1002h\x1b[?1006h\x1b[?1004h"
const standaloneLeave = "\x1b[?2026l\x1b[0m\x1b[?1004l\x1b[?1006l\x1b[?1002l\x1b[?25h\x1b[?1049l"

type standaloneRead struct {
	data []byte
	err  error
}

// Run displays the viewer directly on its terminal, without a Bubble Tea program.
func Run(v *Viewer) error { return runStandalone(v, os.Stdin, os.Stdout) }

func runStandalone(v *Viewer, in, out *os.File) (err error) {
	sig := make(chan os.Signal, 4)
	standaloneSignals(sig)
	defer signal.Stop(sig)
	state, e := term.MakeRaw(int(in.Fd()))
	if e != nil {
		return fmt.Errorf("imgview: raw terminal: %w", e)
	}
	active := false
	var output *standaloneOutput
	enter := func() error {
		// Even a partial write may have enabled terminal modes.
		active = true
		if _, e := io.WriteString(out, standaloneEnter); e != nil {
			return e
		}
		return nil
	}
	leave := func() {
		if active {
			if output != nil {
				_, _ = out.Write(output.Close())
			}
			_, _ = io.WriteString(out, standaloneLeave)
			active = false
		}
		_ = term.Restore(int(in.Fd()), state)
	}
	defer leave()
	defer func() { v.cancelLoad(); v.stopAnimation() }()
	if e = enter(); e != nil {
		return e
	}
	ctx, cancel := context.WithCancel(context.Background())
	reader, e := cancelreader.NewReader(in)
	if e != nil {
		cancel()
		return e
	}
	defer reader.Close()
	reads := make(chan standaloneRead, 8)
	readerDone := make(chan struct{})
	defer func() {
		cancel()
		if reader.Cancel() {
			<-readerDone
		}
	}()
	go func() {
		defer close(readerDone)
		buf := make([]byte, 4096)
		for {
			n, e := reader.Read(buf)
			if n > 0 {
				p := append([]byte(nil), buf[:n]...)
				select {
				case reads <- standaloneRead{data: p}:
				case <-ctx.Done():
					return
				}
			}
			if e != nil {
				select {
				case reads <- standaloneRead{err: e}:
				case <-ctx.Done():
				}
				return
			}
		}
	}()
	decoder := standaloneDecoder{}
	pending := []tea.Msg{}
	kitty := v.Renderer() == "kitty"
	if v.Renderer() == "auto" {
		decoder.probing = true
		var id [4]byte
		if _, e = rand.Read(id[:]); e != nil {
			return fmt.Errorf("imgview: probe id: %w", e)
		}
		decoder.probeID = binary.LittleEndian.Uint32(id[:])
		if decoder.probeID == 0 {
			decoder.probeID = 1
		}
		query, e := imageout.KittyQuery(decoder.probeID)
		if e != nil {
			return e
		}
		if _, e = out.Write(query); e != nil {
			return e
		}
		timer := time.NewTimer(300 * time.Millisecond)
	probe:
		for {
			select {
			case r := <-reads:
				if r.err != nil {
					timer.Stop()
					return r.err
				}
				pending = append(pending, decoder.feed(r.data)...)
				if decoder.probeDone || len(pending) >= 4096 {
					break probe
				}
			case s := <-sig:
				if !standaloneResizeSignal(s) {
					timer.Stop()
					return fmt.Errorf("imgview: interrupted by %v", s)
				}
			case <-timer.C:
				break probe
			}
		}
		timer.Stop()
		kitty = decoder.kitty
		decoder.probing = false
	}
	graphicsLimits := config.LoadGraphics().Kitty.Limits()
	output = newStandaloneOutput(v, kitty)
	output.limits = graphicsLimits
	resize := func() {
		w, h, e := term.GetSize(int(out.Fd()))
		if e == nil {
			v.Resize(w, h)
		}
		x, y := standalonePixels(out)
		v.SetCellSize(CellPixelsFor(v.config, image.Pt(x, y)))
	}
	resize()
	msgs := make(chan tea.Msg, 64)
	standaloneCommands(ctx, msgs, v.Init())
	draw := func() error {
		b, e := output.Frame()
		if e != nil {
			return e
		}
		_, e = out.Write(b)
		return e
	}
	if e = draw(); e != nil {
		return e
	}
	escapeTimer := time.NewTimer(time.Hour)
	if !escapeTimer.Stop() {
		<-escapeTimer.C
	}
	defer escapeTimer.Stop()
	var escapeC <-chan time.Time
	armEscape := func() {
		if len(decoder.buf) == 1 && decoder.buf[0] == 27 {
			escapeTimer.Reset(30 * time.Millisecond)
			escapeC = escapeTimer.C
		} else {
			escapeTimer.Stop()
			escapeC = nil
		}
	}
	armEscape()
	for {
		var msg tea.Msg
		if len(pending) > 0 {
			msg = pending[0]
			pending = pending[1:]
		} else {
			select {
			case r := <-reads:
				if r.err != nil {
					return r.err
				}
				pending = append(pending, decoder.feed(r.data)...)
				armEscape()
				continue
			case msg = <-msgs:
			case s := <-sig:
				if standaloneResizeSignal(s) {
					resize()
					if e = draw(); e != nil {
						return e
					}
					continue
				}
				return fmt.Errorf("imgview: interrupted by %v", s)
			case <-escapeC:
				escapeC = nil
				pending = append(pending, decoder.flushEscape()...)
				continue
			}
		}
		if p, ok := msg.(standalonePanic); ok {
			return fmt.Errorf("imgview: command panic: %v", p.value)
		}
		if _, ok := msg.(tea.QuitMsg); ok {
			return nil
		}
		if k, ok := msg.(tea.KeyPressMsg); ok && k.Mod&tea.ModCtrl != 0 {
			if k.Code == 'c' {
				return nil
			}
			if k.Code == 'z' {
				if os.Getenv("MISA_IMGVIEW_EMBEDDED") == "1" {
					const warning = "suspend unavailable when launched from Misa; q returns"
					if v.notice != warning && !strings.HasPrefix(v.notice, warning+" · ") {
						if v.notice != "" {
							v.notice = warning + " · " + v.notice
						} else {
							v.notice = warning
						}
					}
					if e = draw(); e != nil {
						return e
					}
					continue
				}
				leave()
				standaloneSuspend()
				state, e = term.MakeRaw(int(in.Fd()))
				if e != nil {
					return e
				}
				if e = enter(); e != nil {
					return e
				}
				output = newStandaloneOutput(v, kitty)
				output.limits = graphicsLimits
				resize()
				if e = draw(); e != nil {
					return e
				}
				continue
			}
		}
		standaloneCommands(ctx, msgs, v.Update(msg))
		if v.IsQuit() {
			return nil
		}
		if e = draw(); e != nil {
			return e
		}
	}
}
