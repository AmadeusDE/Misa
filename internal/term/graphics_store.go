package term

import (
	"image"

	"github.com/AmadeusDE/Misa/internal/kittygfx"
)

// graphicsLimits bounds what a PTY client may make the terminal store. It is
// the same set of caps kittygfx.Limits carries, under the names the store uses;
// the defaults are derived from kittygfx.DefaultLimits so the protocol package
// stays the one place the numbers live.
//
// The byte caps bound what is stored. Committing a transfer also holds its
// encoded payload and one decoded image at the same time, so the peak is the
// stored total plus one image and its encoded transfer, not the total alone.
type graphicsLimits struct {
	imageWidth, imageHeight int
	imageBytes, totalBytes  int64
	images, placements      int
	inFlightBytes           int64
	payloadBytes            int
}

var defaultGraphicsLimits = graphicsLimitsFrom(kittygfx.DefaultLimits())

// graphicsLimitsFrom converts protocol limits into the store's fields. Zero
// fields take kittygfx's defaults, so a caller can pass a partial Limits.
func graphicsLimitsFrom(l kittygfx.Limits) graphicsLimits {
	l = l.Normalize()
	return graphicsLimits{
		imageWidth:    l.MaxWidth,
		imageHeight:   l.MaxHeight,
		imageBytes:    l.MaxImageBytes,
		totalBytes:    l.MaxTerminalBytes,
		images:        l.MaxImages,
		placements:    l.MaxPlacements,
		inFlightBytes: l.MaxInFlightBytes,
		payloadBytes:  l.MaxPayload,
	}
}

// screen selects the grid a placement is anchored to: the main screen or the
// alternate screen, which are shown one at a time. Image IDs and numbers are
// per screen, so a client on the alternate screen cannot delete the main
// screen's images.
type screen int

const (
	screenMain screen = iota
	screenAlt
	screenCount
)

type gfxKey struct {
	screen screen
	id     uint32
}

// graphicsStore is the terminal's image and placement state. Rasters are
// immutable once published: a replacement installs a new image.Image and
// snapshots keep the old pointer, so nothing copies pixels per frame.
type graphicsStore struct {
	limits   graphicsLimits
	enabled  bool
	closed   bool   // set by Close: the terminal never accepts graphics again
	epoch    uint64 // invalidates work in flight when the store is reset
	gen      uint64 // bumps on every change a frontend must redraw
	seq      uint64 // insertion order, for oldest-first eviction
	imgGen   uint64
	images   map[gfxKey]*gfxImage
	numbers  map[gfxKey]uint32 // (screen, image number) -> image ID
	nextID   [screenCount]uint32
	places   []*gfxPlacement
	rows     int // grid the placements were measured on
	cols     int
	total    int64
	transfer *gfxTransfer
}

type gfxImage struct {
	key    gfxKey
	gen    uint64
	raster image.Image
	bytes  int64
	seq    uint64
}

type gfxPlacement struct {
	imageID uint32
	pid     uint32 // protocol placement ID; 0 keeps every placement separate
	screen  screen
	cell    image.Rectangle // terminal cells, screen coordinates
	clip    image.Rectangle // extra clip, the scroll region it moved in; zero = none
	source  image.Rectangle // pixels inside the raster
	offset  image.Point     // pixel offset inside the first cell
	seq     uint64
}

func (g *graphicsStore) setEnabled(on bool) {
	if g.enabled == on {
		return
	}
	g.enabled = on
	g.clear()
}

// setLimits applies new caps: transfers in flight are discarded, images that
// no longer fit are evicted, and the epoch change drops work being decoded.
func (g *graphicsStore) setLimits(l graphicsLimits) {
	g.limits = l
	g.epoch++
	g.transfer = nil
	for key, img := range g.images {
		bounds := img.raster.Bounds()
		if img.bytes > l.imageBytes || bounds.Dx() > l.imageWidth || bounds.Dy() > l.imageHeight {
			g.dropImage(key.screen, key.id)
		}
	}
	g.enforce()
}

// clear drops every image, placement and partial transfer. The epoch change
// makes a decode that is running outside the terminal lock discard its result.
func (g *graphicsStore) clear() {
	g.epoch++
	g.gen++
	g.images = nil
	g.numbers = nil
	g.places = nil
	g.total = 0
	g.transfer = nil
}

func (g *graphicsStore) image(s screen, id uint32) *gfxImage {
	return g.images[gfxKey{screen: s, id: id}]
}

func (g *graphicsStore) imageByNumber(s screen, number uint32) *gfxImage {
	if id, ok := g.numbers[gfxKey{screen: s, id: number}]; ok {
		return g.image(s, id)
	}
	return nil
}

// resolve finds the image a command names by ID, or by number when the client
// sent only I.
func (g *graphicsStore) resolve(s screen, id, number uint32) *gfxImage {
	if id != 0 {
		return g.image(s, id)
	}
	if number != 0 {
		return g.imageByNumber(s, number)
	}
	return nil
}

