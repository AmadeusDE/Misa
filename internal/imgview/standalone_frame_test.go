package imgview

import (
	"bytes"
	"image"
	"image/color"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/AmadeusDE/Misa/internal/kittygfx"
)

func TestStandaloneFrameLifecycle(t *testing.T) {
	v, _ := New([]string{"a", "b"}, false, "kitty")
	v.Resize(100, 10)
	p := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	p.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 100})
	v.SetImage(0, p, nil)
	v.SetImage(1, p, nil)
	if v.Renderer() != "kitty" {
		t.Fatal(v.Renderer())
	}
	o := newStandaloneOutput(v, true)
	first, err := o.Frame()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(first, []byte("\x1b_G")) || !strings.Contains(string(first), "kitty") || strings.Contains(string(first), "kitty unavailable") {
		t.Fatalf("first frame %q", first)
	}
	v.Update(key("m"))
	next, err := o.Frame()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(next, []byte("\x1b_G")) {
		t.Fatal("status retransmitted image")
	}
	v.Update(key(">"))
	changed, err := o.Frame()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(changed, []byte("\x1b_G")) {
		t.Fatal("transform did not transmit")
	}
	v.Update(key("n"))
	next, err = o.Frame()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(next, []byte("a=d")) {
		t.Fatal("selection did not delete")
	}
	v.Update(key("f1"))
	next, err = o.Frame()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(next, []byte("a=d")) {
		t.Fatal("help did not delete")
	}
	if len(o.Close()) != 0 {
		t.Fatal("already deleted")
	}
}

func TestStandaloneHalfblockWarning(t *testing.T) {
	v, _ := New([]string{"a"}, false, "kitty")
	v.Resize(100, 5)
	v.SetImage(0, image.NewNRGBA(image.Rect(0, 0, 1, 1)), nil)
	if !strings.Contains(v.View(), "kitty unavailable") {
		t.Fatal("embedded warning missing")
	}
	o := newStandaloneOutput(v, false)
	b, err := o.Frame()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "halfblock") {
		t.Fatal("halfblock status missing")
	}
}

func TestStandaloneRasterTransformCropAlpha(t *testing.T) {
	v, _ := New([]string{"a"}, false, "kitty")
	v.Resize(1, 2)
	v.SetCellSize(image.Pt(2, 2), true)
	p := image.NewNRGBA(image.Rect(0, 0, 4, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			p.SetNRGBA(x, y, color.NRGBA{R: uint8(20 + x*40), G: uint8(y * 80), A: 128})
		}
	}
	v.SetImage(0, p, nil)
	v.fit = fitActual
	v.pan = image.Pt(1, 0)
	got, offset, ok := standaloneRaster(v)
	if !ok || offset != (image.Point{}) || got.Bounds().Size() != image.Pt(2, 2) {
		t.Fatalf("raster %v %v %v", got, offset, ok)
	}
	if c := got.NRGBAAt(0, 0); c.A != 128 || c.R != 100 {
		t.Fatalf("cropped pixel %v", c)
	}
	v.items[0].rotation = 1
	v.pan = image.Point{}
	got, _, ok = standaloneRaster(v)
	if !ok {
		t.Fatal("rotation")
	}
	if c := got.NRGBAAt(0, 0); c.A != 128 || c.G != 80 {
		t.Fatalf("rotated pixel %v", c)
	}
}

func TestStandaloneNoOpAndResize(t *testing.T) {
	v, _ := New([]string{"a"}, false, "kitty")
	v.Resize(50, 10)
	v.SetImage(0, image.NewNRGBA(image.Rect(0, 0, 4, 4)), nil)
	o := newStandaloneOutput(v, true)
	if _, err := o.Frame(); err != nil {
		t.Fatal(err)
	}
	b, err := o.Frame()
	if err != nil || len(b) != 0 {
		t.Fatalf("no-op %q %v", b, err)
	}
	v.Resize(51, 10)
	b, err = o.Frame()
	if err != nil || !bytes.Contains(b, []byte("a=T")) {
		t.Fatalf("resize missing retransmit: %q %v", b, err)
	}
}

func TestStandaloneContainColors(t *testing.T) {
	v, _ := New([]string{"a"}, false, "kitty")
	v.Resize(40, 12)
	v.fit = fitWhole
	p := image.NewNRGBA(image.Rect(0, 0, 64, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 64; x++ {
			c := color.NRGBA{R: 255, A: 255}
			if x >= 32 {
				c = color.NRGBA{B: 255, A: 255}
			}
			p.SetNRGBA(x, y, c)
		}
	}
	v.SetImage(0, p, nil)
	img, _, ok := standaloneRaster(v)
	if !ok {
		t.Fatal("no raster")
	}
	t.Logf("bounds %v left %v right %v", img.Bounds(), img.NRGBAAt(100, 100), img.NRGBAAt(300, 100))
	if img.NRGBAAt(100, 100) != (color.NRGBA{R: 255, A: 255}) || img.NRGBAAt(300, 100) != (color.NRGBA{B: 255, A: 255}) {
		t.Fatal("wrong colors")
	}
}

func TestStandaloneScaledAlpha(t *testing.T) {
	v, _ := New([]string{"a"}, false, "kitty")
	v.Resize(4, 3)
	v.SetCellSize(image.Pt(2, 2), true)
	v.fit = fitWhole
	p := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			p.SetNRGBA(x, y, color.NRGBA{R: 200, G: 100, A: 128})
		}
	}
	v.SetImage(0, p, nil)
	got, _, ok := standaloneRaster(v)
	if !ok {
		t.Fatal("no raster")
	}
	c := got.NRGBAAt(1, 1)
	if c.A != 128 || c.R < 198 || c.G < 98 {
		t.Fatalf("scaled alpha %v", c)
	}
}

