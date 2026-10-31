// file.go - describe file type
//
// Ported from toybox toys/posix/file.c (with lib/elf.c arch tables and
// lib/lib.c is_tar_header), commit 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb.
// Toybox is 0BSD licensed; see docs/CREDITS.md and THIRD_PARTY_NOTICES.md.
// Writes were originally Copyright 2016 The Android Open Source Project; the Go
// port below is original Go code written against the same behaviour.
//
// Deliberate differences from the C:
//
//   - ELF metadata is read with pread (fileSource) instead of mmap. Every read
//     is checked against the real file size and against a total budget
//     (fileELFBudget), and a refused read prints the toybox "bad ...?" note, so
//     a truncated or corrupt header cannot read or allocate outside the file.
//   - PT_INTERP strings and ELF note payloads are capped at fileELFMaxItem
//     bytes; the C prints however many bytes the header claims.
//   - "-" (stdin) uses the number of bytes actually available as the file size;
//     the C uses the stat size, which is 0 for a pipe, so its ELF path on stdin
//     fails with "mmap: Invalid argument".
//   - A zero-length regular file is the only case that short-circuits to
//     "empty"; the C tests st_size before opening, so "-s /dev/null" and
//     "-s /dev/zero" print "empty" and "unknown". Here -s opens the device and
//     describes what it reads, as the usage text promises.
//   - The printable test in the text branch follows Go's unicode tables rather
//     than the host glibc's locale tables (which are what toybox queries): the
//     verdict can differ for a code point assigned after the host's Unicode
//     version, and for a locale whose printable set is not UTF-8 wide.
//   - The binary cpio branch reads the magic from the file bytes, which is what
//     the C means by it. The C asks peek(&s, 2), i.e. about two bytes of the
//     local pointer variable rather than the file, so its branch never fires and
//     toybox describes such a file as "data"; this port detects the archive the
//     comment and the 0143561/070707 magics describe.
//   - Exit status is 1 whenever an operand was not described, including a
//     failed stat; the C leaves it at 0 when the stat itself fails.
package builtins

import (
	"fmt"
	"io"
	"os"
	"syscall"
	"unicode"
	"unicode/utf8"
)

const fileSpec = "b(brief)hLs"

var fileSpecParsed *Spec

func init() {
	var err error
	fileSpecParsed, err = ParseSpec(fileSpec)
	if err != nil {
		panic("builtins: bad file spec: " + err.Error())
	}
	Register(Command{Name: "file", Run: runFile})
}

const (
	// filePrefixRead is how much of a file the C reads into toybuf: it reads
	// sizeof(toybuf)-8 bytes of its 4096-byte buffer.
	filePrefixRead = 4088
	// fileToyBufSize is the size of the C's toybuf. Reads past the bytes
	// actually read see zeros, like the C's memset(toybuf, 0, 80) does for the
	// header fields it pokes at.
	fileToyBufSize = 4096
	// fileELFBudget caps the total amount of ELF metadata (program headers,
	// section headers and note payloads) read from one file.
	fileELFBudget = 8 << 20
	// fileELFMaxItem caps a single ELF string or note payload.
	fileELFMaxItem = 4096
)

func runFile(ctx *Context) int {
	// The standalone binary can dispatch a builtin without filling in CmdName;
	// fall back to our own name so --help/--version still name the command.
	if ctx.CmdName == "" {
		local := *ctx
		local.CmdName = "file"
		ctx = &local
	}

	flags, err := fileSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "file:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, fileSpecParsed) {
		return 0
	}

	brief := flags.Bool("b")
	noFollow := flags.Bool("h")
	follow := flags.Bool("L")
	showDev := flags.Bool("s")

	if noFollow && follow {
		fmt.Fprintln(ctx.Stderr, "file: -h and -L cannot be used together")
		return 1
	}

	names := flags.Operands
	if len(names) == 0 {
		fmt.Fprintln(ctx.Stderr, "file: missing operand")
		return 1
	}

	maxName := 0
	for _, name := range names {
		if len(name) > maxName {
			maxName = len(name)
		}
	}

	status := 0
	for _, name := range names {
		if !brief {
			fmt.Fprintf(ctx.Stdout, "%s: %*s", name, maxName-len(name), "")
		}
		if !fileDescribe(ctx, name, follow, showDev) {
			status = 1
		}
	}
	return status
}

// fileErrText is strerror(errno) with glibc's leading capital, derived from the
// errno itself so no host locale or message text leaks in.
func fileErrText(err error) string {
	var errno syscall.Errno
	switch e := err.(type) {
	case *os.PathError:
		errno, _ = e.Err.(syscall.Errno)
	case *os.LinkError:
		errno, _ = e.Err.(syscall.Errno)
	case *os.SyscallError:
		errno, _ = e.Err.(syscall.Errno)
	case syscall.Errno:
		errno = e
	}
	if errno == 0 {
		return err.Error()
	}
	msg := errno.Error()
	if len(msg) > 0 && msg[0] >= 'a' && msg[0] <= 'z' {
		msg = string(msg[0]-32) + msg[1:]
	}
	return msg
}

// fileDeviceNumbers splits a Linux dev_t. The encoding is the same on the BSDs
// for the low 12 bits of the major and the low 8 bits of the minor, which is all
// any real device uses.
func fileDeviceNumbers(fi os.FileInfo) (uint64, uint64) {
	var rdev uint64
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		rdev = uint64(st.Rdev)
	}
	major := (rdev >> 8) & 0xfff
	major |= (rdev >> 32) &^ 0xfff
	minor := rdev & 0xff
	minor |= (rdev >> 12) &^ 0xff
	return major, minor
}