// allocID returns an image ID that is unused on screen s, for a client that
// transmits by number alone.
func (g *graphicsStore) allocID(s screen) uint32 {
	id := g.nextID[s] + 1
	for g.images[gfxKey{screen: s, id: id}] != nil {
		id++
	}
	g.nextID[s] = id
	return id
}

// putImage installs raster under id, replacing any earlier data of that screen
// atomically. A zero id asks for a fresh one.
func (g *graphicsStore) putImage(s screen, id, number uint32, raster image.Image, bytes int64) uint32 {
	if g.images == nil {
		g.images = make(map[gfxKey]*gfxImage)
	}
	if id == 0 {
		id = g.allocID(s)
	}
	key := gfxKey{screen: s, id: id}
	g.imgGen++
	if old, ok := g.images[key]; ok {
		g.total += bytes - old.bytes
		old.raster, old.bytes, old.gen = raster, bytes, g.imgGen
	} else {
		g.seq++
		g.images[key] = &gfxImage{key: key, gen: g.imgGen, raster: raster, bytes: bytes, seq: g.seq}
		g.total += bytes
	}
	if number != 0 {
		if g.numbers == nil {
			g.numbers = make(map[gfxKey]uint32)
		}
		// An image keeps one number: replacing its data rebinds the alias
		// instead of leaving the old one behind forever.
		for key, ref := range g.numbers {
			if key.screen == s && key.id != number && ref == id {
				delete(g.numbers, key)
			}
		}
		g.numbers[gfxKey{screen: s, id: number}] = id
	}
	g.gen++
	g.enforce()
	return id
}

// place anchors p on its screen. An explicit placement ID replaces the earlier
// placement with the same image and ID; ID 0 adds another placement.
func (g *graphicsStore) place(p *gfxPlacement) {
	g.seq++
	p.seq = g.seq
	if p.pid != 0 {
		for i, old := range g.places {
			if old.imageID == p.imageID && old.pid == p.pid && old.screen == p.screen {
				g.places[i] = p
				g.gen++
				g.enforce()
				return
			}
		}
	}
	g.places = append(g.places, p)
	g.gen++
	g.enforce()
}

func (g *graphicsStore) dropImage(s screen, id uint32) {
	key := gfxKey{screen: s, id: id}
	img, ok := g.images[key]
	if !ok {
		return
	}
	delete(g.images, key)
	g.total -= img.bytes
	for number, ref := range g.numbers {
		if ref == id && number.screen == s {
			delete(g.numbers, number)
		}
	}
	g.dropWhere(func(p *gfxPlacement) bool { return p.imageID == id && p.screen == s })
	g.gen++
}

func (g *graphicsStore) dropWhere(match func(*gfxPlacement) bool) bool {
	kept := g.places[:0]
	dropped := false
	for _, p := range g.places {
		if match(p) {
			dropped = true
			continue
		}
		kept = append(kept, p)
	}
	clear(g.places[len(kept):])
	g.places = kept
	return dropped
}

// freeIfUnplaced drops an image's data when no placement of this screen uses
// it any more, which is what the uppercase delete selectors ask for.
func (g *graphicsStore) freeIfUnplaced(s screen, id uint32) {
	if g.image(s, id) == nil {
		return
	}
	for _, p := range g.places {
		if p.screen == s && p.imageID == id {
			return
		}
	}
	g.dropImage(s, id)
}

func (g *graphicsStore) deletePlacementsOf(s screen, id uint32) {
	if g.dropWhere(func(p *gfxPlacement) bool { return p.imageID == id && p.screen == s }) {
		g.gen++
	}
}

func (g *graphicsStore) deletePlacement(s screen, id, pid uint32) {
	if g.dropWhere(func(p *gfxPlacement) bool { return p.imageID == id && p.pid == pid && p.screen == s }) {
		g.gen++
	}
}

// deleteOn drops the placements of one screen, which is what d=a does.
func (g *graphicsStore) deleteOn(s screen) {
	if g.dropWhere(func(p *gfxPlacement) bool { return p.screen == s }) {
		g.gen++
	}
}

// dropScreen removes a screen's images and placements when the screen goes
// away, as the alternate screen does when a full-screen program exits.
func (g *graphicsStore) dropScreen(s screen) {
	dropped := g.dropWhere(func(p *gfxPlacement) bool { return p.screen == s })
	for key, img := range g.images {
		if key.screen != s {
			continue
		}
		delete(g.images, key)
		g.total -= img.bytes
		dropped = true
	}
	for key := range g.numbers {
		if key.screen == s {
			delete(g.numbers, key)
		}
	}
	if dropped {
		g.gen++
	}
}

