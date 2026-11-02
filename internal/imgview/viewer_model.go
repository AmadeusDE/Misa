package imgview

import (
	"context"
	"fmt"

	"image"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/config"
	"github.com/AmadeusDE/Misa/internal/imageframe"
	"github.com/AmadeusDE/Misa/internal/imgview/animdecode"
)

type viewerItem struct {
	path                    string
	err                     error
	loaded, loading, marked bool
	animated                bool
	animErr                 error
	rotation                int
	flipX, flipY            bool
	version, appearance     uint64
}

// Viewer is a read-only image collection and presentation state. Published pixels
// must not be modified by callers after SetImage.
type Viewer struct {
	items                []viewerItem
	index, width, height int
	grid, quit, help     bool
	gridFull             bool
	cache                *Cache
	limits               Limits
	initialFit           fitMode
	inFlight             bool
	cancel               context.CancelFunc
	drag                 bool
	dragAt               image.Point
	clicked              bool
	clickedIndex         int
	prefix               int
	fit                  fitMode
	zoom                 float64
	pan                  image.Point
	cellPixels           image.Point
	exactCellPixels      bool
	thumb                int
	note                 string
	notice               string
	rendererWarning      string
	policy               PlaybackPolicy
	clock                imageframe.Clock
	newPlayback          playbackFactory
	visible, focused     bool // host visibility; focused is settled by a visible push
	anim                 *animation
}
type fitMode int

const (
	fitWhole fitMode = iota
	fitNoUpscale
	fitWidth
	fitHeight
	fitActual
	fitManual
)

func New(paths []string, thumbnail bool, renderer string) (*Viewer, error) {
	return NewConfigured(paths, thumbnail, renderer, config.DefaultImageViewer())
}

// NewConfigured constructs a viewer with validated resource and presentation settings.
// thumbnail and renderer are caller-resolved command-line overrides.
func NewConfigured(paths []string, thumbnail bool, renderer string, cfg config.ImageViewerConfig) (*Viewer, error) {
	cfg.Validate()
	if renderer == "" {
		renderer = cfg.Renderer
	}
	if renderer != "auto" && renderer != "halfblock" && renderer != "kitty" {
		return nil, fmt.Errorf("unsupported renderer %q", renderer)
	}
	fits := map[string]fitMode{"shrink": fitNoUpscale, "contain": fitWhole, "width": fitWidth, "height": fitHeight, "actual": fitActual}
	fit := fits[cfg.Fit]
	// Thumbnail size is configured in pixels; a cell holds one sample column and
	// two sample rows. Map the nominal 128px default to the historical 16-cell
	// grid width.
	thumb := max(8, min(128, cfg.ThumbnailSize/8))
	m := &Viewer{grid: thumbnail || cfg.InitialMode == "thumbnails", fit: fit, initialFit: fit, thumb: thumb, cellPixels: defaultCellPixels,
		cache: NewCache(int64(cfg.CacheMiB) << 20), limits: Limits{MaxEncodedBytes: int64(cfg.MaxFileMiB) << 20, MaxPixels: int64(cfg.MaxPixels), Animation: animdecode.Limits{
			MaxWorkingBytes:  int64(cfg.AnimationCacheMiB) << 20,
			MaxMetadataBytes: int64(cfg.AnimationMetadataMiB) << 20,
			MaxFrames:        cfg.AnimationMaxFrames,
			MinDelay:         time.Duration(cfg.AnimationMinDelayMS) * time.Millisecond,
			GIFShortDelay:    time.Duration(cfg.GIFShortDelayMS) * time.Millisecond,
		}}}
	m.limits.workCache = m.cache
	m.policy = PlaybackPolicy{Autoplay: cfg.AnimationAutoplay, Loops: loopModeFromConfig(cfg.AnimationLoops), FPS: cfg.HalfblockAnimationFPS}
	m.newPlayback = newPlayback
	m.visible, m.focused = true, true
	if renderer == "kitty" {
		m.rendererWarning = "kitty unavailable; using halfblock"
	}
	for _, p := range paths {
		m.items = append(m.items, viewerItem{path: p})
	}
	return m, nil
}

func (m *Viewer) cacheKey(i int) string { return fmt.Sprint(i) }
func (m *Viewer) pixels(i int) *image.NRGBA {
	// The grid always shows still posters; the single view shows the committed
	// animation frame once one has been decoded.
	if a := m.anim; a != nil && a.item == i && a.frame != nil && !m.grid {
		return a.frame
	}
	d, ok := m.cache.Get(m.cacheKey(i))
	if !ok {
		return nil
	}
	return d.Pixels
}
func (m *Viewer) Index() int      { return m.index }
func (m *Viewer) Thumbnail() bool { return m.grid }
func (m *Viewer) IsQuit() bool    { return m.quit }

// Paths returns the collected image paths in presentation order.
func (m *Viewer) Paths() []string {
	paths := make([]string, len(m.items))
	for i, it := range m.items {
		paths[i] = it.path
	}
	return paths
}

// SetNotice sets a persistent status-line notice from the host, such as a
// truncated directory listing. An empty string clears it.
func (m *Viewer) SetNotice(text string) { m.notice = text }