func fileDescribe(ctx *Context, name string, follow, showDev bool) bool {
	if name == "-" {
		return fileDescribeStdin(ctx)
	}

	path := resolvePath(ctx, name)
	var fi os.FileInfo
	var err error
	if follow {
		fi, err = os.Stat(path)
	} else {
		fi, err = os.Lstat(path)
	}
	if err != nil {
		fmt.Fprintf(ctx.Stdout, "cannot open: %s\n", fileErrText(err))
		return false
	}

	mode := fi.Mode()

	if !showDev && mode&os.ModeDevice != 0 {
		kind := "block"
		if mode&os.ModeCharDevice != 0 {
			kind = "character"
		}
		major, minor := fileDeviceNumbers(fi)
		fmt.Fprintf(ctx.Stdout, "%s special (%d/%d)\n", kind, major, minor)
		return true
	}

	if mode.IsRegular() || (showDev && mode&os.ModeDevice != 0) {
		// The C uses this to identify an empty file it may not be allowed to
		// read, so it happens before the open.
		if mode.IsRegular() && fi.Size() == 0 {
			fmt.Fprintln(ctx.Stdout, "empty")
			return true
		}
		f, err := os.Open(path)
		if err != nil {
			// openro(): a failed open warns on stderr and the C then falls
			// through to xputs("unknown").
			fmt.Fprintf(ctx.Stderr, "file: %s: %s\n", name, fileErrText(err))
			fmt.Fprintln(ctx.Stdout, "unknown")
			return false
		}
		defer f.Close()
		src := &fileSource{f: f, size: uint64(fi.Size()), budget: fileELFBudget}
		return fileDescribeRegular(ctx, src, name)
	}

	switch {
	case mode&os.ModeNamedPipe != 0:
		fmt.Fprintln(ctx.Stdout, "fifo")
	case mode.IsDir():
		fmt.Fprintln(ctx.Stdout, "directory")
	case mode&os.ModeSocket != 0:
		fmt.Fprintln(ctx.Stdout, "socket")
	case mode&os.ModeSymlink != 0:
		target, err := os.Readlink(path)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "file: %s: %s\n", name, fileErrText(err))
			fmt.Fprintln(ctx.Stdout, "unknown")
			return false
		}
		broken := ""
		if _, err := os.Stat(path); err != nil {
			broken = "broken "
		}
		fmt.Fprintf(ctx.Stdout, "%ssymbolic link to %s\n", broken, target)
	default:
		fmt.Fprintln(ctx.Stdout, "unknown")
	}
	return true
}

func fileDescribeStdin(ctx *Context) bool {
	if ctx.Stdin == nil {
		fmt.Fprintln(ctx.Stderr, "file: -: Bad file descriptor")
		fmt.Fprintln(ctx.Stdout, "unknown")
		return false
	}

	buf := make([]byte, fileToyBufSize)
	n, err := io.ReadFull(ctx.Stdin, buf[:filePrefixRead])
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		// readall() failure: warn and describe what did arrive.
		fmt.Fprintf(ctx.Stderr, "file: -: %s\n", fileErrText(err))
	}

	if n >= 45 && fileHasPrefix(buf, 0, "\x7fELF") {
		// Random access to the rest of the stream: a pipe has no size, so the
		// bytes that are actually there are the file size.
		rest, _ := io.ReadAll(io.LimitReader(ctx.Stdin, fileELFBudget))
		data := make([]byte, 0, n+len(rest))
		data = append(data, buf[:n]...)
		data = append(data, rest...)
		src := &fileSource{data: data, size: uint64(len(data)), budget: fileELFBudget}
		return fileDescribePrefix(ctx, src, buf, n)
	}

	src := &fileSource{size: uint64(n), budget: fileELFBudget}
	return fileDescribePrefix(ctx, src, buf, n)
}

// fileSource gives the ELF reader bounded random access to a file: a real file
// through pread, or the bytes already read from stdin.
type fileSource struct {
	f      *os.File
	data   []byte
	size   uint64
	budget uint64
}

// readAt returns exactly n bytes at off, refusing anything outside the file or
// beyond the remaining metadata budget.
func (s *fileSource) readAt(off, n uint64) ([]byte, bool) {
	if n == 0 {
		return nil, true
	}
	if off > s.size || n > s.size-off {
		return nil, false
	}
	if n > s.budget {
		return nil, false
	}
	s.budget -= n
	if s.f == nil {
		if off+n > uint64(len(s.data)) {
			return nil, false
		}
		return s.data[off : off+n], true
	}
	b := make([]byte, n)
	if _, err := s.f.ReadAt(b, int64(off)); err != nil {
		return nil, false
	}
	return b, true
}

// fileDescribeRegular reads the prefix of the file and describes it.
func fileDescribeRegular(ctx *Context, src *fileSource, name string) bool {
	buf := make([]byte, fileToyBufSize)
	n := 0
	if src.f != nil {
		got, err := io.ReadFull(src.f, buf[:filePrefixRead])
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			// readall() failure: warn and describe what did arrive.
			fmt.Fprintf(ctx.Stderr, "file: %s: %s\n", name, fileErrText(err))
		}
		n = got
	} else {
		n = len(src.data)
		if n > filePrefixRead {
			n = filePrefixRead
		}
		copy(buf, src.data[:n])
	}
	return fileDescribePrefix(ctx, src, buf, n)
}

// magicBe and magicLe read an n-byte integer at off, treating bytes past the end
// of b as zero (the C reads them out of its zeroed toybuf).
func magicBe(b []byte, off, n int) uint64 {
	var v uint64
	for i := 0; i < n; i++ {
		v <<= 8
		if off+i >= 0 && off+i < len(b) {
			v |= uint64(b[off+i])
		}
	}
	return v
}

func magicLe(b []byte, off, n int) uint64 {
	var v uint64
	for i := n - 1; i >= 0; i-- {
		v <<= 8
		if off+i >= 0 && off+i < len(b) {
			v |= uint64(b[off+i])
		}
	}
	return v
}

// fileHasPrefix is memcmp(b[off:], lit, len(lit)) == 0.
func fileHasPrefix(b []byte, off int, lit string) bool {
	if off < 0 || off+len(lit) > len(b) {
		return false
	}
	for i := 0; i < len(lit); i++ {
		if b[off+i] != lit[i] {
			return false
		}
	}
	return true
}

// fileMatch is toybox's strstart(): does b[*s:] start with lit, and if so
// advance s past it.
func fileMatch(b []byte, s *int, lit string) bool {
	off := *s
	if off < 0 || off+len(lit) > len(b) {
		return false
	}
	for i := 0; i < len(lit); i++ {
		if b[off+i] != lit[i] {
			return false
		}
	}
	*s = off + len(lit)
	return true
}

