// readelf.go - display information about ELF files
//
// Ported from toybox toys/other/readelf.c (with lib/elf.c arch tables), commit
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb. Toybox is 0BSD licensed; see
// docs/CREDITS.md and THIRD_PARTY_NOTICES.md. Original copyright 2019 The
// Android Open Source Project; the Go port below is original Go code written
// against the same behaviour and the same output format.
//
// Deliberate differences from the C:
//
//   - Metadata is read with ReadAt (pread) and every access is checked against
//     the real file size, the way the C's elf_get()/fits() do, instead of
//     indexing into an mmap of the whole file. A truncated file, a section or a
//     symbol table that runs past EOF therefore prints toybox's own message
//     ("Access off end: off[n] of size", "No shdr n", "Bad phdr n") and fails the
//     file with exit 1 rather than walking memory. Those messages carry the
//     file name, and a bad file only fails that operand: the C reports through
//     perror_exit(), which appends whatever strerror(errno) happens to say and
//     aborts the whole command.
//   - shnum/phnum/shentsize are cross-checked against the bytes actually present
//     before any section header or program header is read, so a hostile header
//     with a huge count or an offset beyond EOF reports the first count that fits
//     and stops.
//   - Strings from a string table are truncated at the first NUL or at EOF;
//     the C trusts NUL termination (its own "TODO: trusting this %s" comments).
//     -p and -x dumps are likewise bounded by the file size, not by sh_size.
//   - The GNU ABI note prints its three version words in field order
//     (major.minor.subminor). The C passes four side-effecting elf_int() calls
//     straight to printf, so its output depends on the compiler's argument
//     evaluation order; on x86-64 glibc a 3.2.0 tag comes out as "2.3.0". This
//     port prints 3.2.0, which is what the tag actually says.

package builtins

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// readelfSpec is the option spec. -W is accepted and ignored: like toybox we
// are always wide.
const readelfSpec = "(dyn-syms)Aadehlnp:SsWx:"

var readelfSpecParsed *Spec

func init() {
	var err error
	readelfSpecParsed, err = ParseSpec(readelfSpec)
	if err != nil {
		panic("builtins: bad readelf spec: " + err.Error())
	}
	Register(Command{Name: "readelf", Run: runReadelf})
}

// Section types.
const (
	readelfSHTNull            = 0
	readelfSHTSymtab          = 2
	readelfSHTStrtab          = 3
	readelfSHTDynamic         = 6
	readelfSHTNote            = 7
	readelfSHTNobits          = 8
	readelfSHTDynsym          = 11
	readelfSHTRiscvAttributes = 0x70000003
)

// Program header types.
const (
	readelfPTLoad    = 1
	readelfPTDynamic = 2
	readelfPTInterp  = 3
	readelfPTNote    = 4
)

// Machines we have per-arch output for (Linux kernel numbering).
const (
	readelfEMARM   = 40
	readelfEMARM64 = 183
	readelfEMRISCV = 243
	readelfEMX8664 = 62
)

// errReadelfSoft ends the current dump after a message has already been
// reported; the rest of the file is still scanned.
var errReadelfSoft = errors.New("readelf: soft")

// readelfName is one row of a name table.
type readelfName struct {
	v uint64
	s string
}

func readelfLookup(table []readelfName, v uint64) string {
	for _, e := range table {
		if e.v == v {
			return e.s
		}
	}
	return fmt.Sprintf("0x%x", v)
}

var readelfDTTypes = []readelfName{
	{0, "x(NULL)"}, {1, "N(NEEDED)"}, {2, "b(PLTRELSZ)"},
	{3, "x(PLTGOT)"}, {4, "x(HASH)"}, {5, "x(STRTAB)"}, {6, "x(SYMTAB)"}, {7, "x(RELA)"},
	{8, "b(RELASZ)"}, {9, "b(RELAENT)"}, {10, "b(STRSZ)"}, {11, "b(SYMENT)"},
	{12, "x(INIT)"}, {13, "x(FINI)"}, {14, "S(SONAME)"}, {15, "R(RPATH)"},
	{16, "x(SYMBOLIC)"}, {17, "x(REL)"}, {18, "b(RELSZ)"}, {19, "b(RELENT)"},
	{20, "P(PLTREL)"}, {21, "x(DEBUG)"}, {22, "x(TEXTREL)"}, {23, "x(JMPREL)"},
	{24, "d(BIND_NOW)"}, {25, "x(INIT_ARRAY)"}, {26, "x(FINI_ARRAY)"},
	{27, "b(INIT_ARRAYSZ)"}, {28, "b(FINI_ARRAYSZ)"}, {29, "R(RUNPATH)"},
	{30, "f(FLAGS)"}, {32, "x(PREINIT_ARRAY)"}, {33, "x(PREINIT_ARRAYSZ)"},
	{35, "b(RELRSZ)"}, {36, "x(RELR)"}, {37, "b(RELRENT)"},
	{0x6000000f, "x(ANDROID_REL)"}, {0x60000010, "b(ANDROID_RELSZ)"},
	{0x60000011, "x(ANDROID_RELA)"}, {0x60000012, "b(ANDROID_RELASZ)"},
	{0x6fffe000, "x(ANDROID_RELR)"}, {0x6fffe001, "b(ANDROID_RELRSZ)"},
	{0x6fffe003, "x(ANDROID_RELRENT)"}, {0x6ffffef5, "x(GNU_HASH)"},
	{0x6ffffef6, "x(TLSDESC_PLT)"}, {0x6ffffef7, "x(TLSDESC_GOT)"},
	{0x6ffffff0, "x(VERSYM)"}, {0x6ffffff9, "d(RELACOUNT)"},
	{0x6ffffffa, "d(RELCOUNT)"}, {0x6ffffffb, "F(FLAGS_1)"},
	{0x6ffffffc, " (VERDEF)"}, {0x6ffffffd, "d(VERDEFNUM)"},
	{0x6ffffffe, "x(VERNEED)"}, {0x6fffffff, "d(VERNEEDNUM)"},
}

var readelfETTypes = []readelfName{
	{0, "NONE (None)"}, {1, "REL (Relocatable file)"},
	{2, "EXEC (Executable file)"}, {3, "DYN (Shared object file)"},
	{4, "CORE (Core file)"},
}

var readelfNTTypeCore = []readelfName{
	{1, "NT_PRSTATUS"}, {2, "NT_FPREGSET"}, {3, "NT_PRPSINFO"},
	{5, "NT_PLATFORM"}, {6, "NT_AUXV"},
	{0x46494c45, "NT_FILE"}, {0x53494749, "NT_SIGINFO"},
}

var readelfNTTypeLinux = []readelfName{
	{0x200, "NT_386_TLS"}, {0x202, "NT_X86_XSTATE"},
	{0x400, "NT_ARM_VFP"}, {0x401, "NT_ARM_TLS"}, {0x405, "NT_ARM_SVE"},
}

var readelfOSABI = []readelfName{{0, "UNIX - System V"}}

var readelfPHTypes = []readelfName{
	{0, "NULL"}, {1, "LOAD"}, {2, "DYNAMIC"}, {3, "INTERP"},
	{4, "NOTE"}, {5, "SHLIB"}, {6, "PHDR"}, {7, "TLS"},
	{0x6474e550, "GNU_EH_FRAME"}, {0x6474e551, "GNU_STACK"},
	{0x6474e552, "GNU_RELRO"}, {0x70000001, "EXIDX"},
}

