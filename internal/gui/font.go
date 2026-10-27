package gui

import (
	"fmt"
	"image"
	"image/draw"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/gomonobolditalic"
	"golang.org/x/image/font/gofont/gomonoitalic"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

type style uint8

const (
	styleRegular style = iota
	styleBold
	styleItalic
	styleBoldItalic
)

func cellStyle(bold, italic bool) style {
	s := styleRegular
	if bold {
		s |= styleBold
	}
	if italic {
		s |= styleItalic
	}
	return s
}

var fcStyles = [4]string{"", ":bold", ":italic", ":bold:italic"}

var goMono = [4][]byte{gomono.TTF, gomonobold.TTF, gomonoitalic.TTF, gomonobolditalic.TTF}

// fontFile is a parsed font, shared by every size made from it.
type fontFile struct {
	sf  *sfnt.Font
	buf sfnt.Buffer
}

func (f *fontFile) has(r rune) bool {
	i, err := f.sf.GlyphIndex(&f.buf, r)
	return err == nil && i != 0
}

// fontLibrary finds and parses font files once per process.
type fontLibrary struct {
	files    map[string]*fontFile
	fallback map[rune]*fontFile
	fcMatch  bool
}

func newFontLibrary() *fontLibrary {
	_, err := exec.LookPath("fc-match")
	return &fontLibrary{files: map[string]*fontFile{}, fallback: map[rune]*fontFile{}, fcMatch: err == nil}
}

func (l *fontLibrary) open(path string, index int) *fontFile {
	key := path + "#" + strconv.Itoa(index)
	if f, ok := l.files[key]; ok {
		return f
	}
	f := parseFontFile(path, index)
	l.files[key] = f
	return f
}

func parseFontFile(path string, index int) *fontFile {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	if sf, err := sfnt.Parse(data); err == nil {
		return &fontFile{sf: sf}
	}
	coll, err := sfnt.ParseCollection(data)
	if err != nil {
		return nil
	}
	sf, err := coll.Font(index)
	if err != nil {
		return nil
	}
	return &fontFile{sf: sf}
}

func (l *fontLibrary) embedded(s style) *fontFile {
	key := "gomono#" + strconv.Itoa(int(s))
	if f, ok := l.files[key]; ok {
		return f
	}
	sf, _ := sfnt.Parse(goMono[s])
	f := &fontFile{sf: sf}
	l.files[key] = f
	return f
}

// match resolves a fontconfig pattern to a font file.
func (l *fontLibrary) match(pattern string) *fontFile {
	if !l.fcMatch {
		return nil
	}
	out, err := exec.Command("fc-match", "-f", "%{file}\n%{index}", pattern).Output()
	if err != nil {
		return nil
	}
	path, idx, _ := strings.Cut(string(out), "\n")
	index, _ := strconv.Atoi(strings.TrimSpace(idx))
	if path == "" {
		return nil
	}
	return l.open(path, index)
}

// family returns the font for spec in style s: a file path (regular only) or a
// fontconfig family, defaulting to monospace.
func (l *fontLibrary) family(spec string, s style) *fontFile {
	if isFontPath(spec) {
		if s == styleRegular {
			return l.open(spec, 0)
		}
		return nil
	}
	if spec == "" {
		spec = "monospace"
	}
	return l.match(spec + fcStyles[s])
}

func isFontPath(spec string) bool {
	lower := strings.ToLower(spec)
	return strings.ContainsRune(spec, '/') || strings.HasSuffix(lower, ".ttf") ||
		strings.HasSuffix(lower, ".otf") || strings.HasSuffix(lower, ".ttc")
}

// fallbackFor asks fontconfig for a font that has r, once per rune.
func (l *fontLibrary) fallbackFor(r rune) *fontFile {
	if f, ok := l.fallback[r]; ok {
		return f
	}
	f := l.match(fmt.Sprintf("monospace:charset=%x", r))
	if f != nil && !f.has(r) {
		f = nil
	}
	l.fallback[r] = f
	return f
}

type glyphKey struct {
	r     rune
	style style
	span  int
}

type glyph struct {
	mask *image.Alpha // in cell coordinates: (0,0) is the cell's top left
}

type sizedFace struct {
	file *fontFile
	face font.Face
}

// fontSet renders glyphs at one pixel size and fixes the cell grid.
type fontSet struct {
	lib          *fontLibrary
	opts         opentype.FaceOptions
	styles       [4]*sizedFace
	extra        map[*fontFile]*sizedFace
	glyphs       map[glyphKey]*glyph
	cellW, cellH int
	ascent       int
	underline    int
	lineWidth    int
}

// newFontSet loads spec at size points for a buffer scale.
func newFontSet(lib *fontLibrary, spec string, size float64, scale int) (*fontSet, error) {
	fs := &fontSet{
		lib:    lib,
		opts:   opentype.FaceOptions{Size: size, DPI: 96 * float64(scale), Hinting: font.HintingFull},
		extra:  map[*fontFile]*sizedFace{},
		glyphs: map[glyphKey]*glyph{},
	}
	regular := lib.family(spec, styleRegular)
	if regular == nil {
		regular = lib.embedded(styleRegular)
	}
	base, err := fs.sized(regular)
	if err != nil {
		return nil, err
	}
	fs.styles[styleRegular] = base
	m := base.face.Metrics()
	adv, _ := base.face.GlyphAdvance('M')
	fs.cellW = max(1, adv.Round())
	fs.ascent = m.Ascent.Ceil()
	fs.cellH = max(1, fs.ascent+m.Descent.Ceil())
	fs.lineWidth = max(1, int(math.Round(float64(fs.cellH)/18)))
	fs.underline = min(fs.cellH-fs.lineWidth, fs.ascent+max(1, m.Descent.Ceil()/2))
	for _, s := range []style{styleBold, styleItalic, styleBoldItalic} {
		fs.styles[s] = fs.styleFace(spec, s, regular == lib.embedded(styleRegular))
	}
	return fs, nil
}

// styleFace loads a bold or italic variant, rejecting one whose advance
// differs from the grid (fontconfig substitutes other families freely).
func (fs *fontSet) styleFace(spec string, s style, embedded bool) *sizedFace {
	f := fs.lib.embedded(s)
	if !embedded {
		f = fs.lib.family(spec, s)
	}
	if f == nil {
		return nil
	}
	sf, err := fs.sized(f)
	if err != nil {
		return nil
	}
	if adv, ok := sf.face.GlyphAdvance('M'); !ok || abs(adv.Round()-fs.cellW) > 0 {
		return nil
	}
	return sf
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func (fs *fontSet) sized(f *fontFile) (*sizedFace, error) {
	if sf, ok := fs.extra[f]; ok {
		return sf, nil
	}
	face, err := opentype.NewFace(f.sf, &fs.opts)
	if err != nil {
		return nil, err
	}
	sf := &sizedFace{file: f, face: face}
	fs.extra[f] = sf
	return sf, nil
}

// faceFor picks the first font that has r: the style's face, the regular
// face, a fontconfig fallback, then Go Mono.
func (fs *fontSet) faceFor(r rune, s style) *sizedFace {
	for _, sf := range []*sizedFace{fs.styles[s], fs.styles[styleRegular]} {
		if sf != nil && sf.file.has(r) {
			return sf
		}
	}
	if f := fs.lib.fallbackFor(r); f != nil {
		if sf, err := fs.sized(f); err == nil {
			return sf
		}
	}
	if sf, err := fs.sized(fs.lib.embedded(styleRegular)); err == nil && sf.file.has(r) {
		return sf
	}
	return nil
}

// glyph returns r's mask for a cell span cells wide, or nil if nothing draws.
func (fs *fontSet) glyph(r rune, s style, span int) *glyph {
	key := glyphKey{r, s, span}
	if g, ok := fs.glyphs[key]; ok {
		return g
	}
	g := fs.render(r, s, span)
	fs.glyphs[key] = g
	return g
}

func (fs *fontSet) render(r rune, s style, span int) *glyph {
	if r <= ' ' {
		return nil
	}
	if m := boxGlyph(r, fs.cellW, fs.cellH); m != nil {
		return &glyph{mask: m}
	}
	sf := fs.faceFor(r, s)
	if sf == nil {
		sf = fs.styles[styleRegular]
	}
	dot := fixed.P(0, fs.ascent)
	if adv, ok := sf.face.GlyphAdvance(r); ok && span > 1 {
		dot.X = (fixed.I(span*fs.cellW) - adv) / 2
	}
	dr, mask, mp, _, ok := sf.face.Glyph(dot, r)
	if !ok || dr.Empty() {
		return nil
	}
	m := image.NewAlpha(dr)
	draw.Draw(m, dr, mask, mp, draw.Src)
	return &glyph{mask: m}
}
