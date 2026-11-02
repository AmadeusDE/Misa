package imgview

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"image"
	"strings"
	"time"

	"github.com/AmadeusDE/Misa/internal/imageout"
	"github.com/AmadeusDE/Misa/internal/kittygfx"
	"github.com/charmbracelet/x/ansi"
)

type standaloneKey struct {
	pixels                         *image.NRGBA
	index, width, height, rotation int
	flipX, flipY                   bool
	pan, cell                      image.Point
	scale                          float64
	version                        uint64
}

type standaloneOutput struct {
	v             *Viewer
	kitty         bool
	limits        kittygfx.Limits
	id            uint32
	active        bool
	key           standaloneKey
	digest        [32]byte
	offset        image.Point
	size          image.Point
	display       image.Point
	rows          []string
	width, height int
	mode          bool
	started       bool
}

func newStandaloneOutput(v *Viewer, kitty bool) *standaloneOutput {
	var id uint32
	for id == 0 {
		var b [4]byte
		if _, err := rand.Read(b[:]); err != nil {
			panic("imgview: cannot allocate random image ID: " + err.Error())
		}
		id = binary.LittleEndian.Uint32(b[:])
	}
	if kitty {
		v.policy.FPS = v.config.AnimationFPS
	}
	return &standaloneOutput{v: v, kitty: kitty, id: id, limits: kittygfx.DefaultLimits()}
}

func (o *standaloneOutput) Frame() ([]byte, error) {
	v := o.v
	var key standaloneKey
	var raster *image.NRGBA
	var offset, display image.Point
	var after []byte
	var digest [32]byte
	// A local APC cap can reject even the probe, so auto mode cannot
	// distinguish it from an unsupported terminal by response alone. Check
	// whether a minimal transfer could ever fit the locally configured limits.
	warning := false
	if !o.kitty && v.Renderer() == "auto" && !v.grid && !v.help && len(v.items) > 0 && v.pixels(v.index) != nil {
		_, err := imageout.KittyTransmitLimited(o.id, image.NewNRGBA(image.Rect(0, 0, 1, 1)), image.Point{}, v.cellSize(), o.limits)
		warning = err != nil
	}
	useKitty := false
	if o.kitty && !v.grid && !v.help && len(v.items) > 0 && v.items[v.index].err == nil {
		it := v.items[v.index]
		key = standaloneKey{pixels: v.pixels(v.index), index: v.index, width: v.width, height: v.height, rotation: it.rotation, flipX: it.flipX, flipY: it.flipY, pan: v.pan, cell: v.cellSize(), scale: v.scale(), version: it.version}
		if key.pixels != nil && o.active && key == o.key {
			useKitty = true
		} else {
			raster, offset, display, useKitty = standaloneRasterLimited(v, o.limits)
			warning = key.pixels != nil && !useKitty
		}
	}
	// Build transmission before committing mode, so a limit too small even for
	// one chunk falls back to visible halfblocks rather than a blank image.
	if useKitty && raster != nil {
		digest = sha256.Sum256(raster.Pix)
		clear := !o.started || o.width != v.width || o.height != v.height || o.mode != useKitty
		if clear || !o.active || key.index != o.key.index || digest != o.digest || offset != o.offset || raster.Bounds().Size() != o.size || display != o.display || key.cell != o.key.cell {
			var err error
			if display != raster.Bounds().Size() {
				after, err = imageout.KittyTransmitScaled(o.id, raster, offset, key.cell, display, o.limits)
			} else {
				after, err = imageout.KittyTransmitLimited(o.id, raster, offset, key.cell, o.limits)
			}
			if err != nil {
				useKitty = false
				warning = true
				after = nil
			}
		}
	}
	// Budget the base64 transport bytes of a visible animation frame. The
	// source is cropped to this rectangle, so offscreen pixels cost nothing.
	var interval time.Duration
	if useKitty {
		size := o.size
		if raster != nil {
			size = raster.Bounds().Size()
		}
		raw := int64(size.X) * int64(size.Y) * 4
		encoded := ((raw + 2) / 3) * 4
		rate := int64(v.config.AnimationTransportMiBS) << 20
		if rate > 0 {
			interval = time.Duration((encoded*int64(time.Second) + rate - 1) / rate)
		}
	}
	if interval != v.policy.MinInterval {
		v.policy.MinInterval = interval
		if v.anim != nil && v.anim.ctrl != nil {
			if p, ok := v.anim.ctrl.(interface{ SetMinInterval(time.Duration) }); ok {
				p.SetMinInterval(interval)
				// Wake an armed wait so a shorter or longer budget takes effect now.
				if v.anim.retime != nil {
					close(v.anim.retime)
					v.anim.retime = nil
				}
			}
		}
	}
	clear := !o.started || o.width != v.width || o.height != v.height || o.mode != useKitty
	var before []byte
	if o.active && (!useKitty || key.index != o.key.index) {
		before = imageout.KittyDelete(o.id)
		o.active = false
	}
	if useKitty && raster != nil {
		o.digest, o.offset, o.size, o.display = digest, offset, raster.Bounds().Size(), display
		o.active = true
	}

	o.key = key
	text := v.view(useKitty, true)
	lines := strings.Split(text, "\n")
	if warning && len(lines) > 0 {
		i := len(lines) - 1
		lines[i] = ansi.Truncate("kitty limits; using halfblock · "+lines[i], v.width, "")
	}
	rows := make(map[int]string)
	if clear {
		for i, line := range lines {
			if i < v.height {
				rows[i] = line
			}
		}
	} else {
		for i, line := range lines {
			if i < v.height && (i >= len(o.rows) || o.rows[i] != line) {
				rows[i] = line
			}
		}
		for i := len(lines); i < len(o.rows) && i < v.height; i++ {
			rows[i] = ""
		}
	}
	o.rows = append(o.rows[:0], lines...)
	o.width, o.height, o.mode, o.started = v.width, v.height, useKitty, true
	return imageout.TerminalFrame(rows, clear, before, after), nil
}

func (o *standaloneOutput) Close() []byte {
	if !o.active {
		return nil
	}
	o.active = false
	return imageout.KittyDelete(o.id)
}