var readelfSHTypes = []readelfName{
	{0, "NULL"}, {1, "PROGBITS"}, {2, "SYMTAB"}, {3, "STRTAB"},
	{4, "RELA"}, {5, "HASH"}, {6, "DYNAMIC"}, {7, "NOTE"}, {8, "NOBITS"},
	{9, "REL"}, {10, "SHLIB"}, {11, "DYNSYM"}, {14, "INIT_ARRAY"},
	{15, "FINI_ARRAY"}, {16, "PREINIT_ARRAY"}, {17, "GROUP"},
	{18, "SYMTAB_SHNDX"}, {19, "RELR"},
	{0x60000001, "ANDROID_REL"}, {0x60000002, "ANDROID_RELA"},
	{0x6fffff00, "ANDROID_RELR"}, {0x6ffffff6, "GNU_HASH"},
	{0x6ffffffd, "VERDEF"}, {0x6ffffffe, "VERNEED"},
	{0x6fffffff, "VERSYM"}, {0x70000001, "ARM_EXIDX"},
	{0x70000003, "ATTRIBUTES"},
}

var readelfSTBTypes = []readelfName{{0, "LOCAL"}, {1, "GLOBAL"}, {2, "WEAK"}}

var readelfSTTTypes = []readelfName{
	{0, "NOTYPE"}, {1, "OBJECT"}, {2, "FUNC"}, {3, "SECTION"},
	{4, "FILE"}, {5, "COMMON"}, {6, "TLS"}, {10, "GNU_IFUNC"},
}

var readelfSTVTypes = []readelfName{
	{0, "DEFAULT"}, {1, "INTERNAL"}, {2, "HIDDEN"}, {3, "PROTECTED"},
}

var readelfRISCVAttrTags = []readelfName{
	{4, "stack_align"}, {5, "arch"}, {6, "unaligned_access"},
	{8, "priv_spec"}, {10, "priv_spec_minor"}, {12, "priv_spec_revision"},
	{14, "atomic_abi"}, {16, "x3_reg_usage"},
}

var readelfArchNames = []readelfName{
	{0x9026, "alpha"}, {93, "arc"}, {195, "arcv2"}, {40, "arm"}, {183, "arm64"},
	{0x18ad, "avr32"}, {247, "bpf"}, {106, "blackfin"}, {140, "c6x"}, {23, "cell"},
	{76, "cris"}, {252, "csky"}, {0x5441, "frv"}, {46, "h8300"}, {164, "hexagon"},
	{50, "ia64"}, {258, "loongarch"}, {88, "m32r"}, {0x9041, "m32r"},
	{4, "m68k"}, {174, "metag"}, {189, "microblaze"}, {0xbaab, "microblaze-old"},
	{8, "mips"}, {10, "mips-old"}, {89, "mn10300"}, {0xbeef, "mn10300-old"},
	{113, "nios2"}, {92, "openrisc"}, {0x8472, "openrisc-old"}, {15, "parisc"},
	{20, "ppc"}, {21, "ppc64"}, {243, "riscv"}, {22, "s390"}, {0xa390, "s390-old"},
	{135, "score"}, {42, "sh"}, {2, "sparc"}, {18, "sparc8+"}, {43, "sparc9"},
	{188, "tile"}, {191, "tilegx"}, {3, "386"}, {6, "486"}, {62, "x86-64"},
	{94, "xtensa"}, {0xabc7, "xtensa-old"},
}

func readelfArchName(machine int) string {
	for _, e := range readelfArchNames {
		if int(e.v) == machine {
			return e.s
		}
	}
	return fmt.Sprintf("unknown arch %d", machine)
}

// readelfPrintFlags is lib/elf.c's elf_print_flags: a comma-prefixed suffix
// for the "Flags:" header line.
func readelfPrintFlags(arch int, flags int) string {
	var b strings.Builder
	switch arch {
	case readelfEMARM: // arm32
		fmt.Fprintf(&b, ", EABI%d", (flags>>24)&0xf)
		if flags&0x200 != 0 {
			b.WriteString(", soft float")
		} else if flags&0x400 != 0 {
			b.WriteString(", hard float")
		}
	case readelfEMRISCV:
		if flags&1 != 0 {
			b.WriteString(", C")
		}
		if flags&8 != 0 {
			b.WriteString(", E")
		}
		if flags&0x10 != 0 {
			b.WriteString(", TSO")
		}
		floatNames := []string{"soft", "single", "double", "quad"}
		fmt.Fprintf(&b, ", %s float", floatNames[(flags&0x6)/2])
	}
	return b.String()
}

// readelfSection is one decoded section header.
type readelfSection struct {
	name      string
	typ       uint32
	flags     uint64
	addr      uint64
	offset    uint64
	size      uint64
	link      uint32
	info      uint32
	addralign uint64
	entsize   uint64
}

// readelfPhdr is one decoded program header.
type readelfPhdr struct {
	typ    uint32
	flags  uint32
	offset uint64
	vaddr  uint64
	paddr  uint64
	filesz uint64
	memsz  uint64
	align  uint64
}

// readelfCtx is the per-file state (toybox's GLOBALS for readelf).
type readelfCtx struct {
	ctx   *Context
	w     *bufio.Writer
	file  *os.File
	f     string
	size  uint64
	multi bool

	bits   int // 0 = 32-bit, 1 = 64-bit
	endian int // 1 = little, 2 = big

	phoff, shoff         uint64
	phentsize, shentsize int
	phnum, shnum         int
	entry                uint64
	machine              int
	etype                int

	hasShstr bool
	shstr    []byte

	shCount int
	phCount int

	secCache []*readelfSection
	secBad   []bool
	phCache  []*readelfPhdr
	phBad    []bool

	doA, doD, doH, doL, doN, doS, doSym, doDynSyms bool
	xSpec, pSpec                                   string
	hasX, hasP                                     bool

	failed bool
}

func runReadelf(ctx *Context) int {
	flags, err := readelfSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "readelf:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, readelfSpecParsed) {
		return 0
	}

	doA := flags.Bool("A")
	doD := flags.Bool("d")
	doH := flags.Bool("h")
	doL := flags.Bool("l")
	doN := flags.Bool("n")
	doS := flags.Bool("S")
	doSym := flags.Bool("s")
	doDynSyms := flags.Bool("dyn-syms")
	xSpec, hasX := flags.ExplicitString("x")
	pSpec, hasP := flags.ExplicitString("p")

	if flags.Bool("a") {
		doA, doD, doH, doL, doN, doS, doSym, doDynSyms =
			true, true, true, true, true, true, true, true
	}
	if flags.Bool("e") {
		doH, doL, doS = true, true, true
	}
	if doSym {
		doDynSyms = true
	}
	if len(flags.Operands) == 0 {
		fmt.Fprintln(ctx.Stderr, "readelf: missing operand")
		return 1
	}
	if !(doA || doD || doH || doL || doN || doS || doSym || doDynSyms || hasX || hasP) {
		fmt.Fprintln(ctx.Stderr, "readelf: needs a flag (see \"readelf --help\")")
		return 1
	}

	opts := readelfOptions{
		doA: doA, doD: doD, doH: doH, doL: doL, doN: doN, doS: doS,
		doSym: doSym, doDynSyms: doDynSyms,
		xSpec: xSpec, hasX: hasX, pSpec: pSpec, hasP: hasP,
	}
	status := 0
	multi := len(flags.Operands) > 1
	for _, name := range flags.Operands {
		if !readelfOne(ctx, name, multi, opts) {
			status = 1
		}
	}
	return status
}