func TestStandaloneRandomIDs(t *testing.T) {
	v, _ := New(nil, false, "auto")
	seen := map[uint32]bool{}
	for i := 0; i < 100; i++ {
		id := newStandaloneOutput(v, true).id
		if id == 0 || seen[id] {
			t.Fatalf("invalid or duplicate image ID %d", id)
		}
		seen[id] = true
	}
}

func TestStandaloneTransportBudgetTracksGeometry(t *testing.T) {
	v, _ := New([]string{"a"}, false, "kitty")
	v.config.AnimationTransportMiBS = 1
	v.Resize(100, 51)
	v.SetCellSize(image.Pt(10, 20), true)
	v.SetImage(0, image.NewNRGBA(image.Rect(0, 0, 1000, 1000)), nil)
	o := newStandaloneOutput(v, true)
	if _, err := o.Frame(); err != nil {
		t.Fatal(err)
	}
	full := v.policy.MinInterval
	if full < 5*time.Second {
		t.Fatalf("encoded 1000x1000 at 1MiB/s: %v", full)
	}
	v.Resize(50, 26)
	if _, err := o.Frame(); err != nil {
		t.Fatal(err)
	}
	small := v.policy.MinInterval
	if small >= full || small < time.Second {
		t.Fatalf("resize %v -> %v", full, small)
	}
	v.fit = fitManual
	v.zoom = 0.25
	if _, err := o.Frame(); err != nil {
		t.Fatal(err)
	}
	if zoom := v.policy.MinInterval; zoom >= small || zoom == 0 {
		t.Fatalf("zoom %v -> %v", small, zoom)
	}
}

func TestStandaloneTransportExtendsArmedTick(t *testing.T) {
	v, _ := New([]string{"a"}, false, "kitty")
	v.config.AnimationTransportMiBS = 1
	v.Resize(100, 51)
	v.SetCellSize(image.Pt(10, 20), true)
	v.SetImage(0, image.NewNRGBA(image.Rect(0, 0, 1000, 1000)), nil)
	p, clock := newTestPlayback(t, 100, 0, 20*time.Millisecond, PlaybackPolicy{Autoplay: true, FPS: 30})
	a := &animation{ctrl: p, visible: true, armed: true}
	v.anim = a
	initial := p.NextDelay()
	o := newStandaloneOutput(v, true)
	if _, err := o.Frame(); err != nil {
		t.Fatal(err)
	}
	clock.Advance(initial)
	v.tick(viewerTick{anim: a})
	if p.Frame() != 0 {
		t.Fatalf("early tick advanced to %d", p.Frame())
	}
	if d := p.NextDelay(); d < time.Second {
		t.Fatalf("budget was not applied: %v", d)
	}
}

func TestStandaloneBudgetChangeWakesTimer(t *testing.T) {
	v, _ := New([]string{"a"}, false, "kitty")
	v.config.AnimationTransportMiBS = 1
	v.Resize(100, 51)
	v.SetCellSize(image.Pt(10, 20), true)
	v.SetImage(0, image.NewNRGBA(image.Rect(0, 0, 1000, 1000)), nil)
	p, clock := newTestPlayback(t, 100, 0, 20*time.Millisecond, PlaybackPolicy{Autoplay: true, FPS: 30})
	v.clock = clock
	a := &animation{ctrl: p, visible: true, armed: true, stop: make(chan struct{})}
	v.anim = a
	cmd := v.waitCmd(a)
	o := newStandaloneOutput(v, true)
	if _, err := o.Frame(); err != nil {
		t.Fatal(err)
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		if _, ok := msg.(viewerTick); !ok {
			t.Fatalf("retime returned %T", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("budget change did not wake timer")
	}
}

func TestStandaloneLimitedRasterPreservesPlacement(t *testing.T) {
	v, _ := New([]string{"a"}, false, "kitty")
	v.Resize(40, 12)
	v.fit = fitWhole
	p := image.NewNRGBA(image.Rect(0, 0, 64, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 64; x++ {
			c := color.NRGBA{R: 255, A: 255}
			if x >= 32 {
				c = color.NRGBA{B: 255, A: 255}
			}
			p.SetNRGBA(x, y, c)
		}
	}
	v.SetImage(0, p, nil)
	l := kittygfx.DefaultLimits()
	l.MaxWidth = 50
	l.MaxHeight = 50
	l.MaxImageBytes = 10000
	l.MaxTerminalBytes = 10000
	l.MaxInFlightBytes = 14000
	raster, offset, display, ok := standaloneRasterLimited(v, l)
	if !ok || display != image.Pt(400, 200) || offset != image.Pt(0, 10) {
		t.Fatalf("geometry %v %v %v", offset, display, ok)
	}
	if raster.Bounds().Dx() > 50 || raster.Bounds().Dy() > 50 {
		t.Fatal(raster.Bounds())
	}
	if c := raster.NRGBAAt(10, 10); c.R < 240 {
		t.Fatalf("left %v", c)
	}
}

func TestStandaloneLimitFallbackNotice(t *testing.T) {
	v, _ := New([]string{"a"}, false, "kitty")
	v.Resize(100, 8)
	v.SetImage(0, image.NewNRGBA(image.Rect(0, 0, 8, 8)), nil)
	o := newStandaloneOutput(v, true)
	o.limits.MaxPayload = 8
	b, err := o.Frame()
	if err != nil || !strings.Contains(string(b), "kitty limits; using halfblock") || bytes.Contains(b, []byte("a=T")) {
		t.Fatalf("fallback %q %v", b, err)
	}
}