func fileIsSpace(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	}
	return false
}

// fileWcWidth reports -1 for a code point that is not printable, which is what
// glibc's wcwidth() says about everything outside its locale's printable set:
// controls, C1, surrogates, and code points Unicode leaves unassigned. The width
// itself (0 or 1) is never used, only the sign, so combining marks need no
// special case. Private use and format characters are printable to glibc, so
// they are here too. toybox's utf8towc() returns 0 for a NUL byte, so wcwidth()
// is never asked about one.
func fileWcWidth(r rune) int {
	if r == 0 || r < 0x20 || (r >= 0x7f && r < 0xa0) {
		return -1
	}
	if r >= 0xd800 && r <= 0xdfff {
		return -1
	}
	if unicode.IsGraphic(r) || unicode.In(r, unicode.Co, unicode.Cf) {
		return 1
	}
	return -1
}

// filePDFVersion parses "%PDF-<int>.<uint>" the way sscanf("%%PDF%d.%u") does.
func filePDFVersion(b []byte) (int, uint32, bool) {
	if len(b) < 5 || b[0] != '%' || !fileHasPrefix(b, 1, "PDF") {
		return 0, 0, false
	}
	i := 4
	neg := false
	if i < len(b) && (b[i] == '+' || b[i] == '-') {
		neg = b[i] == '-'
		i++
	}
	start := i
	for i < len(b) && b[i] >= '0' && b[i] <= '9' {
		i++
	}
	if i == start {
		return 0, 0, false
	}
	num := 0
	for _, c := range b[start:i] {
		num = num*10 + int(c-'0')
		if num > 1<<20 {
			num = 1 << 20
		}
	}
	if neg {
		num = -num
	}
	if i >= len(b) || b[i] != '.' {
		return 0, 0, false
	}
	i++
	if i < len(b) && (b[i] == '+' || b[i] == '-') {
		i++
	}
	start = i
	for i < len(b) && b[i] >= '0' && b[i] <= '9' {
		i++
	}
	if i == start {
		return 0, 0, false
	}
	var magic uint32
	for _, c := range b[start:i] {
		magic = magic*10 + uint32(c-'0')
	}
	return num, magic, true
}

// fileTarCksum is tar_cksum() from lib/lib.c: the checksum field (eight spaces)
// and the byte before it are left out.
func fileTarCksum(h []byte) uint32 {
	cksum := uint32(8 * ' ')
	for i := 0; i < 500; {
		cksum += uint32(h[i])
		if i == 147 {
			i += 9
		} else {
			i++
		}
	}
	return cksum
}

// fileParseOctal8 is sscanf(s, "%8o", &i).
func fileParseOctal8(s []byte) int {
	i := 0
	for i < len(s) && fileIsSpace(s[i]) {
		i++
	}
	neg := false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		neg = s[i] == '-'
		i++
	}
	v, digits := 0, 0
	for i < len(s) && digits < 8 && s[i] >= '0' && s[i] <= '7' {
		v = v*8 + int(s[i]-'0')
		i++
		digits++
	}
	if digits == 0 {
		return 0
	}
	if neg {
		return -v
	}
	return v
}

// fileIsTarHeader is is_tar_header() from lib/lib.c.
func fileIsTarHeader(h []byte) bool {
	if len(h) < 512 {
		return false
	}
	if h[257] != 0 && !fileHasPrefix(h, 257, "ustar") {
		return false
	}
	if h[148] != '0' && h[148] != ' ' {
		return false
	}
	sum := fileParseOctal8(h[148:])
	return sum != 0 && fileTarCksum(h) == uint32(sum)
}

// fileArchNames is elf_arch_name()'s table from lib/elf.c.
var fileArchNames = []struct {
	val  uint64
	name string
}{
	{0x9026, "alpha"}, {93, "arc"}, {195, "arcv2"}, {40, "arm"},
	{183, "arm64"}, {0x18ad, "avr32"}, {247, "bpf"}, {106, "blackfin"},
	{140, "c6x"}, {23, "cell"}, {76, "cris"}, {252, "csky"}, {0x5441, "frv"},
	{46, "h8300"}, {164, "hexagon"}, {50, "ia64"}, {258, "loongarch"},
	{88, "m32r"}, {0x9041, "m32r"}, {4, "m68k"}, {174, "metag"},
	{189, "microblaze"}, {0xbaab, "microblaze-old"}, {8, "mips"},
	{10, "mips-old"}, {89, "mn10300"}, {0xbeef, "mn10300-old"}, {113, "nios2"},
	{92, "openrisc"}, {0x8472, "openrisc-old"}, {15, "parisc"}, {20, "ppc"},
	{21, "ppc64"}, {243, "riscv"}, {22, "s390"}, {0xa390, "s390-old"},
	{135, "score"}, {42, "sh"}, {2, "sparc"}, {18, "sparc8+"}, {43, "sparc9"},
	{188, "tile"}, {191, "tilegx"}, {3, "386"}, {6, "486"}, {62, "x86-64"},
	{94, "xtensa"}, {0xabc7, "xtensa-old"},
}

func fileArchName(t uint64) string {
	for _, e := range fileArchNames {
		if e.val == t {
			return e.name
		}
	}
	return fmt.Sprintf("unknown arch %d", t)
}

// filePrintFlags is elf_print_flags() from lib/elf.c.
func filePrintFlags(w io.Writer, arch, flags uint64) {
	switch arch {
	case 40: // arm
		fmt.Fprintf(w, ", EABI%d", (flags>>24)&0xf)
		if flags&0x200 != 0 {
			fmt.Fprint(w, ", soft float")
		} else if flags&0x400 != 0 {
			fmt.Fprint(w, ", hard float")
		}
	case 243: // riscv
		if flags&1 != 0 {
			fmt.Fprint(w, ", C")
		}
		if flags&8 != 0 {
			fmt.Fprint(w, ", E")
		}
		if flags&0x10 != 0 {
			fmt.Fprint(w, ", TSO")
		}
		float := []string{"soft", "single", "double", "quad"}
		fmt.Fprintf(w, ", %s float", float[(flags&0x6)/2])
	}
}