// readelfOptions is the post-implication flag set.
type readelfOptions struct {
	doA, doD, doH, doL, doN, doS, doSym, doDynSyms bool
	xSpec, pSpec                                   string
	hasX, hasP                                     bool
}

func readelfOne(ctx *Context, name string, multi bool, opts readelfOptions) bool {

	f, err := os.Open(resolvePath(ctx, name))
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "readelf: %s: %s\n", name, fmtOsErr(err))
		return false
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "readelf: %s: %s\n", name, fmtOsErr(err))
		return false
	}
	if st.Size() == 0 {
		fmt.Fprintf(ctx.Stderr, "readelf: %s: empty\n", name)
		return false
	}
	if !st.Mode().IsRegular() {
		fmt.Fprintf(ctx.Stderr, "readelf: %s: not a regular file\n", name)
		return false
	}

	w := bufio.NewWriter(ctx.Stdout)
	r := &readelfCtx{
		ctx:       ctx,
		w:         w,
		file:      f,
		f:         name,
		size:      uint64(st.Size()),
		multi:     multi,
		doA:       opts.doA,
		doD:       opts.doD,
		doH:       opts.doH,
		doL:       opts.doL,
		doN:       opts.doN,
		doS:       opts.doS,
		doSym:     opts.doSym,
		doDynSyms: opts.doDynSyms,
		xSpec:     opts.xSpec,
		hasX:      opts.hasX,
		pSpec:     opts.pSpec,
		hasP:      opts.hasP,
	}
	err = r.scan()
	w.Flush()
	if err != nil && !errors.Is(err, errReadelfSoft) {
		fmt.Fprintf(ctx.Stderr, "readelf: %s: %s\n", name, err)
		return false
	}
	return !r.failed
}

// softf reports a per-file problem: toybox's error_msg() wording, on stderr
// with the file name, and the run fails.
func (r *readelfCtx) softf(format string, args ...any) {
	fmt.Fprintf(r.ctx.Stderr, "readelf: %s: ", r.f)
	fmt.Fprintf(r.ctx.Stderr, format, args...)
	fmt.Fprintln(r.ctx.Stderr)
	r.failed = true
}

// errorf reports a problem and stops scanning this file.
func (r *readelfCtx) errorf(format string, args ...any) error {
	r.softf(format, args...)
	return errReadelfSoft
}

func (r *readelfCtx) offEnd(off, n uint64) error {
	return fmt.Errorf("Access off end: %d[%d] of %d", off, n, r.size)
}

// bytes reads exactly n bytes at off, mirroring elf_get's bounds check.
func (r *readelfCtx) bytes(off, n uint64) ([]byte, error) {
	if off > r.size || n > r.size-off {
		return nil, r.offEnd(off, n)
	}
	if n == 0 {
		return nil, nil
	}
	buf := make([]byte, n)
	if _, err := r.file.ReadAt(buf, int64(off)); err != nil {
		return nil, r.offEnd(off, n)
	}
	return buf, nil
}

// slice returns as much of [off, off+n) as the file holds; used by the byte
// dumpers, which must not read out of bounds.
func (r *readelfCtx) slice(off, n uint64) []byte {
	if off >= r.size || n == 0 {
		return nil
	}
	if n > r.size-off {
		n = r.size - off
	}
	buf := make([]byte, n)
	m, _ := r.file.ReadAt(buf, int64(off))
	return buf[:m]
}

// cstring reads a NUL-terminated string, truncating at end of file.
func (r *readelfCtx) cstring(off uint64) string {
	var out []byte
	for off < r.size {
		n := r.size - off
		if n > 256 {
			n = 256
		}
		b := r.slice(off, n)
		if i := bytes.IndexByte(b, 0); i >= 0 {
			return string(append(out, b[:i]...))
		}
		out = append(out, b...)
		if len(b) == 0 {
			break
		}
		off += uint64(len(b))
	}
	return string(out)
}

// readelfCStringBytes cuts b at the first NUL, the way printf("%s") would.
func readelfCStringBytes(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		return string(b[:i])
	}
	return string(b)
}

// readelfCursor reads fields in the file's byte order, tracking the current
// offset the way toybox advances a char*.
type readelfCursor struct {
	r   *readelfCtx
	off uint64
	err error
}

func (r *readelfCtx) cursor(off uint64) *readelfCursor {
	return &readelfCursor{r: r, off: off}
}

func (c *readelfCursor) take(n uint64) []byte {
	if c.err != nil {
		return nil
	}
	b, err := c.r.bytes(c.off, n)
	if err != nil {
		c.err = err
		return nil
	}
	c.off += n
	return b
}

func (c *readelfCursor) u8() uint8 {
	b := c.take(1)
	if len(b) < 1 {
		return 0
	}
	return b[0]
}

func (c *readelfCursor) u16() uint16 {
	b := c.take(2)
	if len(b) < 2 {
		return 0
	}
	if c.r.endian == 2 {
		return binary.BigEndian.Uint16(b)
	}
	return binary.LittleEndian.Uint16(b)
}

func (c *readelfCursor) u32() uint32 {
	b := c.take(4)
	if len(b) < 4 {
		return 0
	}
	if c.r.endian == 2 {
		return binary.BigEndian.Uint32(b)
	}
	return binary.LittleEndian.Uint32(b)
}

func (c *readelfCursor) u64() uint64 {
	b := c.take(8)
	if len(b) < 8 {
		return 0
	}
	if c.r.endian == 2 {
		return binary.BigEndian.Uint64(b)
	}
	return binary.LittleEndian.Uint64(b)
}

// word reads an ELF "long": 4 bytes for ELF32, 8 for ELF64.
func (c *readelfCursor) word() uint64 {
	if c.r.bits != 0 {
		return c.u64()
	}
	return uint64(c.u32())
}

// fits mirrors readelf.c's fits(): the range must be inside the file.
func (r *readelfCtx) fits(what string, n int, off, size uint64) bool {
	if off > r.size || size > r.size || off > r.size-size {
		var extra string
		if n != -1 {
			extra = fmt.Sprintf(" %d", n)
		}
		r.softf("%s%s's offset %d + size %d > file size %d", what, extra, off, size, r.size)
		return false
	}
	return true
}

// shName resolves a section name offset against the section header string
// table, truncating at the table/file end.
func (r *readelfCtx) shName(off uint64) (string, bool) {
	if !r.hasShstr {
		return "?", true
	}
	if off > uint64(len(r.shstr)) {
		return "", false
	}
	return readelfCStringBytes(r.shstr[off:]), true
}

// sectionCount clamps the section count to the headers that actually fit in
// the file, reporting the first one that does not.
func (r *readelfCtx) sectionCount() int {
	if r.shnum == 0 {
		return 0
	}
	if r.shentsize <= 0 {
		return 0
	}
	avail := uint64(0)
	if r.shoff < r.size {
		avail = r.size - r.shoff
	}
	fit := avail / uint64(r.shentsize)
	if fit < uint64(r.shnum) {
		r.softf("No shdr %d", fit)
		return int(fit)
	}
	return r.shnum
}