func (m *Viewer) MarkedCount() (n int) {
	for _, i := range m.items {
		if i.marked {
			n++
		}
	}
	return
}
func (m *Viewer) Resize(w, h int) {
	// Keep viewport allocations and cell arithmetic bounded even for untrusted hosts.
	m.width = max(0, min(w, 4096))
	m.height = max(0, min(h, 4096))
	if m.width > 0 {
		m.height = min(m.height, (1<<20)/m.width)
	}
	m.clampPan()
}
func (m *Viewer) SetSize(w, h int) { m.Resize(w, h) }

// SetImage supplies a decoded image or error for an item. It is also useful to
// hosts with their own decode scheduling; nil image with nil error is an error.
func (m *Viewer) SetImage(i int, img image.Image, err error) {
	if i < 0 || i >= len(m.items) {
		return
	}
	it := &m.items[i]
	it.loaded = true
	it.loading = false
	it.err = err
	it.animated = false
	it.version++
	m.cache.Delete(m.cacheKey(i))
	var p *image.NRGBA
	if img != nil {
		b := img.Bounds()
		if b.Dx() <= 0 || b.Dy() <= 0 || int64(b.Dx()) > int64(m.limits.MaxPixels)/int64(b.Dy()) {
			it.err = ErrLimit
		} else if n, ok := img.(*image.NRGBA); ok && b.Min == (image.Point{}) {
			p = n
		} else {
			p = image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
			for y := 0; y < b.Dy(); y++ {
				for x := 0; x < b.Dx(); x++ {
					p.Set(x, y, img.At(x+b.Min.X, y+b.Min.Y))
				}
			}
		}
	}
	if p != nil && !m.cache.Put(m.cacheKey(i), Decoded{Pixels: p}) {
		it.err = fmt.Errorf("image exceeds cache budget")
	}
	if p == nil && it.err == nil {
		it.err = fmt.Errorf("no image")
	}
	m.clampPan()
}

type viewerLoaded struct {
	index   int
	version uint64
	decoded Decoded
	err     error
}

func (m *Viewer) load() tea.Cmd {
	if len(m.items) == 0 || m.quit {
		return nil
	}
	if m.inFlight {
		return nil
	}
	candidates := []int{m.index}
	if m.grid && !m.gridFull {
		start, end := m.visibleRange()
		for i := start; i < end; i++ {
			if i != m.index && !m.gridFull {
				candidates = append(candidates, i)
			}
		}
	}
	for _, i := range candidates {
		it := &m.items[i]
		if !it.loaded || (it.err == nil && m.pixels(i) == nil) {
			it.loading = true
			m.inFlight = true
			ctx, cancel := context.WithCancel(context.Background())
			m.cancel = cancel
			path, version, limits := it.path, it.version, m.limits
			return func() tea.Msg {
				l := NewLoader(1, 1)
				defer l.Close()
				l.Submit(Request{Context: ctx, Path: path, Limits: limits, Generation: version, Visible: true})
				select {
				case r := <-l.Results():
					return viewerLoaded{i, version, r.Image, r.Err}
				case <-ctx.Done():
					return viewerLoaded{i, version, Decoded{}, ctx.Err()}
				}
			}
		}
	}
	return nil
}
func (m *Viewer) Init() tea.Cmd { return m.load() }
func (m *Viewer) Update(msg tea.Msg) tea.Cmd {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.Resize(v.Width, v.Height)
	case tea.KeyPressMsg:
		m.key(v)
	case tea.FocusMsg:
		m.focused = true
	case tea.BlurMsg:
		m.focused = false
	case tea.MouseClickMsg:
		m.click(v.Mouse())
	case tea.MouseWheelMsg:
		m.wheel(v.Mouse())
	case tea.MouseMotionMsg:
		m.motion(v.Mouse())
	case tea.MouseReleaseMsg:
		m.drag = false
	case viewerLoaded:
		m.loaded(v)
	case viewerAnimation:
		m.animationOpened(v)
	case viewerFrame:
		m.frameDecoded(v)
	case viewerTick:
		m.tick(v)
	}
	if a := m.anim; a != nil && (a.item != m.index || a.version != m.items[a.item].version) {
		m.stopAnimation()
	}
	if m.quit {
		m.stopAnimation()
		if m.cancel != nil {
			m.cancel()
		}
	}
	m.syncAnimation()
	return tea.Batch(m.load(), m.ensureAnimation(), m.animCmd())
}

func (m *Viewer) loaded(v viewerLoaded) {
	m.inFlight = false
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	if v.index < 0 || v.index >= len(m.items) {
		return
	}
	it := &m.items[v.index]
	if it.version != v.version {
		return
	}
	if v.decoded.Pixels == nil {
		m.SetImage(v.index, nil, v.err)
	} else {
		m.SetImage(v.index, v.decoded.Pixels, v.err)
	}
	it.animated = v.decoded.Animated
	// Stop filling this page when admitting a new image displaced one.
	if m.grid {
		start, end := m.visibleRange()
		for j := start; j < end; j++ {
			if j != v.index && m.items[j].loaded && m.items[j].err == nil && m.pixels(j) == nil {
				m.gridFull = true
				break
			}
		}
	}
}
func safeName(p string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == 0x7f || r == 0x2028 || r == 0x2029 {
			return '�'
		}
		return r
	}, filepath.Base(p))
}
func (m *Viewer) selectIndex(i int) {
	if len(m.items) == 0 {
		return
	}
	i = max(0, min(i, len(m.items)-1))
	if i != m.index {
		m.index = i
		m.clicked = false
		m.pan = image.Point{}
		m.fit = m.initialFit
		m.gridFull = false
		m.note = ""
	}
}