// fileCString returns the bytes of b up to the first NUL, which is what a C
// "%s" (or "%.64s") conversion prints.
func fileCString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// fileDescribePrefix is do_regular_file() from toys/posix/file.c.
func fileDescribePrefix(ctx *Context, src *fileSource, buf []byte, n int) bool {
	w := ctx.Stdout
	s := 0

	if n == 0 {
		fmt.Fprintln(w, "empty")
		return true
	}

	// 45 bytes: https://www.muppetlabs.com/~breadbox/software/tiny/teensy.html
	if n >= 45 && fileHasPrefix(buf, 0, "\x7fELF") {
		fileDoELF(ctx, src, buf)
		return true
	}

	// These two have to be decided before the chain below, because a failed
	// match falls through to the next test in the C's else-if chain.
	pdfMajor, pdfMagic, isPDF := 0, uint32(0), false
	if buf[0] == '%' {
		pdfMajor, pdfMagic, isPDF = filePDFVersion(buf[:n])
	}
	isPE := n > 0x70 && fileHasPrefix(buf, 0, "MZ") && fileIsPE(buf, n)

	switch {
	case fileHasPrefix(buf, 0, "!<arch>\n"):
		fmt.Fprintln(w, "ar archive")

	case isPDF:
		fmt.Fprintf(w, "PDF document, version %d.%d\n", -pdfMajor, pdfMagic)

	case n > 28 && fileMatch(buf, &s, "\x89PNG\x0d\x0a\x1a\x0a"):
		// PNG is big-endian: https://www.w3.org/TR/PNG/#7Integers-and-byte-order
		chunkLength := magicBe(buf, s, 4)
		fmt.Fprint(w, "PNG image data")
		// The IHDR chunk comes first: https://www.w3.org/TR/PNG/#11IHDR
		s += 4
		if chunkLength == 13 && fileMatch(buf, &s, "IHDR") {
			// https://www.w3.org/TR/PNG/#6Colour-values
			colors := []string{"grayscale", "", "color RGB", "indexed color",
				"grayscale with alpha", "", "color RGBA"}
			colour := "unknown"
			if idx := int(buf[s+9]); idx < len(colors) && colors[idx] != "" {
				colour = colors[idx]
			}
			interlaced := "non-"
			if buf[s+12] != 0 {
				interlaced = ""
			}
			fmt.Fprintf(w, ", %d x %d, %d-bit/%s, %sinterlaced",
				int32(magicBe(buf, s, 4)), int32(magicBe(buf, s+4, 4)), buf[s+8], colour, interlaced)
		}
		fmt.Fprintln(w)

	// https://www.w3.org/Graphics/GIF/spec-gif89a.txt
	case n > 16 && (fileMatch(buf, &s, "GIF87a") || fileMatch(buf, &s, "GIF89a")):
		fmt.Fprintf(w, "GIF image data, version %s, %d x %d\n",
			string(buf[s-3:s]), magicLe(buf, s, 2), magicLe(buf, s+2, 2))

	// https://en.wikipedia.org/wiki/JPEG#Syntax_and_structure
	case n > 32 && fileHasPrefix(buf, 0, "\xff\xd8"):
		types := []string{"baseline", "extended sequential", "progressive"}
		fmt.Fprint(w, "JPEG image data")
		for s < n-8 {
			marker := magicBe(buf, s, 2)
			if marker >= 0xffd0 && marker <= 0xffd9 {
				s += 2 // No data.
			} else if marker >= 0xffc0 && marker <= 0xffc2 {
				fmt.Fprintf(w, ", %s, %dx%d", types[marker-0xffc0],
					magicBe(buf, s+7, 2), magicBe(buf, s+5, 2))
				break
			} else {
				s += int(magicBe(buf, s+2, 2)) + 2
			}
		}
		fmt.Fprintln(w)

	case n > 8 && fileMatch(buf, &s, "\xca\xfe\xba\xbe"):
		count := magicBe(buf, s, 4)
		// 0xcafebabe can be a Java class file or a Mach-O universal binary.
		// Java major version numbers start with 0x2d for JDK 1.1.
		if count < 0x2d && uint64(n) >= count*20 {
			// https://eclecticlight.co/2020/07/28/universal-binaries-inside-fat-headers/
			plural := "s"
			if count == 1 {
				plural = ""
			}
			fmt.Fprintf(w, "Mach-O universal binary with %d architecture%s:", count, plural)
			for i := uint64(0); i < count; i++ {
				arch := magicBe(buf, s+4+int(i)*20, 4)
				name := "unknown"
				switch arch {
				case 0x00000007:
					name = "i386"
				case 0x01000007:
					name = "x86_64"
				case 0x0000000c:
					name = "arm"
				case 0x0100000c:
					name = "arm64"
				}
				fmt.Fprintf(w, " [%s]", name)
			}
			fmt.Fprintln(w)
		} else {
			// https://en.wikipedia.org/wiki/Java_class_file#General_layout
			major := magicBe(buf, s+2, 2)
			fmt.Fprintf(w, "Java class file, version %d.%d (Java 1.%d)\n",
				major, magicBe(buf, s, 2), major-44)
		}

	// https://source.android.com/devices/tech/dalvik/dex-format#dex-file-magic
	case n > 8 && fileMatch(buf, &s, "dex\n") && buf[s+3] == 0:
		fmt.Fprintf(w, "Android dex file, version %s\n", fileCString(buf[s:s+3]))

	// https://people.freebsd.org/~kientzle/libarchive/man/cpio.5.txt
	case n > 6 && fileMatch(buf, &s, "07070"):
		format := "unknown type"
		switch buf[s] {
		case '7':
			format = "pre-SVR4 or odc"
		case '1':
			format = "SVR4 with no CRC"
		case '2':
			format = "SVR4 with CRC"
		}
		fmt.Fprintf(w, "ASCII cpio archive (%s)\n", format)

	// peek(&s, 2) is a native-endian read, so 0143561 is the swapped magic.
	case n > 33 && (magicLe(buf, 0, 2) == 0143561 || magicLe(buf, 0, 2) == 070707):
		if magicLe(buf, 0, 2) == 0143561 {
			fmt.Fprint(w, "byte-swapped ")
		}
		fmt.Fprintln(w, "cpio archive")

	// tar archive (old, ustar/pax, or gnu)
	case n > 500 && fileIsTarHeader(buf):
		kind := "old"
		if buf[257] != 0 {
			kind = "POSIX"
		}
		gnu := ""
		if buf[262] == ' ' && buf[263] == ' ' {
			gnu = " (GNU)"
		}
		fmt.Fprintf(w, "%s tar archive%s\n", kind, gnu)

	// zip/jar/apk archive, ODF/OOXML document, or such
	case n > 5 && fileMatch(buf, &s, "PK\x03\x04"):
		fmt.Fprint(w, "Zip archive data")
		if buf[s] != 0 {
			fmt.Fprintf(w, ", requires at least v%d.%d to extract", buf[s]/10, buf[s]%10)
		}
		fmt.Fprintln(w)

	case n > 9 && fileMatch(buf, &s, "7z\xbc\xaf\x27\x1c"):
		fmt.Fprint(w, "7-zip archive data")
		if buf[s] != 0 || buf[s+1] != 0 {
			fmt.Fprintf(w, ", version %d.%d", buf[s], buf[s+1])
		}
		fmt.Fprintln(w)

	case n > 4 && fileMatch(buf, &s, "BZh") && buf[s] >= '0' && buf[s] <= '9':
		fmt.Fprintf(w, "bzip2 compressed data, block size = %c00k\n", buf[s])

	// memcmp() with a 7-byte literal also compares the string's NUL.
	case n > 31 && fileHasPrefix(buf, 0, "\xfd7zXZ\x00\x00"):
		fmt.Fprintln(w, "xz compressed data")

	case n > 10 && fileMatch(buf, &s, "\x28\xb5\x2f\xfd"):
		fmt.Fprintln(w, "zstd compressed data")

	case n > 10 && fileMatch(buf, &s, "\x1f\x8b"):
		fmt.Fprintln(w, "gzip compressed data")

	case n > 32 && fileHasPrefix(buf, 1, "\xfa\xed\xfe"):
		bit := 64
		if buf[0] == 0xce {
			bit = 32
		}
		fmt.Fprintf(w, "Mach-O %d-bit ", bit)
		what := ""
		switch buf[4] {
		case 7:
			what = "x86"
			if bit == 64 {
				what = "x86-"
			}
		case 12:
			what = "arm"
		case 18:
			what = "ppc"
		}
		if what != "" {
			if bit == 32 {
				fmt.Fprintf(w, "%s ", what)
			} else {
				fmt.Fprintf(w, "%s64 ", what)
			}
		} else {
			fmt.Fprintf(w, "(bad arch %d) ", buf[4])
		}
		switch buf[12] {
		case 1:
			fmt.Fprintln(w, "object")
		case 2:
			fmt.Fprintln(w, "executable")
		case 6:
			fmt.Fprintln(w, "shared library")
		default:
			fmt.Fprintf(w, "(bad type %d)\n", buf[9])
		}

	case n > 36 && fileHasPrefix(buf, 0, "OggS\x00\x02"):
		fmt.Fprint(w, "Ogg data")
		// https://wiki.xiph.org/MIMETypesCodecs
		switch {
		case fileHasPrefix(buf, s+28, "CELT    "):
			fmt.Fprint(w, ", celt audio")
		case fileHasPrefix(buf, s+28, "CMML    "):
			fmt.Fprint(w, ", cmml text")
		case fileHasPrefix(buf, s+28, "BBCD\x00"):
			fmt.Fprint(w, ", dirac video")
		case fileHasPrefix(buf, s+28, "\x7fFLAC"):
			fmt.Fprint(w, ", flac audio")
		case fileHasPrefix(buf, s+28, "\x8bJNG\r\n\x1a\n"):
			fmt.Fprint(w, ", jng video")
		case fileHasPrefix(buf, s+28, "\x80kate\x00\x00\x00"):
			fmt.Fprint(w, ", kate text")
		case fileHasPrefix(buf, s+28, "OggMIDI\x00"):
			fmt.Fprint(w, ", midi text")
		case fileHasPrefix(buf, s+28, "\x8aMNG\r\n\x1a\n"):
			fmt.Fprint(w, ", mng video")
		case fileHasPrefix(buf, s+28, "OpusHead"):
			fmt.Fprint(w, ", opus audio")
		case fileHasPrefix(buf, s+28, "PCM     "):
			fmt.Fprint(w, ", pcm audio")
		case fileHasPrefix(buf, s+28, "\x89PNG\r\n\x1a\n"):
			fmt.Fprint(w, ", png video")
		case fileHasPrefix(buf, s+28, "Speex   "):
			fmt.Fprint(w, ", speex audio")
		case fileHasPrefix(buf, s+28, "\x80theora"):
			fmt.Fprint(w, ", theora video")
		case fileHasPrefix(buf, s+28, "\x01vorbis"):
			fmt.Fprint(w, ", vorbis audio")
		case fileHasPrefix(buf, s+28, "YUV4MPEG"):
			fmt.Fprint(w, ", yuv4mpeg video")
		}
		fmt.Fprintln(w)

	case n > 32 && fileHasPrefix(buf, 0, "RIF") && fileHasPrefix(buf, 8, "WAVEfmt "):
		// https://en.wikipedia.org/wiki/WAV
		le := buf[3] == 'F'
		peek := func(off, size int) uint64 {
			if le {
				return magicLe(buf, off, size)
			}
			return magicBe(buf, off, size)
		}
		format := peek(20, 2)
		channels := peek(22, 2)
		hz := int32(peek(24, 4))
		bits := peek(34, 2)
		order := "BE"
		if le {
			order = "LE"
		}
		fmt.Fprintf(w, "WAV audio, %s, ", order)
		if bits != 0 {
			fmt.Fprintf(w, "%d-bit, ", bits)
		}
		switch channels {
		case 1:
			fmt.Fprint(w, "mono, ")
		case 2:
			fmt.Fprint(w, "stereo, ")
		default:
			fmt.Fprintf(w, "%d-channel, ", channels)
		}
		fmt.Fprintf(w, "%d Hz, ", hz)
		// https://tools.ietf.org/html/rfc2361
		switch format {
		case 0x01:
			fmt.Fprint(w, "PCM")
		case 0x03:
			fmt.Fprint(w, "IEEE float")
		case 0x06:
			fmt.Fprint(w, "A-law")
		case 0x07:
			fmt.Fprint(w, "µ-law")
		case 0x11:
			fmt.Fprint(w, "ADPCM")
		case 0x22:
			fmt.Fprint(w, "Truespeech")
		case 0x31:
			fmt.Fprint(w, "GSM")
		case 0x55:
			fmt.Fprint(w, "MP3")
		case 0x70:
			fmt.Fprint(w, "CELP")
		case 0xfffe:
			fmt.Fprint(w, "extensible")
		default:
			fmt.Fprintf(w, "unknown format %d", format)
		}
		fmt.Fprintln(w)

	case n > 12 && magicBe(buf, 0, 4) == 0x10000:
		fmt.Fprintln(w, "TrueType font")

	case n > 12 && fileHasPrefix(buf, 0, "ttcf\x00"):
		fmt.Fprintf(w, "TrueType font collection, version %d, %d fonts\n",
			magicBe(buf, 4, 2), int32(magicBe(buf, 8, 4)))

	// https://docs.microsoft.com/en-us/typography/opentype/spec/otff
	case n > 12 && fileMatch(buf, &s, "OTTO"):
		fmt.Fprintln(w, "OpenType font")

	case fileMatch(buf, &s, "BC\xc0\xde"):
		fmt.Fprintln(w, "LLVM IR bitcode")

	case fileMatch(buf, &s, "-----BEGIN CERTIFICATE-----"):
		fmt.Fprintln(w, "PEM certificate")

	// https://msdn.microsoft.com/en-us/library/windows/desktop/ms680547(v=vs.85).aspx
	case isPE:
		fileDescribePE(ctx, buf, n)

	// https://en.wikipedia.org/wiki/BMP_file_format
	case n > 0x32 && fileHasPrefix(buf, 0, "BM") && magicBe(buf, 6, 4) == 0:
		fmt.Fprintf(w, "BMP image, %d x %d, %d bpp\n",
			int32(magicLe(buf, 18, 4)), int32(magicLe(buf, 22, 4)), magicLe(buf, 28, 2))

	// https://github.com/torvalds/linux/blob/master/tools/perf/Documentation/perf.data-file-format.txt
	case n >= 104 && fileMatch(buf, &s, "PERFILE2"):
		fmt.Fprintln(w, "Linux perf data")

	// https://android.googlesource.com/platform/system/core/+/master/libsparse/sparse_format.h
	case n > 28 && magicLe(buf, 0, 4) == 0xed26ff3a:
		fmt.Fprintf(w, "Android sparse image v%d.%d, %d %d-byte blocks (%d chunks)\n",
			magicLe(buf, 4, 2), magicLe(buf, 6, 2), int32(magicLe(buf, 16, 4)),
			int32(magicLe(buf, 12, 4)), int32(magicLe(buf, 20, 4)))

	// https://android.googlesource.com/platform/system/tools/mkbootimg/+/refs/heads/master/include/bootimg/bootimg.h
	case n > 1632 && fileHasPrefix(buf, 0, "ANDROID!"):
		fmt.Fprintf(w, "Android boot image v%d\n", int32(magicLe(buf, 40, 4)))

	// https://source.android.com/devices/architecture/dto/partitions
	case n > 32 && magicBe(buf, 0, 4) == 0xd7b7ab1e:
		fmt.Fprintf(w, "Android DTB/DTBO v%d, %d entries\n",
			int32(magicBe(buf, 28, 4)), int32(magicBe(buf, 16, 4)))

	// frameworks/base/core/java/com/android/internal/util/BinaryXmlSerializer.java
	case n > 4 && fileHasPrefix(buf, 0, "ABX"):
		fmt.Fprintf(w, "Android Binary XML v%d\n", buf[3])

	// https://webassembly.github.io/spec/core/binary/modules.html#binary-module
	case n > 8 && fileHasPrefix(buf, 0, "\x00asm"):
		fmt.Fprintf(w, "wasm binary module version %d\n", int32(magicLe(buf, 4, 4)))

	// Text files, including shell scripts.
	default:
		fileDescribeText(ctx, buf, n)
	}
	return true
}