func (r *readelfCtx) programCount() int {
	if r.phnum == 0 {
		return 0
	}
	if r.phentsize <= 0 {
		return 0
	}
	avail := uint64(0)
	if r.phoff < r.size {
		avail = r.size - r.phoff
	}
	fit := avail / uint64(r.phentsize)
	if fit < uint64(r.phnum) {
		r.softf("Bad phdr %d", fit)
		return int(fit)
	}
	return r.phnum
}

// section decodes section header i, caching the result (the program header
// mapping loop walks every section for every segment).
func (r *readelfCtx) section(i int) (*readelfSection, error) {
	if i < 0 || i >= r.shCount {
		if i >= 0 && i < len(r.secBad) && r.secBad[i] {
			return nil, errReadelfSoft
		}
		if i >= 0 && i < len(r.secBad) {
			r.secBad[i] = true
		}
		r.softf("No shdr %d", i)
		return nil, errReadelfSoft
	}
	if s := r.secCache[i]; s != nil {
		return s, nil
	}
	if r.secBad[i] {
		return nil, errReadelfSoft
	}
	s, err := r.loadSection(i)
	if err != nil {
		r.secBad[i] = true
		return nil, err
	}
	r.secCache[i] = s
	return s, nil
}

func (r *readelfCtx) loadSection(i int) (*readelfSection, error) {
	off := r.shoff + uint64(i)*uint64(r.shentsize)
	c := r.cursor(off)
	nameOff := c.u32()
	s := &readelfSection{}
	s.typ = c.u32()
	s.flags = c.word()
	s.addr = c.word()
	s.offset = c.word()
	s.size = c.word()
	s.link = c.u32()
	s.info = c.u32()
	s.addralign = c.word()
	s.entsize = c.word()
	if c.err != nil {
		return nil, c.err
	}
	if s.typ != readelfSHTNobits && !r.fits("section header", i, s.offset, s.size) {
		return nil, errReadelfSoft
	}
	name, ok := r.shName(uint64(nameOff))
	if !ok {
		r.softf("Bad name for sh %d", i)
		return nil, errReadelfSoft
	}
	s.name = name
	return s, nil
}

// programHeader decodes program header i, caching the result.
func (r *readelfCtx) programHeader(i int) (*readelfPhdr, error) {
	if i < 0 || i >= r.phCount {
		if i >= 0 && i < len(r.phBad) {
			if r.phBad[i] {
				return nil, errReadelfSoft
			}
			r.phBad[i] = true
		}
		r.softf("Bad phdr %d", i)
		return nil, errReadelfSoft
	}
	if ph := r.phCache[i]; ph != nil {
		return ph, nil
	}
	if r.phBad[i] {
		return nil, errReadelfSoft
	}
	ph, err := r.loadProgramHeader(i)
	if err != nil {
		r.phBad[i] = true
		return nil, err
	}
	r.phCache[i] = ph
	return ph, nil
}

func (r *readelfCtx) loadProgramHeader(i int) (*readelfPhdr, error) {
	off := r.phoff + uint64(i)*uint64(r.phentsize)
	c := r.cursor(off)
	ph := &readelfPhdr{}
	ph.typ = c.u32()
	// Elf64_Phdr reordered fields.
	if r.bits != 0 {
		ph.flags = c.u32()
		ph.offset = c.u64()
		ph.vaddr = c.u64()
		ph.paddr = c.u64()
		ph.filesz = c.u64()
		ph.memsz = c.u64()
		ph.align = c.u64()
	} else {
		ph.offset = uint64(c.u32())
		ph.vaddr = uint64(c.u32())
		ph.paddr = uint64(c.u32())
		ph.filesz = uint64(c.u32())
		ph.memsz = uint64(c.u32())
		ph.flags = c.u32()
		ph.align = uint64(c.u32())
	}
	if c.err != nil {
		return nil, c.err
	}
	if !r.fits("program header", i, ph.offset, ph.filesz) {
		return nil, errReadelfSoft
	}
	return ph, nil
}

// findSection resolves a section spec: a number (base 0, as estrtol) or a
// name.
func (r *readelfCtx) findSection(spec string) (*readelfSection, error) {
	if spec == "" {
		// estrtol("") yields 0 with nothing left over.
		if r.shCount > 0 {
			s, err := r.section(0)
			if err != nil {
				return nil, err
			}
			return s, nil
		}
	} else if n, ok := readelfParseNum(spec); ok && n < uint64(r.shCount) {
		s, err := r.section(int(n))
		if err != nil {
			return nil, err
		}
		return s, nil
	}
	for i := 0; i < r.shCount; i++ {
		s, err := r.section(i)
		if err != nil {
			if errors.Is(err, errReadelfSoft) {
				continue
			}
			return nil, err
		}
		if s.name == spec {
			return s, nil
		}
	}
	r.softf("no section '%s", spec)
	return nil, errReadelfSoft
}

// readelfParseNum is estrtol(spec, &end, 0): a base-0 integer with the whole
// string consumed.
func readelfParseNum(s string) (uint64, bool) {
	t := strings.TrimLeft(s, " \t\n\v\f\r")
	if t == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(t, 0, 64)
	if err != nil || n < 0 {
		return 0, false
	}
	return uint64(n), true
}