// scroll moves the placements anchored inside a scroll region by n rows and
// drops the ones that left the region. Placements of the other screen and of
// rows outside the region stay where they are.
func (g *graphicsStore) scroll(s screen, top, n, bot, cols int, up bool) {
	if n <= 0 || len(g.places) == 0 {
		return
	}
	delta := n
	if up {
		delta = -n
	}
	region := image.Rect(0, top, cols, bot+1)
	kept := g.places[:0]
	changed := false
	for _, p := range g.places {
		// Any placement with rows inside the region scrolls with them; an
		// anchor-only test would leave a tall placement pinned above the
		// margin once its anchor scrolled out.
		if p.screen != s || p.cell.Intersect(region).Empty() {
			kept = append(kept, p)
			continue
		}
		// The rows of the region scroll away with it, so from now on the
		// placement is clipped to the region: what has left it is gone, what
		// is still inside stays visible until the whole placement is out.
		if p.clip.Empty() {
			p.clip = region
		} else {
			p.clip = p.clip.Intersect(region)
		}
		p.cell = p.cell.Add(image.Pt(0, delta))
		changed = true
		if p.cell.Intersect(p.clip).Empty() {
			continue
		}
		kept = append(kept, p)
	}
	clear(g.places[len(kept):])
	g.places = kept
	if changed {
		g.gen++
	}
}

// resize follows a grid change: the main screen may have moved up by shift
// rows, and placements that no longer intersect the grid are dropped. A decode
// running outside the terminal lock computed its geometry on the old grid, so
// a real grid change drops it instead of committing a stale placement.
func (g *graphicsStore) resize(shift, rows, cols int) {
	changed := false
	if g.rows != rows || g.cols != cols {
		g.rows, g.cols = rows, cols
		g.epoch++
		// Every placement's Clip is its cell rectangle intersected with the
		// grid, so a resize changes what a frontend draws even when nothing
		// moves or is evicted.
		changed = len(g.places) > 0
	}
	if shift > 0 {
		for _, p := range g.places {
			if p.screen == screenMain {
				p.cell = p.cell.Add(image.Pt(0, -shift))
				if !p.clip.Empty() {
					// The rows the placement may still paint moved up with it.
					p.clip = p.clip.Add(image.Pt(0, -shift))
				}
				changed = true
			}
		}
	}
	if g.clip(image.Rect(0, 0, cols, rows)) || changed {
		g.gen++
	}
}

// clip drops placements that no longer intersect bounds, reporting whether any
// went away.
func (g *graphicsStore) clip(bounds image.Rectangle) bool {
	kept := g.places[:0]
	dropped := false
	for _, p := range g.places {
		if placementClip(p, bounds).Empty() {
			dropped = true
			continue
		}
		kept = append(kept, p)
	}
	if dropped {
		clear(g.places[len(kept):])
		g.places = kept
	}
	return dropped
}

// enforce evicts the oldest images and placements until the limits hold.
func (g *graphicsStore) enforce() {
	changed := false
	for len(g.images) > g.limits.images || g.total > g.limits.totalBytes {
		victim := g.oldestImage()
		if victim == nil {
			break
		}
		g.dropImage(victim.key.screen, victim.key.id)
		changed = true
	}
	if over := len(g.places) - g.limits.placements; over > 0 {
		copy(g.places, g.places[over:])
		clear(g.places[len(g.places)-over:])
		g.places = g.places[:len(g.places)-over]
		changed = true
	}
	if changed {
		g.gen++
	}
}

func (g *graphicsStore) oldestImage() *gfxImage {
	var oldest *gfxImage
	for _, img := range g.images {
		if oldest == nil || img.seq < oldest.seq {
			oldest = img
		}
	}
	return oldest
}

// placementClip is what a frontend may actually draw of p: its cell rectangle
// intersected with the screen and with the scroll region it moved in, if any.
func placementClip(p *gfxPlacement, bounds image.Rectangle) image.Rectangle {
	c := p.cell.Intersect(bounds)
	if !p.clip.Empty() {
		c = c.Intersect(p.clip)
	}
	return c
}

// visible appends the placements of s that the screen shows, reusing dst and
// releasing the rasters a longer earlier snapshot still held.
func (g *graphicsStore) visible(s screen, rows, cols int, dst []GraphicsPlacement) []GraphicsPlacement {
	out := dst[:0]
	bounds := image.Rect(0, 0, cols, rows)
	for _, p := range g.places {
		if p.screen != s {
			continue
		}
		img := g.image(s, p.imageID)
		if img == nil {
			continue
		}
		clip := placementClip(p, bounds)
		if clip.Empty() {
			continue
		}
		// The placement remembers the source window it was made with; a
		// replacement under the same id may publish a smaller raster, so the
		// window is clamped to the pixels that exist now.
		src := p.source
		if src.Empty() {
			src = img.raster.Bounds() // no source window: the whole raster
		}
		src = src.Intersect(img.raster.Bounds())
		if src.Empty() {
			continue
		}
		out = append(out, GraphicsPlacement{
			ImageID:         p.imageID,
			PlacementID:     p.pid,
			ImageGeneration: img.gen,
			Raster:          img.raster,
			CellRect:        p.cell,
			Clip:            clip,
			Source:          src,
			PixelOffset:     p.offset,
		})
	}
	if len(out) < len(dst) {
		clear(dst[len(out):])
	}
	return out
}