// fileIsPE is the "MZ" half of the MS PE32 test: the e_lfanew offset has to be
// inside the prefix and point at a PE signature.
func fileIsPE(buf []byte, n int) bool {
	magic := int(magicLe(buf, 0x3c, 4))
	return magic < n-4 && fileHasPrefix(buf, magic, "\x50\x45\x00\x00")
}

// fileDescribePE is the MS PE32 branch of do_regular_file(), including the Linux
// kernel images that look like PE files.
func fileDescribePE(ctx *Context, buf []byte, n int) {
	w := ctx.Stdout
	magic := int(magicLe(buf, 0x3c, 4))

	// Linux kernel images look like PE files.
	// https://www.kernel.org/doc/Documentation/arm64/booting.txt
	if fileHasPrefix(buf, 0x38, "ARMd") {
		fmt.Fprintln(w, "Linux arm64 kernel image")
		return
	}
	if fileHasPrefix(buf, 0x202, "HdrS") {
		// https://www.kernel.org/doc/Documentation/x86/boot.txt
		verOff := int(magicLe(buf, 0x20e, 2))
		fmt.Fprint(w, "Linux x86-64 kernel image")
		off := 0x200 + verOff
		var version []byte
		if off < n {
			version = buf[off:n]
		}
		fmt.Fprintf(w, ", version %s\n", fileCString(version))
		return
	}

	subsystem := ""
	if magicLe(buf, magic+22, 2)&0x2000 != 0 {
		subsystem = "(DLL) "
	}
	plus := ""
	if magicLe(buf, magic+24, 2) == 0x20b {
		plus = "+"
	}
	fmt.Fprintf(w, "MS PE32%s executable %s", plus, subsystem)
	if magicLe(buf, magic+20, 2) > 70 {
		types := []string{"", "native", "GUI", "console", "OS/2", "driver", "CE",
			"EFI", "EFI boot", "EFI runtime", "EFI ROM", "XBOX", "", "boot"}
		name := "unknown"
		if t := int(magicLe(buf, magic+92, 2)); t < len(types) && types[t] != "" {
			name = types[t]
		}
		fmt.Fprintf(w, "(%s) ", name)
	}
	arch := "-64"
	if magicLe(buf, magic+4, 2) == 0x14c {
		arch = ""
	}
	fmt.Fprintf(w, "x86%s\n", arch)
}