func (r *readelfCtx) scan() error {
	if r.size < 45 {
		return r.errorf("not ELF")
	}
	ident, err := r.bytes(0, 16)
	if err != nil {
		return err
	}
	if ident[0] != 0x7f || ident[1] != 'E' || ident[2] != 'L' || ident[3] != 'F' {
		return r.errorf("not ELF")
	}

	r.bits = int(ident[4]) - 1
	r.endian = int(ident[5])
	if r.bits < 0 || r.bits > 1 || r.endian < 1 || r.endian > 2 || ident[6] != 1 {
		return r.errorf("bad ELF")
	}

	hdr := r.cursor(16) // EI_NIDENT
	etype := hdr.u16()
	machine := hdr.u16()
	version := hdr.u32()
	entry := hdr.word()
	r.phoff = hdr.word()
	r.shoff = hdr.word()
	flags := hdr.u32()
	ehsize := hdr.u16()
	r.phentsize = int(hdr.u16())
	phnum := int(hdr.u16())
	r.phnum = phnum
	r.shentsize = int(hdr.u16())
	r.shnum = int(hdr.u16())
	shstrndx := int(hdr.u16())
	if hdr.err != nil {
		return hdr.err
	}
	r.etype = int(etype)
	r.machine = int(machine)
	r.entry = entry

	if r.multi {
		fmt.Fprintf(r.w, "\nFile: %s\n", r.f)
	}

	if r.doH {
		r.printHeader(ident, int(version), int(flags), int(ehsize), phnum, shstrndx)
	}

	if r.phoff > r.size {
		return r.errorf("bad phoff")
	}
	if r.shoff > r.size {
		return r.errorf("bad shoff")
	}
	if r.shentsize == 0 && r.shnum > 0 {
		return r.errorf("bad shentsize")
	}
	if r.phentsize == 0 && phnum > 0 {
		return r.errorf("bad phentsize")
	}

	r.shCount = r.sectionCount()
	r.phCount = r.programCount()
	if r.shCount > 0 {
		r.secCache = make([]*readelfSection, r.shCount)
		r.secBad = make([]bool, r.shCount)
	}
	if r.phCount > 0 {
		r.phCache = make([]*readelfPhdr, r.phCount)
		r.phBad = make([]bool, r.phCount)
	}

	// Set up the section header string table so section names resolve.
	// Core files have shstrndx == 0.
	r.hasShstr = false
	if shstrndx != 0 {
		s, err := r.loadSectionOrNil(shstrndx)
		if err != nil {
			return err
		}
		if s == nil || s.typ != readelfSHTStrtab {
			return r.errorf("bad shstrndx")
		}
		r.hasShstr = true
		r.shstr = r.slice(s.offset, s.size)
	}

	w := 8 << r.bits

	if r.doS {
		if r.shnum == 0 {
			fmt.Fprint(r.w, "\nThere are no sections in this file.\n")
		} else {
			if !r.doH {
				fmt.Fprintf(r.w, "There are %d section headers, starting at offset %s:\n", r.shnum, readelfAltHex(r.shoff))
			}
			fmt.Fprintf(r.w, "\nSection Headers:\n"+
				"  [Nr] %-17s %-15s %-*s %-6s %-6s ES Flg Lk Inf Al\n",
				"Name", "Type", w, "Address", "Off", "Size")
		}
	}

	// We iterate through the section headers even when not dumping them, to
	// find the tables the other flags need.
	var dynamic, dynstr, dynsym, strtab, symtab readelfSection
	for i := 0; i < r.shCount; i++ {
		s, err := r.section(i)
		if err != nil {
			if errors.Is(err, errReadelfSoft) {
				continue
			}
			return err
		}
		switch s.typ {
		case readelfSHTSymtab:
			symtab = *s
		case readelfSHTDynamic:
			dynamic = *s
		case readelfSHTDynsym:
			dynsym = *s
		case readelfSHTStrtab:
			if s.name == ".strtab" {
				strtab = *s
			} else if s.name == ".dynstr" {
				dynstr = *s
			}
		}

		if r.doS {
			fmt.Fprintf(r.w, "  [%2d] %-17s %-15s %0*x %06x %06x %02x %3s %2d %2d %2d\n",
				i, s.name, readelfLookup(readelfSHTypes, uint64(s.typ)), w,
				s.addr, s.offset, s.size, s.entsize,
				readelfSectionFlags(s.flags, r.machine), s.link, s.info, s.addralign)
		}
	}
	if r.doS && r.shnum != 0 {
		fmt.Fprint(r.w, "Key:\n  (W)rite, (A)lloc, e(X)ecute, (M)erge, (S)trings, (I)nfo\n"+
			"  (L)ink order, (O)S, (G)roup, (T)LS, (C)ompressed, (x) unknown\n")
		if r.machine == readelfEMARM || r.machine == readelfEMARM64 {
			fmt.Fprint(r.w, "  (y) purecode\n")
		} else if r.machine == readelfEMX8664 {
			fmt.Fprint(r.w, "  (l) large\n")
		}
	}

	if r.doL {
		if err := r.printProgramHeaders(w, phnum); err != nil {
			return err
		}
	}

	if r.doD {
		if err := r.printDynamic(w, &dynamic, &dynstr); err != nil {
			return err
		}
	}

	if r.doDynSyms {
		if err := r.showSymbols(&dynsym, &dynstr); err != nil {
			return err
		}
	}
	if r.doSym {
		if err := r.showSymbols(&symtab, &strtab); err != nil {
			return err
		}
	}

	if r.doN {
		found := false
		for i := 0; i < r.shCount; i++ {
			s, err := r.section(i)
			if err != nil {
				if errors.Is(err, errReadelfSoft) {
					continue
				}
				return err
			}
			if s.typ == readelfSHTNote {
				fmt.Fprintf(r.w, "\nDisplaying notes found in: %s\n", s.name)
				if err := r.showNotes(s.offset, s.size); err != nil && !errors.Is(err, errReadelfSoft) {
					return err
				}
				found = true
			}
		}
		for i := 0; !found && i < r.phCount; i++ {
			ph, err := r.programHeader(i)
			if err != nil {
				if errors.Is(err, errReadelfSoft) {
					continue
				}
				return err
			}
			if ph.typ == readelfPTNote {
				fmt.Fprintf(r.w, "\nDisplaying notes found at file offset 0x%x with length 0x%x:\n",
					ph.offset, ph.filesz)
				if err := r.showNotes(ph.offset, ph.filesz); err != nil && !errors.Is(err, errReadelfSoft) {
					return err
				}
			}
		}
	}

	// TODO: ARC/ARM/CSKY have these too.
	if r.doA && r.machine == readelfEMRISCV {
		for i := 0; i < r.shCount; i++ {
			s, err := r.section(i)
			if err != nil {
				if errors.Is(err, errReadelfSoft) {
					continue
				}
				return err
			}
			if s.typ == readelfSHTRiscvAttributes {
				if err := r.showAttributes(s.offset, s.size); err != nil && !errors.Is(err, errReadelfSoft) {
					return err
				}
			}
		}
	}

	if r.hasX {
		s, err := r.findSection(r.xSpec)
		if err != nil && !errors.Is(err, errReadelfSoft) {
			return err
		}
		if s != nil {
			if err := r.hexDump(s); err != nil {
				return err
			}
		}
	}

	if r.hasP {
		s, err := r.findSection(r.pSpec)
		if err != nil && !errors.Is(err, errReadelfSoft) {
			return err
		}
		if s != nil {
			if err := r.stringDump(s); err != nil {
				return err
			}
		}
	}

	return nil
}

// loadSectionOrNil is get_sh() without the cache, for the shstrtab index
// (which may lie outside the clamp when the table is truncated).
func (r *readelfCtx) loadSectionOrNil(i int) (*readelfSection, error) {
	if i < 0 || i >= r.shnum || r.shentsize <= 0 {
		r.softf("No shdr %d", i)
		return nil, nil
	}
	if i >= r.shCount {
		r.softf("No shdr %d", i)
		return nil, nil
	}
	return r.loadSection(i)
}

func readelfSectionFlags(flags uint64, machine int) string {
	var b strings.Builder
	const letters = "WAXxMSILOGTC"
	for j := 0; j < len(letters); j++ {
		bit := uint64(1) << uint(j)
		if flags&bit != 0 {
			b.WriteByte(letters[j])
			flags &^= bit
		}
	}
	if machine == readelfEMARM || machine == readelfEMARM64 {
		if flags&(1<<29) != 0 {
			b.WriteByte('y')
			flags &^= 1 << 29
		}
	} else if machine == readelfEMX8664 {
		if flags&(1<<28) != 0 {
			b.WriteByte('l')
			flags &^= 1 << 28
		}
	}
	if flags != 0 {
		b.WriteByte('x')
	}
	return b.String()
}

func (r *readelfCtx) printHeader(ident []byte, version, flags, ehsize, phnum, shstrndx int) {
	fmt.Fprint(r.w, "ELF Header:\n")
	fmt.Fprint(r.w, "  Magic:   ")
	for i := 0; i < 16; i++ {
		if i == 15 {
			fmt.Fprintf(r.w, "%02x\n", ident[i])
		} else {
			fmt.Fprintf(r.w, "%02x ", ident[i])
		}
	}
	class := 32
	if r.bits != 0 {
		class = 64
	}
	fmt.Fprintf(r.w, "  Class:                             ELF%d\n", class)
	endianName := "little"
	if r.endian == 2 {
		endianName = "big"
	}
	fmt.Fprintf(r.w, "  Data:                              2's complement, %s endian\n", endianName)
	fmt.Fprint(r.w, "  Version:                           1 (current)\n")
	fmt.Fprintf(r.w, "  OS/ABI:                            %s\n", readelfLookup(readelfOSABI, uint64(ident[7])))
	fmt.Fprintf(r.w, "  ABI Version:                       %d\n", ident[8])
	fmt.Fprintf(r.w, "  Type:                              %s\n", readelfLookup(readelfETTypes, uint64(r.etype)))
	fmt.Fprintf(r.w, "  Machine:                           %s\n", readelfArchName(r.machine))
	fmt.Fprintf(r.w, "  Version:                           0x%x\n", version)
	fmt.Fprintf(r.w, "  Entry point address:               0x%x\n", r.entry)
	fmt.Fprintf(r.w, "  Start of program headers:          %d (bytes into file)\n", r.phoff)
	fmt.Fprintf(r.w, "  Start of section headers:          %d (bytes into file)\n", r.shoff)
	fmt.Fprintf(r.w, "  Flags:                             0x%x", flags)
	fmt.Fprint(r.w, readelfPrintFlags(r.machine, flags))
	fmt.Fprint(r.w, "\n")
	fmt.Fprintf(r.w, "  Size of this header:               %d (bytes)\n", ehsize)
	fmt.Fprintf(r.w, "  Size of program headers:           %d (bytes)\n", r.phentsize)
	fmt.Fprintf(r.w, "  Number of program headers:         %d\n", phnum)
	fmt.Fprintf(r.w, "  Size of section headers:           %d (bytes)\n", r.shentsize)
	fmt.Fprintf(r.w, "  Number of section headers:         %d\n", r.shnum)
	fmt.Fprintf(r.w, "  Section header string table index: %d\n", shstrndx)
}

func (r *readelfCtx) printProgramHeaders(w int, phnum int) error {
	fmt.Fprint(r.w, "\n")
	if phnum == 0 {
		fmt.Fprint(r.w, "There are no program headers in this file.\n")
		return nil
	}
	if !r.doH {
		fmt.Fprintf(r.w, "Elf file type is %s\nEntry point %s\n"+
			"There are %d program headers, starting at offset %d\n\n",
			readelfLookup(readelfETTypes, uint64(r.etype)), readelfAltHex(r.entry), phnum, r.phoff)
	}
	fmt.Fprintf(r.w, "Program Headers:\n  %-14s %-8s %-*s   %-*s   %-7s %-7s Flg Align\n",
		"Type", "Offset", w, "VirtAddr", w, "PhysAddr", "FileSiz", "MemSiz")
	for i := 0; i < phnum; i++ {
		ph, err := r.programHeader(i)
		if err != nil {
			if errors.Is(err, errReadelfSoft) {
				continue
			}
			return err
		}
		fmt.Fprintf(r.w, "  %-14s 0x%06x 0x%0*x 0x%0*x 0x%05x 0x%05x %c%c%c %s\n",
			readelfLookup(readelfPHTypes, uint64(ph.typ)), ph.offset,
			w, ph.vaddr, w, ph.paddr, ph.filesz, ph.memsz,
			readelfFlagChar(ph.flags&4 != 0, 'R'), readelfFlagChar(ph.flags&2 != 0, 'W'),
			readelfFlagChar(ph.flags&1 != 0, 'E'), readelfAltHex(ph.align))
		if ph.typ == readelfPTInterp && ph.filesz < r.size &&
			ph.offset < r.size && ph.filesz-1 < r.size-ph.offset {
			s := r.cstring(ph.offset)
			pad := int(ph.filesz) - 1 - len(s)
			if pad < 0 {
				pad = 0
			}
			fmt.Fprintf(r.w, "      [Requesting program interpreter: %s%s]\n", strings.Repeat(" ", pad), s)
		}
	}

	fmt.Fprint(r.w, "\n Section to Segment mapping:\n  Segment Sections...\n")
	for i := 0; i < phnum; i++ {
		ph, err := r.programHeader(i)
		if err != nil {
			if errors.Is(err, errReadelfSoft) {
				continue
			}
			return err
		}
		fmt.Fprintf(r.w, "   %02d     ", i)
		for j := 0; j < r.shCount; j++ {
			s, err := r.section(j)
			if err != nil {
				if errors.Is(err, errReadelfSoft) {
					continue
				}
				return err
			}
			if s.name == "" {
				continue
			}
			if s.offset >= ph.offset && s.offset+s.size <= ph.offset+ph.filesz {
				fmt.Fprintf(r.w, "%s ", s.name)
			}
		}
		fmt.Fprint(r.w, "\n")
	}
	return nil
}

// readelfAltHex is C's %#x: no "0x" prefix for zero.
func readelfAltHex(v uint64) string {
	if v == 0 {
		return "0"
	}
	return "0x" + strconv.FormatUint(v, 16)
}

func readelfFlagChar(set bool, c byte) byte {
	if set {
		return c
	}
	return ' '
}

func (r *readelfCtx) printDynamic(w int, dynamic, dynstr *readelfSection) error {
	dyn := dynamic.offset
	end := dynamic.offset + dynamic.size

	fmt.Fprint(r.w, "\n")
	if dynamic.size == 0 {
		fmt.Fprint(r.w, "There is no dynamic section in this file.\n")
		return nil
	}
	if dynamic.entsize == 0 {
		fmt.Fprint(r.w, "Bad dynamic entry size 0!\n")
		return nil
	}
	fmt.Fprintf(r.w, "Dynamic section at offset 0x%x contains %d entries:\n"+
		"  %-*s %-20s %s\n", dynamic.offset, dynamic.size/dynamic.entsize,
		w+2, "Tag", "Type", "Name/Value")

	c := r.cursor(dyn)
	for c.off < end {
		tag := c.word()
		val := c.word()
		if c.err != nil {
			return c.err
		}
		typeName := readelfLookup(readelfDTTypes, tag)
		disp := typeName
		if typeName[0] != '0' {
			disp = typeName[1:]
		}
		fmt.Fprintf(r.w, " 0x%0*x %-20s ", w, tag, disp)
		switch typeName[0] {
		case 'd':
			fmt.Fprintf(r.w, "%d\n", int64(val))
		case 'b':
			fmt.Fprintf(r.w, "%d (bytes)\n", int64(val))
		case 's':
			fmt.Fprintf(r.w, "%s\n", r.cstring(dynstr.offset+val))
		case 'f', 'F':
			r.printDynamicFlags(w, typeName[0] == 'F', val)
		case 'N', 'R', 'S':
			s := "???"
			if dynstr.offset <= r.size && val <= r.size && dynstr.offset <= r.size-val {
				s = r.cstring(dynstr.offset + val)
			}
			name := "Library soname"
			if typeName[0] == 'N' {
				name = "Shared library"
			} else if typeName[0] == 'R' {
				name = "Library runpath"
			}
			fmt.Fprintf(r.w, "%s: [%s]\n", name, s)
		case 'P':
			s := readelfLookup(readelfDTTypes, val)
			n := len(s)
			if s[0] != '0' {
				s = s[2:]
				n -= 3
			}
			if n < 0 {
				n = 0
			}
			fmt.Fprintf(r.w, "%*.*s\n", n, n, s)
		default:
			fmt.Fprintf(r.w, "0x%x\n", val)
		}
	}
	return nil
}

var readelfDFNames = []readelfName{
	{0, "ORIGIN"}, {1, "SYMBOLIC"}, {2, "TEXTREL"}, {3, "BIND_NOW"}, {4, "STATIC_TLS"},
}