// fileDescribeText is the text branch of do_regular_file(): "#!" scripts, ASCII
// text, UTF-8 text, or data.
func fileDescribeText(ctx *Context, buf []byte, n int) {
	w := ctx.Stdout
	s := 0
	what := ""

	if n > 3 && fileMatch(buf, &s, "#!") {
		// Whitespace is allowed between the #! and the interpreter
		for s < n && fileIsSpace(buf[s]) {
			s++
		}
		if fileMatch(buf, &s, "/usr/bin/env") {
			for s < n && fileIsSpace(buf[s]) {
				s++
			}
		}
		start := s
		for s < n && buf[s] != 0 && !fileIsSpace(buf[s]) {
			s++
		}
		what = string(buf[start:s]) + " script"
	} else {
		// Distinguish ASCII text, UTF-8 text, or data
		for i := 0; i < n; i++ {
			c := buf[i]
			if (c >= 0x20 && c < 0x7f) || fileIsSpace(c) {
				continue
			}
			r, size := utf8.DecodeRune(buf[i:n])
			if (r != utf8.RuneError || size > 1) && fileWcWidth(r) >= 0 {
				i += size - 1
				if what == "" {
					what = "UTF-8 text"
				}
				continue
			}
			what = "data"
			break
		}
	}
	if what == "" {
		what = "ASCII text"
	}
	fmt.Fprintln(w, what)
}