var readelfDF1Names = []readelfName{
	{0, "NOW"}, {1, "GLOBAL"}, {2, "GROUP"}, {3, "NODELETE"},
	{5, "INITFIRST"}, {27, "PIE"},
}

func (r *readelfCtx) printDynamicFlags(w int, isFlags1 bool, val uint64) {
	names := readelfDFNames
	if isFlags1 {
		names = readelfDF1Names
		fmt.Fprint(r.w, "Flags: ")
	}
	for _, e := range names {
		mask := uint64(1) << e.v
		if val&mask != 0 {
			val &^= mask
			fmt.Fprint(r.w, e.s)
			if val != 0 {
				fmt.Fprint(r.w, " ")
			}
		}
	}
	if val != 0 {
		fmt.Fprintf(r.w, "0x%x", val)
	}
	fmt.Fprint(r.w, "\n")
}

func (r *readelfCtx) showSymbols(table, strtab *readelfSection) error {
	entrySize := uint64(16)
	if r.bits != 0 {
		entrySize = 24
	}
	numsym := table.size / entrySize
	if numsym == 0 {
		return nil
	}

	fmt.Fprintf(r.w, "\nSymbol table '%s' contains %d entries:\n"+
		"   Num:    %*s  Size Type    Bind   Vis      Ndx Name\n",
		table.name, numsym, 5+8*r.bits, "Value")

	c := r.cursor(table.offset)
	for i := uint64(0); i < numsym; i++ {
		var stName, stValue, stSize, stShndx, stInfo, stOther uint64

		// The various fields were moved around for 64-bit.
		if r.bits != 0 {
			stName = uint64(c.u32())
			stInfo = uint64(c.u8())
			stOther = uint64(c.u8())
			stShndx = uint64(c.u16())
			stValue = c.u64()
			stSize = c.u64()
		} else {
			stName = uint64(c.u32())
			stValue = uint64(c.u32())
			stSize = uint64(c.u32())
			stInfo = uint64(c.u8())
			stOther = uint64(c.u8())
			stShndx = uint64(c.u16())
		}
		if c.err != nil {
			return c.err
		}

		name := "???"
		if strtab.offset < r.size && stName < r.size-strtab.offset {
			name = r.cstring(strtab.offset + stName)
		}

		ndx := "UND"
		if stShndx == 0xfff1 {
			ndx = "ABS"
		} else if stShndx != 0 {
			ndx = strconv.FormatUint(stShndx, 10)
		}

		fmt.Fprintf(r.w, "%6d: %0*x %5d %-7s %-6s %-9s%3s %s\n",
			i, 8*(r.bits+1), stValue, stSize,
			readelfLookup(readelfSTTTypes, stInfo&0xf),
			readelfLookup(readelfSTBTypes, stInfo>>4),
			readelfLookup(readelfSTVTypes, stOther&3), ndx, name)
	}
	return nil
}

func readelfNoteMatch(namesz uint64, raw []byte, expected string) bool {
	if namesz != uint64(len(expected))+1 {
		return false
	}
	if uint64(len(raw)) < namesz {
		return false
	}
	return string(raw[:namesz-1]) == expected && raw[namesz-1] == 0
}

func (r *readelfCtx) uleb(c *readelfCursor, end uint64) uint64 {
	var result uint64
	shift := uint(0)
	for {
		if c.err != nil {
			return 0
		}
		if c.off >= end {
			c.err = errors.New("EOF in uleb128")
			return 0
		}
		b, err := r.bytes(c.off, 1)
		if err != nil {
			c.err = err
			return 0
		}
		c.off++
		result |= uint64(b[0]&0x7f) << shift
		shift += 7
		if shift > 56 {
			c.err = errors.New("uleb128 too long")
			return 0
		}
		if b[0]&0x80 == 0 {
			return result
		}
	}
}

func (r *readelfCtx) showAttributes(off, size uint64) error {
	end := off + size

	// Attributes sections start with an 'A'...
	b, err := r.bytes(off, 1)
	if err != nil {
		return err
	}
	if off == size || b[0] != 'A' {
		return r.errorf("bad attributes @%d", off)
	}

	c := r.cursor(off + 1)
	// A subsection starts with a uint32 length and ASCII vendor name.
	_ = c.u32()
	if c.err != nil {
		return c.err
	}
	vendorOff := c.off
	probe := r.slice(vendorOff, 32)
	if bytes.IndexByte(probe, 0) < 0 {
		return r.errorf("bad vendor name")
	}
	vendor := readelfCStringBytes(probe)
	fmt.Fprintf(r.w, "\nAttribute Section: %s\n", vendor)
	c.off = vendorOff + uint64(len(vendor)) + 1

	// ...followed by one or more sub-subsections.
	tag := r.uleb(c, end)
	if c.err != nil {
		return c.err
	}
	length := c.u32()
	if c.err != nil {
		return c.err
	}
	if tag == 1 {
		fmt.Fprint(r.w, "File Attributes\n")
		for c.off < end {
			tag = r.uleb(c, end)
			if c.err != nil {
				return c.err
			}
			fmt.Fprintf(r.w, "  Tag_RISCV_%s: ", readelfLookup(readelfRISCVAttrTags, tag))
			// Even riscv tags have uleb128 values, odd ones strings.
			if tag&1 == 0 {
				v := r.uleb(c, end)
				if c.err != nil {
					return c.err
				}
				fmt.Fprintf(r.w, "%d\n", int64(v))
			} else {
				s := r.cstring(c.off)
				fmt.Fprintf(r.w, "%s\n", s)
				c.off += uint64(len(s)) + 1
			}
		}
	} else {
		return r.errorf("unknown attributes tag=%x (size=%d)\n", tag, length)
	}
	return nil
}

func (r *readelfCtx) showNotes(off, size uint64) error {
	if !r.fits("note", -1, off, size) {
		return errReadelfSoft
	}

	fmt.Fprintf(r.w, "  %-20s%11s\tDescription\n", "Owner", "Data size")
	end := off + size
	for note := off; note < end; {
		c := r.cursor(note)
		namesz := uint64(c.u32())
		descsz := uint64(c.u32())
		ntype := uint64(c.u32())
		if c.err != nil {
			return c.err
		}
		if namesz > size || descsz > size {
			return r.errorf("bad note @%d", off)
		}

		nameOff := note + 12
		raw, err := r.bytes(nameOff, namesz)
		if err != nil {
			return err
		}
		fmt.Fprintf(r.w, "  %-20s 0x%08x\t", readelfCStringBytes(raw), descsz)

		descOff := nameOff + namesz
		pOff := nameOff
		j := uint64(0)

		switch {
		case readelfNoteMatch(namesz, raw, "GNU"):
			pOff = descOff
			switch ntype {
			case 1: // NT_GNU_ABI_TAG
				ac := r.cursor(descOff)
				osid := ac.u32()
				v1 := ac.u32()
				v2 := ac.u32()
				v3 := ac.u32()
				if ac.err != nil {
					return ac.err
				}
				osname := "?"
				if osid == 0 {
					osname = "Linux"
				}
				fmt.Fprintf(r.w, "NT_GNU_ABI_TAG\tOS: %s, ABI: %d.%d.%d", osname, v1, v2, v3)
				j = 1
				pOff = ac.off
			case 3: // NT_GNU_BUILD_ID
				fmt.Fprint(r.w, "NT_GNU_BUILD_ID\t")
				buf := r.slice(descOff, descsz)
				for _, x := range buf {
					fmt.Fprintf(r.w, "%02x", x)
				}
				j = uint64(len(buf))
				pOff = descOff + j
			case 4: // NT_GNU_GOLD_VERSION
				buf := r.slice(descOff, descsz)
				fmt.Fprintf(r.w, "NT_GNU_GOLD_VERSION\t%s", readelfCStringBytes(buf))
				j = 1
				pOff = descOff
			case 5: // NT_GNU_PROPERTY_TYPE_0
				fmt.Fprint(r.w, "NT_GNU_PROPERTY_TYPE_0\n    Properties:")
				pc := r.cursor(descOff)
				for int64(descsz)-int64(j) > 0 {
					prType := pc.u32()
					prSize := pc.u32()
					if pc.err != nil {
						return pc.err
					}
					j += 8
					if pc.off > end {
						return r.errorf("bad property @%d", off)
					}
					if prSize != 4 {
						// Just hex dump anything we aren't familiar with.
						buf := r.slice(pc.off, uint64(prSize))
						for _, x := range buf {
							fmt.Fprintf(r.w, "%02x", x)
						}
						fmt.Fprint(r.w, "\n")
						j += uint64(prSize)
						pc.off += uint64(prSize)
					} else {
						prData := pc.u32()
						_ = pc.u32() // Skip padding.
						if pc.err != nil {
							return pc.err
						}
						j += 8
						switch prType {
						case 0xc0000000:
							fmt.Fprint(r.w, "\tarm64 features:")
							if prData&1 != 0 {
								fmt.Fprint(r.w, " bti")
							}
							if prData&2 != 0 {
								fmt.Fprint(r.w, " pac")
							}
							fmt.Fprint(r.w, "\n")
						case 0xc0000002:
							fmt.Fprint(r.w, "\tx86 feature:")
							if prData&1 != 0 {
								fmt.Fprint(r.w, " ibt")
							}
							if prData&2 != 0 {
								fmt.Fprint(r.w, " shstk")
							}
							fmt.Fprint(r.w, "\n")
						case 0xc0008002:
							fmt.Fprintf(r.w, "\tx86 isa needed: x86-64v%d", readelfFFS(prData))
						default:
							fmt.Fprintf(r.w, "\tother (%s): %s", readelfAltHex(uint64(prType)), readelfAltHex(uint64(prData)))
						}
					}
				}
				pOff = pc.off
			default:
				// Unknown GNU note type: the hex dump starts 4 bytes early.
				pOff = descOff - 4
			}
		case readelfNoteMatch(namesz, raw, "Android"):
			pOff = descOff
			switch ntype {
			case 1: // NT_VERSION
				ac := r.cursor(descOff)
				api := ac.u32()
				if ac.err != nil {
					return ac.err
				}
				fmt.Fprintf(r.w, "NT_VERSION\tAPI level %d", api)
				j = 1
				pOff = ac.off
				if descsz >= 132 {
					ndk := readelfCStringBytes(r.slice(pOff, 64))
					build := readelfCStringBytes(r.slice(pOff+64, 64))
					fmt.Fprintf(r.w, ", NDK %s (%s)", ndk, build)
				}
			case 5: // NT_PAD_SEGMENT
				ac := r.cursor(descOff)
				pad := ac.u32()
				if ac.err != nil {
					return ac.err
				}
				fmt.Fprintf(r.w, "NT_PAD_SEGMENT\tpad_segment=%d", pad)
				j = 1
				pOff = ac.off
			default:
				pOff = descOff - 8
			}
		case readelfNoteMatch(namesz, raw, "CORE"):
			pOff = descOff
			if desc := readelfLookup(readelfNTTypeCore, ntype); desc[0] != '0' {
				fmt.Fprint(r.w, desc)
				j = 1
			}
		case readelfNoteMatch(namesz, raw, "LINUX"):
			pOff = descOff
			if desc := readelfLookup(readelfNTTypeLinux, ntype); desc[0] != '0' {
				fmt.Fprint(r.w, desc)
				j = 1
			}
		default:
			pOff = nameOff
		}

		// If we didn't do custom output above, show a hex dump.
		if j == 0 {
			fmt.Fprintf(r.w, "0x%x\t", ntype)
			buf := r.slice(pOff, descsz)
			for k, x := range buf {
				if k != 0 {
					fmt.Fprint(r.w, " ")
				} else {
					fmt.Fprint(r.w, "\t")
				}
				fmt.Fprintf(r.w, "%02x", x)
			}
		}
		fmt.Fprint(r.w, "\n")

		note += 3*4 + ((namesz + 3) &^ 3) + ((descsz + 3) &^ 3)
	}
	return nil
}

// readelfFFS is ffs(3): the 1-based index of the lowest set bit, 0 if none.
func readelfFFS(v uint32) int {
	for i := 0; i < 32; i++ {
		if v&(1<<uint(i)) != 0 {
			return i + 1
		}
	}
	return 0
}

func (r *readelfCtx) hexDump(s *readelfSection) error {
	fmt.Fprintf(r.w, "\nHex dump of section '%s':\n", s.name)
	var offset uint64
	for offset < s.size {
		row := s.size - offset
		if row > 16 {
			row = 16
		}
		buf, err := r.bytes(s.offset+offset, row)
		if err != nil {
			return err
		}
		fmt.Fprintf(r.w, "  0x%08x ", offset)
		space := 2*16 + 16/4
		for i, b := range buf {
			fmt.Fprintf(r.w, "%02x", b)
			space -= 2
			if (i+1)%4 == 0 {
				fmt.Fprint(r.w, " ")
				space--
			}
		}
		if space > 0 {
			fmt.Fprint(r.w, strings.Repeat(" ", space))
		}
		for _, b := range buf {
			if b >= ' ' && b <= '~' {
				fmt.Fprintf(r.w, "%c", b)
			} else {
				fmt.Fprint(r.w, ".")
			}
		}
		fmt.Fprint(r.w, "\n")
		offset += row
	}
	fmt.Fprint(r.w, "\n")
	return nil
}

func (r *readelfCtx) stringDump(s *readelfSection) error {
	fmt.Fprintf(r.w, "\nString dump of section '%s':\n", s.name)

	const chunk = 1 << 16
	var run []byte
	var runStart uint64
	any := false
	flush := func() {
		if len(run) == 0 {
			return
		}
		fmt.Fprintf(r.w, "  [%6x]  ", runStart)
		r.w.Write(run)
		fmt.Fprint(r.w, "\n")
		any = true
		run = run[:0]
	}

	for pos := uint64(0); pos < s.size; {
		n := s.size - pos
		if n > chunk {
			n = chunk
		}
		buf, err := r.bytes(s.offset+pos, n)
		if err != nil {
			return err
		}
		if len(buf) == 0 {
			break
		}
		for i, b := range buf {
			if b >= ' ' && b <= '~' {
				if len(run) == 0 {
					runStart = pos + uint64(i)
				}
				run = append(run, b)
			} else {
				flush()
			}
		}
		pos += uint64(len(buf))
		if uint64(len(buf)) < n {
			break
		}
	}
	flush()

	if !any {
		fmt.Fprint(r.w, "  No strings found in this section.\n")
	}
	fmt.Fprint(r.w, "\n")
	return nil
}