// fileDoELF is do_elf_file() from toys/posix/file.c. Struct offsets are
// calculated by hand, as the C does: two code paths for 32/64 bits is awkward.
func fileDoELF(ctx *Context, src *fileSource, p []byte) {
	w := ctx.Stdout
	var (
		endian, bits, bail uint32
		arch               uint64
		dynamic            bool
		stripped           = true
		phentsize, phnum   uint64
		phoff, shsize      uint64
		shnum, shoff       uint64
		i, j               uint64
		elfInt             func([]byte, int, int) uint64
	)

	fmt.Fprint(w, "ELF ")

	endian = uint32(p[5])
	bits = uint32(p[4]) - 1
	elfInt = magicLe
	if endian == 2 {
		elfInt = magicBe
	}

	// executable type
	switch elfInt(p, 16, 2) {
	case 1:
		fmt.Fprint(w, "relocatable")
	case 2:
		fmt.Fprint(w, "executable")
	case 3:
		fmt.Fprint(w, "shared object")
	case 4:
		fmt.Fprint(w, "core dump")
	default:
		fmt.Fprintf(w, "(bad type %d)", elfInt(p, 16, 2))
		bail++
	}
	fdpicOff := 36
	if bits != 0 {
		fdpicOff = 48
	}
	if elfInt(p, fdpicOff, 4)&0x8000 != 0 {
		fmt.Fprint(w, " (fdpic)")
	}
	fmt.Fprint(w, ", ")

	// "64-bit". bits is unsigned in the C and printed with %d, so a class byte
	// of 0 (bits == 0xffffffff) prints -1, as the reference build does.
	if bits&^uint32(1) != 0 {
		fmt.Fprintf(w, "(bad class %d) ", int32(bits))
		bail++
	} else {
		fmt.Fprintf(w, "%d-bit ", uint32(32)<<bits)
	}

	// "LSB"
	switch endian {
	case 1:
		fmt.Fprint(w, "LSB ")
	case 2:
		fmt.Fprint(w, "MSB ")
	default:
		fmt.Fprintf(w, "(bad endian %d) ", endian)
		bail++
	}

	// "x86"
	arch = elfInt(p, 18, 2)
	fmt.Fprint(w, fileArchName(arch))

	// If what we've seen so far doesn't seem consistent, bail.
	if bail != 0 {
		goto bad
	}

	filePrintFlags(w, arch, elfInt(p, 36+12*int(bits), 4))

	// Stash what we need from the header; it's okay to reuse the buffer after.
	phentsize = elfInt(p, 42+12*int(bits), 2)
	phnum = elfInt(p, 44+12*int(bits), 2)
	phoff = elfInt(p, 28+4*int(bits), 4+4*int(bits))
	shsize = elfInt(p, 46+12*int(bits), 2)
	shnum = elfInt(p, 48+12*int(bits), 2)
	shoff = elfInt(p, 32+8*int(bits), 4+4*int(bits))

	// With binutils, phentsize seems to only be non-zero if phnum is non-zero.
	// Such ELF files are rare, but do exist. (Android's crtbegin files, say.)
	if phnum != 0 && phentsize != 32+24*uint64(bits) {
		fmt.Fprintf(w, ", bad phentsize %d?", phentsize)
		goto bad
	}
	if phoff > src.size || phnum*phentsize > src.size-phoff {
		fmt.Fprintf(w, ", bad phoff %d?", phoff)
		goto bad
	}
	if shoff > src.size || shnum*shsize > src.size-shoff {
		// The C prints phoff here; keep the quirk.
		fmt.Fprintf(w, ", bad shoff %d?", phoff)
		goto bad
	}

	// Read the phdrs for dynamic vs static. (Note: fields reordered on 64 bit)
	for i = 0; i < phnum; i++ {
		phdr, ok := src.readAt(phoff+i*phentsize, phentsize)
		if !ok {
			fmt.Fprintf(w, ", bad phdr %d?", i)
			goto bad
		}
		pType := magicLe(phdr, 0, 4)
		if endian == 2 {
			pType = magicBe(phdr, 0, 4)
		}
		if pType < 2 || pType > 4 { // 2 = PT_DYNAMIC, 3 = PT_INTERP, 4 = PT_NOTE
			continue
		}
		if pType == 2 {
			dynamic = true
		}
		readInt := func(off, size int) uint64 {
			if endian == 2 {
				return magicBe(phdr, off, size)
			}
			return magicLe(phdr, off, size)
		}
		pOffset := readInt(4<<bits, int(4<<bits))
		pFilesz := readInt(16<<bits, int(4<<bits))
		if pType == 3 {
			if pOffset > src.size || pFilesz > src.size-pOffset {
				fmt.Fprintf(w, ", bad phdr %d?", i)
				goto bad
			}
			want := pFilesz
			if want > fileELFMaxItem {
				want = fileELFMaxItem
			}
			str, ok := src.readAt(pOffset, want)
			if !ok {
				fmt.Fprintf(w, ", bad phdr %d?", i)
				goto bad
			}
			fmt.Fprintf(w, ", dynamic (%s)", fileCString(str))
		}
	}
	if !dynamic {
		fmt.Fprint(w, ", static")
	}

	// We need to read the shdrs for stripped/unstripped and any notes. Notes are
	// in program headers *and* section headers, but some files don't contain
	// program headers, so check here. (Note: fields reordered on 64 bit)
	for i = 0; i < shnum; i++ {
		shdrOff := shoff + i*shsize
		shdrLen := uint64(8 + (12 << bits) + 4)
		if shdrOff > src.size || src.size-shdrOff < shdrLen {
			fmt.Fprintf(w, ", bad shdr %d?", i)
			goto bad
		}
		shdr, ok := src.readAt(shdrOff, shdrLen)
		if !ok {
			fmt.Fprintf(w, ", bad shdr %d?", i)
			goto bad
		}
		readInt := func(off, size int) uint64 {
			if endian == 2 {
				return magicBe(shdr, off, size)
			}
			return magicLe(shdr, off, size)
		}
		shType := readInt(4, 4)
		shOffset := readInt(8+(8<<bits), int(4<<bits))
		shSize := readInt(8+(12<<bits), 4)
		if shType == 8 { // SHT_NOBITS
			shSize = 0
		}
		if shOffset > src.size || shSize > src.size-shOffset {
			fmt.Fprintf(w, ", bad shdr %d?", i)
			goto bad
		}

		if shType == 2 { // SHT_SYMTAB
			stripped = false
			break
		} else if shType == 7 { // SHT_NOTE
			noteOff := shOffset

			// An ELF note is a sequence of entries, each consisting of a
			// ndhr followed by n_namesz+n_descsz bytes of data (each of those
			// rounded up to the next 4 bytes, without this being reflected in
			// the header byte counts themselves).
			for shSize >= 3*4 { // Don't try to read a truncated entry.
				if noteOff > src.size || src.size-noteOff < 3*4 {
					fmt.Fprintf(w, ", bad note %d?", i)
					goto bad
				}
				note, ok := src.readAt(noteOff, 3*4)
				if !ok {
					fmt.Fprintf(w, ", bad note %d?", i)
					goto bad
				}
				readInt := func(off, size int) uint64 {
					if endian == 2 {
						return magicBe(note, off, size)
					}
					return magicLe(note, off, size)
				}
				nNamesz := readInt(0, 4)
				nDescsz := readInt(4, 4)
				nType := readInt(8, 4)
				notesz := 3*4 + ((nNamesz + 3) &^ 3) + ((nDescsz + 3) &^ 3)

				// Are the name/desc sizes consistent, and does the claimed size
				// of the note actually fit in the section?
				if notesz < nNamesz || notesz < nDescsz || notesz > shSize {
					fmt.Fprintf(w, ", bad note %d size?", i)
					goto bad
				}

				if nNamesz == 4 && nType == 3 {
					name, ok := src.readAt(noteOff+12, 4)
					if !ok {
						fmt.Fprintf(w, ", bad note %d?", i)
						goto bad
					}
					if fileHasPrefix(name, 0, "GNU\x00") {
						fmt.Fprint(w, ", BuildID=")
						want := nDescsz
						if want > fileELFMaxItem {
							want = fileELFMaxItem
						}
						desc, ok := src.readAt(noteOff+16, want)
						if !ok {
							fmt.Fprintf(w, ", bad note %d?", i)
							goto bad
						}
						for j = 0; j < uint64(len(desc)); j++ {
							fmt.Fprintf(w, "%02x", desc[j])
						}
					}
				} else if nNamesz == 8 {
					name, ok := src.readAt(noteOff+12, 8)
					if !ok {
						fmt.Fprintf(w, ", bad note %d?", i)
						goto bad
					}
					if fileHasPrefix(name, 0, "Android\x00") && nType == 1 && nDescsz >= 4 {
						api, ok := src.readAt(noteOff+20, 4)
						if !ok {
							fmt.Fprintf(w, ", bad note %d?", i)
							goto bad
						}
						v := int32(magicLe(api, 0, 4))
						if endian == 2 {
							v = int32(magicBe(api, 0, 4))
						}
						fmt.Fprintf(w, ", for Android %d", v)
						// NDK r14 and later also include NDK version info. OS
						// binaries and binaries built by older NDKs don't.
						if nDescsz >= 4+64+64 {
							ndk, ok := src.readAt(noteOff+24, 128)
							if !ok {
								fmt.Fprintf(w, ", bad note %d?", i)
								goto bad
							}
							fmt.Fprintf(w, ", built by NDK %s (%s)",
								fileCString(ndk[:64]), fileCString(ndk[64:]))
						}
					}
				}

				noteOff += notesz
				shSize -= notesz
			}
		}
	}
	if stripped {
		fmt.Fprint(w, ", stripped")
	} else {
		fmt.Fprint(w, ", not stripped")
	}

bad:
	fmt.Fprintln(w)
}
