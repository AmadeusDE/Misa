package builtins

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- synthetic ELF fixtures -------------------------------------------------

// readelfTestSec is one section for readelfBuildELF.
type readelfTestSec struct {
	name      string
	typ       uint32
	flags     uint64
	addr      uint64
	data      []byte
	link      uint32
	info      uint32
	addralign uint64
	entsize   uint64
}

// readelfTestPh is one program header for readelfBuildELF. When sec >= 0 the
// offset, sizes and addresses come from that section.
type readelfTestPh struct {
	typ    uint32
	flags  uint32
	sec    int
	off    uint64
	filesz uint64
	memsz  uint64
	vaddr  uint64
	paddr  uint64
	align  uint64
}

// readelfTestELF describes a fixture.
type readelfTestELF struct {
	bits    int // 32 or 64
	be      bool
	machine uint16
	etype   uint16
	entry   uint64
	flags   uint32
	secs    []readelfTestSec
	phs     []readelfTestPh
}

// readelfLayout records where the builder put everything.
type readelfLayout struct {
	ehsize    int
	phentsize int
	shentsize int
	phoff     uint64
	shoff     uint64
	phnum     int
	shnum     int
	shstrndx  int
	secOff    []uint64
	secName   []uint32
	shstrOff  uint64
	shstrSize uint64
	size      uint64
	be        bool
	bits      int
}

func (l readelfLayout) put16(b []byte, off uint64, v uint16) {
	if l.be {
		b[off] = byte(v >> 8)
		b[off+1] = byte(v)
	} else {
		b[off] = byte(v)
		b[off+1] = byte(v >> 8)
	}
}

func (l readelfLayout) put32(b []byte, off uint64, v uint32) {
	if l.be {
		b[off] = byte(v >> 24)
		b[off+1] = byte(v >> 16)
		b[off+2] = byte(v >> 8)
		b[off+3] = byte(v)
	} else {
		b[off] = byte(v)
		b[off+1] = byte(v >> 8)
		b[off+2] = byte(v >> 16)
		b[off+3] = byte(v >> 24)
	}
}

func (l readelfLayout) put64(b []byte, off uint64, v uint64) {
	if l.be {
		for i := 0; i < 8; i++ {
			b[off+uint64(i)] = byte(v >> uint(56-8*i))
		}
	} else {
		for i := 0; i < 8; i++ {
			b[off+uint64(i)] = byte(v >> uint(8*i))
		}
	}
}

func (l readelfLayout) putWord(b []byte, off uint64, v uint64) uint64 {
	if l.bits == 64 {
		l.put64(b, off, v)
		return 8
	}
	l.put32(b, off, uint32(v))
	return 4
}

func readelfAlign(v, a uint64) uint64 {
	if a < 1 {
		a = 1
	}
	return (v + a - 1) / a * a
}

// readelfBuildELF lays out an ELF file byte by byte and returns it together
// with the layout. A .shstrtab section is always appended.
func readelfBuildELF(f readelfTestELF) ([]byte, readelfLayout) {
	l := readelfLayout{bits: f.bits, be: f.be}
	l.ehsize, l.phentsize, l.shentsize = 52, 32, 40
	if f.bits == 64 {
		l.ehsize, l.phentsize, l.shentsize = 64, 56, 64
	}
	l.phnum = len(f.phs)

	secs := append([]readelfTestSec(nil), f.secs...)
	names := make([]string, len(secs)+1)
	for i, s := range secs {
		names[i] = s.name
	}
	names[len(secs)] = ".shstrtab"
	shstr := []byte{0}
	l.secName = make([]uint32, len(names))
	for i, n := range names {
		l.secName[i] = uint32(len(shstr))
		shstr = append(shstr, n...)
		shstr = append(shstr, 0)
	}
	secs = append(secs, readelfTestSec{name: ".shstrtab", typ: 3, addralign: 1, data: shstr})
	l.shstrndx = len(secs) - 1
	l.shnum = len(secs)

	l.phoff = uint64(l.ehsize)
	l.secOff = make([]uint64, len(secs))
	off := uint64(l.ehsize + len(f.phs)*l.phentsize)
	for i := range secs {
		off = readelfAlign(off, secs[i].addralign)
		l.secOff[i] = off
		off += uint64(len(secs[i].data))
	}
	l.shoff = readelfAlign(off, 8)
	l.shstrOff = l.secOff[l.shstrndx]
	l.shstrSize = uint64(len(shstr))
	l.size = l.shoff + uint64(len(secs)*l.shentsize)

	b := make([]byte, l.size)

	// ELF header.
	copy(b, []byte{0x7f, 'E', 'L', 'F'})
	b[4] = 1
	if f.bits == 64 {
		b[4] = 2
	}
	b[5] = 1
	if f.be {
		b[5] = 2
	}
	b[6] = 1
	b[7] = 0
	b[8] = 0
	pos := uint64(16)
	l.put16(b, pos, f.etype)
	l.put16(b, pos+2, f.machine)
	l.put32(b, pos+4, 1)
	pos += 8
	pos += l.putWord(b, pos, f.entry)
	pos += l.putWord(b, pos, l.phoff)
	pos += l.putWord(b, pos, l.shoff)
	l.put32(b, pos, f.flags)
	pos += 4
	l.put16(b, pos, uint16(l.ehsize))
	l.put16(b, pos+2, uint16(l.phentsize))
	l.put16(b, pos+4, uint16(l.phnum))
	l.put16(b, pos+6, uint16(l.shentsize))
	l.put16(b, pos+8, uint16(l.shnum))
	l.put16(b, pos+10, uint16(l.shstrndx))

	// Section contents.
	for i, s := range secs {
		copy(b[l.secOff[i]:], s.data)
	}

	// Program headers.
	for i, ph := range f.phs {
		off := l.phoff + uint64(i*l.phentsize)
		var o, filesz, memsz, vaddr, paddr uint64
		if ph.sec >= 0 {
			o = l.secOff[ph.sec]
			filesz = uint64(len(secs[ph.sec].data))
			memsz = filesz
			vaddr = secs[ph.sec].addr
			paddr = vaddr
		} else {
			o, filesz, memsz, vaddr, paddr = ph.off, ph.filesz, ph.memsz, ph.vaddr, ph.paddr
		}
		if ph.filesz != 0 {
			filesz = ph.filesz
		}
		if ph.memsz != 0 {
			memsz = ph.memsz
		}
		if ph.vaddr != 0 {
			vaddr = ph.vaddr
			paddr = ph.vaddr
		}
		if ph.paddr != 0 {
			paddr = ph.paddr
		}
		if ph.off != 0 {
			o = ph.off
		}
		l.put32(b, off, ph.typ)
		if f.bits == 64 {
			l.put32(b, off+4, ph.flags)
			l.put64(b, off+8, o)
			l.put64(b, off+16, vaddr)
			l.put64(b, off+24, paddr)
			l.put64(b, off+32, filesz)
			l.put64(b, off+40, memsz)
			l.put64(b, off+48, ph.align)
		} else {
			l.put32(b, off+4, uint32(o))
			l.put32(b, off+8, uint32(vaddr))
			l.put32(b, off+12, uint32(paddr))
			l.put32(b, off+16, uint32(filesz))
			l.put32(b, off+20, uint32(memsz))
			l.put32(b, off+24, ph.flags)
			l.put32(b, off+28, uint32(ph.align))
		}
	}

	// Section headers.
	for i, s := range secs {
		off := l.shoff + uint64(i*l.shentsize)
		size := uint64(len(s.data))
		if s.typ == readelfSHTNobits {
			size = s.entsize // NOBITS reuses entsize as the "size" knob
		}
		l.put32(b, off, l.secName[i])
		l.put32(b, off+4, s.typ)
		pos := off + 8
		pos += l.putWord(b, pos, s.flags)
		pos += l.putWord(b, pos, s.addr)
		pos += l.putWord(b, pos, l.secOff[i])
		pos += l.putWord(b, pos, size)
		l.put32(b, pos, s.link)
		l.put32(b, pos+4, s.info)
		pos += 8
		pos += l.putWord(b, pos, s.addralign)
		l.putWord(b, pos, s.entsize)
	}
	return b, l
}

// readelfUleb encodes a uleb128 value (for RISC-V attribute fixtures).
func readelfUleb(v uint64) []byte {
	var out []byte
	for {
		c := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			out = append(out, c|0x80)
		} else {
			return append(out, c)
		}
	}
}

// readelfNote builds one ELF note.
func readelfNote(owner string, typ uint32, desc []byte, be bool) []byte {
	var b bytes.Buffer
	put := func(v uint32) {
		var t [4]byte
		if be {
			t = [4]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
		} else {
			t = [4]byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
		}
		b.Write(t[:])
	}
	put(uint32(len(owner) + 1))
	put(uint32(len(desc)))
	put(typ)
	b.WriteString(owner)
	b.WriteByte(0)
	for b.Len()%4 != 0 {
		b.WriteByte(0)
	}
	b.Write(desc)
	for b.Len()%4 != 0 {
		b.WriteByte(0)
	}
	return b.Bytes()
}

// readelfSymbol builds one symbol table entry.
func readelfSymbol(bits int, be bool, name uint32, info, other byte, shndx uint16, value, size uint64) []byte {
	var b bytes.Buffer
	put32 := func(v uint32) {
		var t [4]byte
		if be {
			t = [4]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
		} else {
			t = [4]byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
		}
		b.Write(t[:])
	}
	put16 := func(v uint16) {
		if be {
			b.Write([]byte{byte(v >> 8), byte(v)})
		} else {
			b.Write([]byte{byte(v), byte(v >> 8)})
		}
	}
	put64 := func(v uint64) {
		var t [8]byte
		for i := 0; i < 8; i++ {
			if be {
				t[i] = byte(v >> uint(56-8*i))
			} else {
				t[i] = byte(v >> uint(8*i))
			}
		}
		b.Write(t[:])
	}
	put32(name)
	if bits == 64 {
		b.WriteByte(info)
		b.WriteByte(other)
		put16(shndx)
		put64(value)
		put64(size)
	} else {
		put32(uint32(value))
		put32(uint32(size))
		b.WriteByte(info)
		b.WriteByte(other)
		put16(shndx)
	}
	return b.Bytes()
}

// readelfDynamic builds one .dynamic entry pair.
func readelfDynamic(bits int, be bool, tag, val uint64) []byte {
	var b bytes.Buffer
	put := func(v uint64) {
		n := 4
		if bits == 64 {
			n = 8
		}
		for i := 0; i < n; i++ {
			if be {
				b.WriteByte(byte(v >> uint(8*(n-1-i))))
			} else {
				b.WriteByte(byte(v >> uint(8*i)))
			}
		}
	}
	put(tag)
	put(val)
	return b.Bytes()
}

// readelfSymEntSize is the size of one symbol table entry.
func readelfSymEntSize(bits int) uint64 {
	if bits == 64 {
		return 24
	}
	return 16
}

// readelfDynEntSize is the size of one .dynamic entry.
func readelfDynEntSize(bits int) uint64 {
	if bits == 64 {
		return 16
	}
	return 8
}

// readelfMachine picks a machine with a distinct elf_arch_name() per fixture.
func readelfMachine(bits int, be bool) uint16 {
	switch {
	case bits == 32 && !be:
		return 40 // arm
	case bits == 32 && be:
		return 8 // mips
	case bits == 64 && !be:
		return 183 // arm64
	default:
		return 21 // ppc64
	}
}

// readelfEFlags returns header flags exercising elf_print_flags().
func readelfEFlags(bits int, be bool) uint32 {
	if bits == 32 && !be {
		return 0x05000200 // EABI5, soft float
	}
	return 0
}

// readelfStdFixture is the standard 10-section test file: .interp, .text,
// .note.gnu.build-id, .dynstr, .dynsym, .dynamic, .symtab, .strtab.
func readelfStdFixture(bits int, be bool) ([]byte, readelfLayout) {
	desc := []byte{0xde, 0xad, 0xbe, 0xef, 0x01, 0x02, 0x03, 0x04}
	dynstr := []byte("\x00main\x00libc.so\x00mylib.so\x00")
	needed := uint64(6)  // "libc.so"
	soname := uint64(14) // "mylib.so"
	strtab := []byte("\x00main\x00")
	text := []byte{
		0x55, 0x48, 0x89, 0xe5, 0xb8, 0x2a, 0x00, 0x00,
		0x00, 0x5d, 0xc3, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x90, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90,
		0x00, 0x01, 0x02, 0x03, 0x7f, 0x41, 0x42, 0x43,
	}
	interp := []byte("/system/bin/linker\x00")

	var dyn []byte
	dyn = append(dyn, readelfDynamic(bits, be, 1, needed)...)              // NEEDED
	dyn = append(dyn, readelfDynamic(bits, be, 14, soname)...)             // SONAME
	dyn = append(dyn, readelfDynamic(bits, be, 28, 16)...)                 // FINI_ARRAYSZ
	dyn = append(dyn, readelfDynamic(bits, be, 30, 8)...)                  // FLAGS: BIND_NOW
	dyn = append(dyn, readelfDynamic(bits, be, 0x6ffffffb, 0x08000001)...) // FLAGS_1: NOW|PIE
	dyn = append(dyn, readelfDynamic(bits, be, 20, 7)...)                  // PLTREL: RELA
	dyn = append(dyn, readelfDynamic(bits, be, 0, 0)...)                   // NULL

	var dynsym []byte
	dynsym = append(dynsym, readelfSymbol(bits, be, 0, 0, 0, 0, 0, 0)...)
	dynsym = append(dynsym, readelfSymbol(bits, be, 1, 0x12, 0, 12, 0x1010, 12)...)

	var symtab []byte
	symtab = append(symtab, readelfSymbol(bits, be, 0, 0, 0, 0, 0, 0)...)
	symtab = append(symtab, readelfSymbol(bits, be, 1, 0x12, 0, 1, 0x1000, 32)...)
	symtab = append(symtab, readelfSymbol(bits, be, 0, 0x11, 0, 0xfff1, 0x2000, 4)...)

	ehsize, phentsize := 52, 32
	if bits == 64 {
		ehsize, phentsize = 64, 56
	}

	secs := []readelfTestSec{
		{name: "", typ: 0},
		{name: ".interp", typ: 1, flags: 2, addr: 0x400, data: interp, addralign: 1},
		{name: ".text", typ: 1, flags: 6, addr: 0x1000, data: text, addralign: 16},
		{name: ".note.gnu.build-id", typ: 7, flags: 2, addr: 0x800,
			data: readelfNote("GNU", 3, desc, be), addralign: 4},
		{name: ".dynstr", typ: 3, flags: 2, data: dynstr, addralign: 1},
		{name: ".dynsym", typ: 11, flags: 2, data: dynsym, link: 4, info: 1,
			addralign: 4, entsize: readelfSymEntSize(bits)},
		{name: ".dynamic", typ: 6, flags: 3, data: dyn, link: 4,
			addralign: 4, entsize: readelfDynEntSize(bits)},
		{name: ".symtab", typ: 2, data: symtab, link: 8, info: 2,
			addralign: 4, entsize: readelfSymEntSize(bits)},
		{name: ".strtab", typ: 3, data: strtab, addralign: 1},
	}
	phs := []readelfTestPh{
		{typ: 6, flags: 4, sec: -1, off: uint64(ehsize),
			filesz: uint64(6 * phentsize), memsz: uint64(6 * phentsize), align: 8},
		{typ: 3, flags: 4, sec: 1, align: 1},
		{typ: 1, flags: 5, sec: 2, align: 0x1000},
		{typ: 4, flags: 4, sec: 3, align: 4},
		{typ: 2, flags: 6, sec: 6, align: 4},
		{typ: 0x6474e551, flags: 6, sec: -1, align: 16},
	}
	return readelfBuildELF(readelfTestELF{
		bits: bits, be: be, machine: readelfMachine(bits, be), etype: 3,
		entry: 0x1000, flags: readelfEFlags(bits, be), secs: secs, phs: phs,
	})
}

// readelfU32LE encodes a little-endian uint32 (attribute subsections).
func readelfU32LE(v uint32) []byte {
	return []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
}

// readelfManySections builds a 64-bit file with n one-byte sections named
// .s0, .s1, ... so that section 22 can be selected by number.
func readelfManySections(n int) ([]byte, readelfLayout) {
	secs := []readelfTestSec{{name: "", typ: 0}}
	for i := 0; i < n; i++ {
		secs = append(secs, readelfTestSec{
			name: fmt.Sprintf(".s%d", i), typ: 1,
			data: []byte(fmt.Sprintf("data%d", i)), addralign: 1,
		})
	}
	return readelfBuildELF(readelfTestELF{bits: 64, machine: 183, etype: 2, secs: secs})
}

// readelfRISCVAttrs builds an SHT_RISCV_ATTRIBUTES payload with the given
// sub-subsection tag.
func readelfRISCVAttrs(tag uint64) []byte {
	var d []byte
	d = append(d, 'A')
	d = append(d, readelfU32LE(4)...) // subsection length
	d = append(d, []byte("riscv\x00")...)
	d = append(d, readelfUleb(tag)...)
	d = append(d, readelfU32LE(8)...) // sub-subsection length
	d = append(d, readelfUleb(5)...)  // arch: odd tags are strings
	d = append(d, []byte("rv64i2p0\x00")...)
	d = append(d, readelfUleb(4)...) // stack_align
	d = append(d, readelfUleb(16)...)
	d = append(d, readelfUleb(6)...) // unaligned_access
	d = append(d, readelfUleb(1)...)
	d = append(d, readelfUleb(0x10)...) // x3_reg_usage
	d = append(d, readelfUleb(2)...)
	return d
}

// readelfRISCVFixture is a RISC-V file with one attributes section.
func readelfRISCVFixture(tag uint64) ([]byte, readelfLayout) {
	secs := []readelfTestSec{
		{name: "", typ: 0},
		{name: ".text", typ: 1, flags: 6, data: []byte("code"), addralign: 1},
		{name: ".riscv.attributes", typ: readelfSHTRiscvAttributes,
			data: readelfRISCVAttrs(tag), addralign: 1},
	}
	return readelfBuildELF(readelfTestELF{bits: 64, machine: readelfEMRISCV, etype: 2, secs: secs})
}

// readelfNotesFixture has one SHT_NOTE section full of notes: every GNU,
// Android, CORE, LINUX and unknown-owner path readelf decodes.
func readelfNotesFixture() ([]byte, readelfLayout) {
	arm64 := []byte{}
	arm64 = append(arm64, readelfU32LE(0xc0000000)...) // GNU_PROPERTY_AARCH64_FEATURE_1_AND
	arm64 = append(arm64, readelfU32LE(4)...)
	arm64 = append(arm64, readelfU32LE(3)...) // bti|pac
	arm64 = append(arm64, readelfU32LE(0)...)

	x86 := []byte{}
	x86 = append(x86, readelfU32LE(0xc0000002)...) // GNU_PROPERTY_X86_FEATURE_1_AND
	x86 = append(x86, readelfU32LE(4)...)
	x86 = append(x86, readelfU32LE(3)...) // ibt|shstk
	x86 = append(x86, readelfU32LE(0)...)
	x86 = append(x86, readelfU32LE(0xc0008002)...) // GNU_PROPERTY_X86_ISA_1_NEEDED
	x86 = append(x86, readelfU32LE(4)...)
	x86 = append(x86, readelfU32LE(1)...)
	x86 = append(x86, readelfU32LE(0)...)

	ndk := []byte{}
	ndk = append(ndk, readelfU32LE(29)...) // API level
	name := make([]byte, 64)
	copy(name, "r26d")
	build := make([]byte, 64)
	copy(build, "1234567")
	ndk = append(ndk, name...)
	ndk = append(ndk, build...)

	var data []byte
	data = append(data, readelfNote("GNU", 3, []byte{1, 2, 3, 4, 5, 6, 7, 8}, false)...)
	data = append(data, readelfNote("GNU", 5, arm64, false)...)
	data = append(data, readelfNote("GNU", 5, x86, false)...)
	data = append(data, readelfNote("GNU", 99, []byte{0xaa, 0xbb, 0xcc, 0xdd}, false)...)
	data = append(data, readelfNote("Android", 1, ndk, false)...)
	data = append(data, readelfNote("Android", 5, readelfU32LE(0x2000), false)...)
	data = append(data, readelfNote("CORE", 1, []byte{0, 0, 0, 0}, false)...)
	data = append(data, readelfNote("LINUX", 0x200, []byte{0, 0, 0, 0}, false)...)
	data = append(data, readelfNote("FOO", 7, []byte{0xde, 0xad}, false)...)

	secs := []readelfTestSec{
		{name: "", typ: 0},
		{name: ".note.multi", typ: 7, flags: 2, addr: 0x400, data: data, addralign: 4},
	}
	return readelfBuildELF(readelfTestELF{bits: 64, machine: 183, etype: 3, secs: secs})
}

const wantHdr32LE = `ELF Header:
  Magic:   7f 45 4c 46 01 01 01 00 00 00 00 00 00 00 00 00
  Class:                             ELF32
  Data:                              2's complement, little endian
  Version:                           1 (current)
  OS/ABI:                            UNIX - System V
  ABI Version:                       0
  Type:                              DYN (Shared object file)
  Machine:                           arm
  Version:                           0x1
  Entry point address:               0x1000
  Start of program headers:          52 (bytes into file)
  Start of section headers:          584 (bytes into file)
  Flags:                             0x5000200, EABI5, soft float
  Size of this header:               52 (bytes)
  Size of program headers:           32 (bytes)
  Number of program headers:         6
  Size of section headers:           40 (bytes)
  Number of section headers:         10
  Section header string table index: 9
`
const wantHdr32BE = `ELF Header:
  Magic:   7f 45 4c 46 01 02 01 00 00 00 00 00 00 00 00 00
  Class:                             ELF32
  Data:                              2's complement, big endian
  Version:                           1 (current)
  OS/ABI:                            UNIX - System V
  ABI Version:                       0
  Type:                              DYN (Shared object file)
  Machine:                           mips
  Version:                           0x1
  Entry point address:               0x1000
  Start of program headers:          52 (bytes into file)
  Start of section headers:          584 (bytes into file)
  Flags:                             0x0
  Size of this header:               52 (bytes)
  Size of program headers:           32 (bytes)
  Number of program headers:         6
  Size of section headers:           40 (bytes)
  Number of section headers:         10
  Section header string table index: 9
`
const wantHdr64LE = `ELF Header:
  Magic:   7f 45 4c 46 02 01 01 00 00 00 00 00 00 00 00 00
  Class:                             ELF64
  Data:                              2's complement, little endian
  Version:                           1 (current)
  OS/ABI:                            UNIX - System V
  ABI Version:                       0
  Type:                              DYN (Shared object file)
  Machine:                           arm64
  Version:                           0x1
  Entry point address:               0x1000
  Start of program headers:          64 (bytes into file)
  Start of section headers:          840 (bytes into file)
  Flags:                             0x0
  Size of this header:               64 (bytes)
  Size of program headers:           56 (bytes)
  Number of program headers:         6
  Size of section headers:           64 (bytes)
  Number of section headers:         10
  Section header string table index: 9
`
const wantHdr64BE = `ELF Header:
  Magic:   7f 45 4c 46 02 02 01 00 00 00 00 00 00 00 00 00
  Class:                             ELF64
  Data:                              2's complement, big endian
  Version:                           1 (current)
  OS/ABI:                            UNIX - System V
  ABI Version:                       0
  Type:                              DYN (Shared object file)
  Machine:                           ppc64
  Version:                           0x1
  Entry point address:               0x1000
  Start of program headers:          64 (bytes into file)
  Start of section headers:          840 (bytes into file)
  Flags:                             0x0
  Size of this header:               64 (bytes)
  Size of program headers:           56 (bytes)
  Number of program headers:         6
  Size of section headers:           64 (bytes)
  Number of section headers:         10
  Section header string table index: 9
`
const wantSections64LE = `There are 10 section headers, starting at offset 0x348:

Section Headers:
  [Nr] Name              Type            Address          Off    Size   ES Flg Lk Inf Al
  [ 0]                   NULL            0000000000000000 000190 000000 00      0  0  0
  [ 1] .interp           PROGBITS        0000000000000400 000190 000013 00   A  0  0  1
  [ 2] .text             PROGBITS        0000000000001000 0001b0 000020 00  AX  0  0 16
  [ 3] .note.gnu.build-id NOTE            0000000000000800 0001d0 000018 00   A  0  0  4
  [ 4] .dynstr           STRTAB          0000000000000000 0001e8 000017 00   A  0  0  1
  [ 5] .dynsym           DYNSYM          0000000000000000 000200 000030 18   A  4  1  4
  [ 6] .dynamic          DYNAMIC         0000000000000000 000230 000070 10  WA  4  0  4
  [ 7] .symtab           SYMTAB          0000000000000000 0002a0 000048 18      8  2  4
  [ 8] .strtab           STRTAB          0000000000000000 0002e8 000006 00      0  0  1
  [ 9] .shstrtab         STRTAB          0000000000000000 0002ee 000056 00      0  0  1
Key:
  (W)rite, (A)lloc, e(X)ecute, (M)erge, (S)trings, (I)nfo
  (L)ink order, (O)S, (G)roup, (T)LS, (C)ompressed, (x) unknown
  (y) purecode
`
const wantSections32BE = `There are 10 section headers, starting at offset 0x248:

Section Headers:
  [Nr] Name              Type            Address  Off    Size   ES Flg Lk Inf Al
  [ 0]                   NULL            00000000 0000f4 000000 00      0  0  0
  [ 1] .interp           PROGBITS        00000400 0000f4 000013 00   A  0  0  1
  [ 2] .text             PROGBITS        00001000 000110 000020 00  AX  0  0 16
  [ 3] .note.gnu.build-id NOTE            00000800 000130 000018 00   A  0  0  4
  [ 4] .dynstr           STRTAB          00000000 000148 000017 00   A  0  0  1
  [ 5] .dynsym           DYNSYM          00000000 000160 000020 10   A  4  1  4
  [ 6] .dynamic          DYNAMIC         00000000 000180 000038 08  WA  4  0  4
  [ 7] .symtab           SYMTAB          00000000 0001b8 000030 10      8  2  4
  [ 8] .strtab           STRTAB          00000000 0001e8 000006 00      0  0  1
  [ 9] .shstrtab         STRTAB          00000000 0001ee 000056 00      0  0  1
Key:
  (W)rite, (A)lloc, e(X)ecute, (M)erge, (S)trings, (I)nfo
  (L)ink order, (O)S, (G)roup, (T)LS, (C)ompressed, (x) unknown
`
const wantPrograms64LE = `
Elf file type is DYN (Shared object file)
Entry point 0x1000
There are 6 program headers, starting at offset 64

Program Headers:
  Type           Offset   VirtAddr           PhysAddr           FileSiz MemSiz  Flg Align
  PHDR           0x000040 0x0000000000000000 0x0000000000000000 0x00150 0x00150 R   0x8
  INTERP         0x000190 0x0000000000000400 0x0000000000000400 0x00013 0x00013 R   0x1
      [Requesting program interpreter: /system/bin/linker]
  LOAD           0x0001b0 0x0000000000001000 0x0000000000001000 0x00020 0x00020 R E 0x1000
  NOTE           0x0001d0 0x0000000000000800 0x0000000000000800 0x00018 0x00018 R   0x4
  DYNAMIC        0x000230 0x0000000000000000 0x0000000000000000 0x00070 0x00070 RW  0x4
  GNU_STACK      0x000000 0x0000000000000000 0x0000000000000000 0x00000 0x00000 RW  0x10

 Section to Segment mapping:
  Segment Sections...
   00     
   01     .interp 
   02     .text 
   03     .note.gnu.build-id 
   04     .dynamic 
   05     
`
const wantPrograms32BE = `
Elf file type is DYN (Shared object file)
Entry point 0x1000
There are 6 program headers, starting at offset 52

Program Headers:
  Type           Offset   VirtAddr   PhysAddr   FileSiz MemSiz  Flg Align
  PHDR           0x000034 0x00000000 0x00000000 0x000c0 0x000c0 R   0x8
  INTERP         0x0000f4 0x00000400 0x00000400 0x00013 0x00013 R   0x1
      [Requesting program interpreter: /system/bin/linker]
  LOAD           0x000110 0x00001000 0x00001000 0x00020 0x00020 R E 0x1000
  NOTE           0x000130 0x00000800 0x00000800 0x00018 0x00018 R   0x4
  DYNAMIC        0x000180 0x00000000 0x00000000 0x00038 0x00038 RW  0x4
  GNU_STACK      0x000000 0x00000000 0x00000000 0x00000 0x00000 RW  0x10

 Section to Segment mapping:
  Segment Sections...
   00     
   01     .interp 
   02     .text 
   03     .note.gnu.build-id 
   04     .dynamic 
   05     
`
const wantNotes64LE = `
Displaying notes found in: .note.gnu.build-id
  Owner                 Data size	Description
  GNU                  0x00000008	NT_GNU_BUILD_ID	deadbeef01020304
`
const wantNotesMulti = `
Displaying notes found in: .note.multi
  Owner                 Data size	Description
  GNU                  0x00000008	NT_GNU_BUILD_ID	0102030405060708
  GNU                  0x00000010	NT_GNU_PROPERTY_TYPE_0
    Properties:	arm64 features: bti pac

  GNU                  0x00000020	NT_GNU_PROPERTY_TYPE_0
    Properties:	x86 feature: ibt shstk
	x86 isa needed: x86-64v1
  GNU                  0x00000004	0x63		47 4e 55 00
  Android              0x00000084	NT_VERSION	API level 29, NDK r26d (1234567)
  Android              0x00000004	NT_PAD_SEGMENT	pad_segment=8192
  CORE                 0x00000004	NT_PRSTATUS
  LINUX                0x00000004	NT_386_TLS
  FOO                  0x00000002	0x7		46 4f
`
const wantSymbols64LE = `
Symbol table '.dynsym' contains 2 entries:
   Num:            Value  Size Type    Bind   Vis      Ndx Name
     0: 0000000000000000     0 NOTYPE  LOCAL  DEFAULT  UND 
     1: 0000000000001010    12 FUNC    GLOBAL DEFAULT   12 main

Symbol table '.symtab' contains 3 entries:
   Num:            Value  Size Type    Bind   Vis      Ndx Name
     0: 0000000000000000     0 NOTYPE  LOCAL  DEFAULT  UND 
     1: 0000000000001000    32 FUNC    GLOBAL DEFAULT    1 main
     2: 0000000000002000     4 OBJECT  GLOBAL DEFAULT  ABS 
`
const wantSymbols32BE = `
Symbol table '.dynsym' contains 2 entries:
   Num:    Value  Size Type    Bind   Vis      Ndx Name
     0: 00000000     0 NOTYPE  LOCAL  DEFAULT  UND 
     1: 00001010    12 FUNC    GLOBAL DEFAULT   12 main

Symbol table '.symtab' contains 3 entries:
   Num:    Value  Size Type    Bind   Vis      Ndx Name
     0: 00000000     0 NOTYPE  LOCAL  DEFAULT  UND 
     1: 00001000    32 FUNC    GLOBAL DEFAULT    1 main
     2: 00002000     4 OBJECT  GLOBAL DEFAULT  ABS 
`
const wantDynSyms64LE = `
Symbol table '.dynsym' contains 2 entries:
   Num:            Value  Size Type    Bind   Vis      Ndx Name
     0: 0000000000000000     0 NOTYPE  LOCAL  DEFAULT  UND 
     1: 0000000000001010    12 FUNC    GLOBAL DEFAULT   12 main
`
const wantDynamic64LE = `
Dynamic section at offset 0x230 contains 7 entries:
  Tag                Type                 Name/Value
 0x0000000000000001 (NEEDED)             Shared library: [libc.so]
 0x000000000000000e (SONAME)             Library soname: [mylib.so]
 0x000000000000001c (FINI_ARRAYSZ)       16 (bytes)
 0x000000000000001e (FLAGS)              BIND_NOW
 0x000000006ffffffb (FLAGS_1)            Flags: NOW PIE
 0x0000000000000014 (PLTREL)             RELA
 0x0000000000000000 (NULL)               0x0
`
const wantDynamic32BE = `
Dynamic section at offset 0x180 contains 7 entries:
  Tag        Type                 Name/Value
 0x00000001 (NEEDED)             Shared library: [libc.so]
 0x0000000e (SONAME)             Library soname: [mylib.so]
 0x0000001c (FINI_ARRAYSZ)       16 (bytes)
 0x0000001e (FLAGS)              BIND_NOW
 0x6ffffffb (FLAGS_1)            Flags: NOW PIE
 0x00000014 (PLTREL)             RELA
 0x00000000 (NULL)               0x0
`
const wantStrings64LE = `
String dump of section '.shstrtab':
  [     2]  .interp
  [     a]  .text
  [    10]  .note.gnu.build-id
  [    23]  .dynstr
  [    2b]  .dynsym
  [    33]  .dynamic
  [    3c]  .symtab
  [    44]  .strtab
  [    4c]  .shstrtab

`
const wantHex64LE = `
Hex dump of section '.text':
  0x00000000 554889e5 b82a0000 005dc300 00000000 UH...*...]......
  0x00000010 90909090 90909090 00010203 7f414243 .............ABC

`
const wantRISCVAttrs = `
Attribute Section: riscv
File Attributes
  Tag_RISCV_arch: rv64i2p0
  Tag_RISCV_stack_align: 16
  Tag_RISCV_unaligned_access: 1
  Tag_RISCV_x3_reg_usage: 2
`

// --- tests ------------------------------------------------------------------

func readelfWrite(t *testing.T, dir, name string, data []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// readelfSectionLine returns the -S row for a section.
func readelfSectionLine(t *testing.T, out, name string) string {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, " "+name+" ") {
			return line
		}
	}
	t.Fatalf("no section row for %q in:\n%s", name, out)
	return ""
}

func TestReadelfHeaderSynthetic(t *testing.T) {
	cases := []struct {
		name string
		bits int
		be   bool
		want string
	}{
		{"elf32-le", 32, false, wantHdr32LE},
		{"elf32-be", 32, true, wantHdr32BE},
		{"elf64-le", 64, false, wantHdr64LE},
		{"elf64-be", 64, true, wantHdr64BE},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			data, _ := readelfStdFixture(c.bits, c.be)
			readelfWrite(t, dir, c.name, data)
			r := runBuiltinIn(t, dir, "readelf", "-h", c.name)
			assertExitCode(t, r, 0)
			assertStdout(t, r, c.want)
			if r.stderr != "" {
				t.Errorf("stderr = %q, want empty", r.stderr)
			}
			// -W is accepted and changes nothing.
			assertStdout(t, runBuiltinIn(t, dir, "readelf", "-hW", c.name), c.want)
			// -e implies -h -l -S.
			assertStdoutContains(t, runBuiltinIn(t, dir, "readelf", "-e", c.name), "ELF Header:")
		})
	}
}

func TestReadelfSectionHeaders(t *testing.T) {
	dir := t.TempDir()
	data, _ := readelfStdFixture(64, false)
	readelfWrite(t, dir, "elf64-le", data)
	r := runBuiltinIn(t, dir, "readelf", "-S", "elf64-le")
	assertExitCode(t, r, 0)
	assertStdout(t, r, wantSections64LE)

	data, _ = readelfStdFixture(32, true)
	readelfWrite(t, dir, "elf32-be", data)
	r = runBuiltinIn(t, dir, "readelf", "-SW", "elf32-be")
	assertExitCode(t, r, 0)
	assertStdout(t, r, wantSections32BE)

	// -h suppresses the "There are N section headers" line.
	r = runBuiltinIn(t, dir, "readelf", "-hS", "elf64-le")
	assertStdoutContains(t, r, "Section Headers:")
	if strings.Contains(r.stdout, "section headers, starting at offset") {
		t.Errorf("-hS should not print the section header count line:\n%s", r.stdout)
	}

	// shnum == 0 (and shstrndx 0, as in core files).
	data, l := readelfStdFixture(64, false)
	l.put16(data, 60, 0)
	l.put16(data, 62, 0)
	readelfWrite(t, dir, "nosections", data)
	r = runBuiltinIn(t, dir, "readelf", "-S", "nosections")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "\nThere are no sections in this file.\n")
	r = runBuiltinIn(t, dir, "readelf", "-h", "nosections")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "Number of section headers:         0")
}

func TestReadelfProgramHeaders(t *testing.T) {
	dir := t.TempDir()
	data, _ := readelfStdFixture(64, false)
	readelfWrite(t, dir, "elf64-le", data)
	r := runBuiltinIn(t, dir, "readelf", "-l", "elf64-le")
	assertExitCode(t, r, 0)
	assertStdout(t, r, wantPrograms64LE)

	data, _ = readelfStdFixture(32, true)
	readelfWrite(t, dir, "elf32-be", data)
	r = runBuiltinIn(t, dir, "readelf", "-lW", "elf32-be")
	assertExitCode(t, r, 0)
	assertStdout(t, r, wantPrograms32BE)

	// -h suppresses the file type / entry point preamble.
	r = runBuiltinIn(t, dir, "readelf", "-hl", "elf64-le")
	assertStdoutContains(t, r, "Program Headers:")
	if strings.Contains(r.stdout, "Elf file type is") {
		t.Errorf("-hl should not print the program header preamble")
	}

	// No program headers at all.
	data, l := readelfStdFixture(64, false)
	l.put16(data, 56, 0) // e_phnum
	readelfWrite(t, dir, "nophdr", data)
	r = runBuiltinIn(t, dir, "readelf", "-l", "nophdr")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "\nThere are no program headers in this file.\n")
}

func TestReadelfNotes(t *testing.T) {
	dir := t.TempDir()
	data, _ := readelfStdFixture(64, false)
	readelfWrite(t, dir, "elf64-le", data)
	r := runBuiltinIn(t, dir, "readelf", "-n", "elf64-le")
	assertExitCode(t, r, 0)
	assertStdout(t, r, wantNotes64LE)

	data, _ = readelfNotesFixture()
	readelfWrite(t, dir, "notes", data)
	r = runBuiltinIn(t, dir, "readelf", "-nW", "notes")
	assertExitCode(t, r, 0)
	assertStdout(t, r, wantNotesMulti)

	// With no SHT_NOTE section the notes come from PT_NOTE program headers.
	secs := []readelfTestSec{
		{name: "", typ: 0},
		{name: ".data", typ: 1, data: []byte("xxxx"), addralign: 1},
	}
	note := readelfNote("GNU", 3, []byte{0x11, 0x22, 0x33, 0x44}, false)
	phs := []readelfTestPh{{typ: 4, flags: 4, sec: -1, off: 0, filesz: uint64(len(note)), align: 4}}
	data, l := readelfBuildELF(readelfTestELF{bits: 64, machine: 183, etype: 3, secs: secs, phs: phs})
	// Place the note right after the section headers so the file stays valid.
	data = append(data, note...)
	phoff := uint64(l.ehsize)
	l.put64(data, phoff+8, uint64(len(data)-len(note))) // p_offset
	l.put64(data, phoff+32, uint64(len(note)))          // p_filesz
	readelfWrite(t, dir, "phnote", data)
	r = runBuiltinIn(t, dir, "readelf", "-n", "phnote")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "Displaying notes found at file offset 0x")
	assertStdoutContains(t, r, "NT_GNU_BUILD_ID\t11223344")

	// GNU ABI tag: the three version words print in field order. Toybox's
	// printf leaves its reader calls unsequenced, so it can scramble them.
	desc := append(readelfU32LE(0), readelfU32LE(3)...)
	desc = append(desc, readelfU32LE(2)...)
	desc = append(desc, readelfU32LE(0)...)
	secs = []readelfTestSec{
		{name: "", typ: 0},
		{name: ".note.abi", typ: 7, flags: 2, data: readelfNote("GNU", 1, desc, false), addralign: 4},
	}
	data, _ = readelfBuildELF(readelfTestELF{bits: 64, machine: 183, etype: 3, secs: secs})
	readelfWrite(t, dir, "abi", data)
	r = runBuiltinIn(t, dir, "readelf", "-n", "abi")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "NT_GNU_ABI_TAG\tOS: Linux, ABI: 3.2.0")
}

func TestReadelfSymbols(t *testing.T) {
	dir := t.TempDir()
	data, _ := readelfStdFixture(64, false)
	readelfWrite(t, dir, "elf64-le", data)
	r := runBuiltinIn(t, dir, "readelf", "-s", "elf64-le")
	assertExitCode(t, r, 0)
	assertStdout(t, r, wantSymbols64LE)

	r = runBuiltinIn(t, dir, "readelf", "--dyn-syms", "elf64-le")
	assertExitCode(t, r, 0)
	assertStdout(t, r, wantDynSyms64LE)

	data, _ = readelfStdFixture(32, true)
	readelfWrite(t, dir, "elf32-be", data)
	r = runBuiltinIn(t, dir, "readelf", "-sW", "elf32-be")
	assertExitCode(t, r, 0)
	assertStdout(t, r, wantSymbols32BE)

	// -s implies --dyn-syms; --dyn-syms alone dumps just .dynsym.
	r = runBuiltinIn(t, dir, "readelf", "--dyn-syms", "elf32-be")
	assertExitCode(t, r, 0)
	if strings.Contains(r.stdout, ".symtab") {
		t.Errorf("--dyn-syms dumped .symtab too:\n%s", r.stdout)
	}

	// A symbol name offset past the end of the file shows "???".
	data, l := readelfStdFixture(64, false)
	l.put32(data, l.secOff[7]+24, 0xffffffff) // st_name of .symtab[1]
	readelfWrite(t, dir, "badsym", data)
	r = runBuiltinIn(t, dir, "readelf", "-s", "badsym")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "???")
}

func TestReadelfDynamic(t *testing.T) {
	dir := t.TempDir()
	data, _ := readelfStdFixture(64, false)
	readelfWrite(t, dir, "elf64-le", data)
	r := runBuiltinIn(t, dir, "readelf", "-d", "elf64-le")
	assertExitCode(t, r, 0)
	assertStdout(t, r, wantDynamic64LE)

	data, _ = readelfStdFixture(32, true)
	readelfWrite(t, dir, "elf32-be", data)
	r = runBuiltinIn(t, dir, "readelf", "-dW", "elf32-be")
	assertExitCode(t, r, 0)
	assertStdout(t, r, wantDynamic32BE)

	// No SHT_DYNAMIC section.
	data, l := readelfStdFixture(64, false)
	l.put32(data, l.shoff+6*uint64(l.shentsize)+4, 1) // .dynamic type -> PROGBITS
	readelfWrite(t, dir, "nodyn", data)
	r = runBuiltinIn(t, dir, "readelf", "-d", "nodyn")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "\nThere is no dynamic section in this file.\n")

	// entsize 0.
	data, l = readelfStdFixture(64, false)
	l.put64(data, l.shoff+6*uint64(l.shentsize)+56, 0) // .dynamic sh_entsize
	readelfWrite(t, dir, "zerodyn", data)
	r = runBuiltinIn(t, dir, "readelf", "-d", "zerodyn")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "\nBad dynamic entry size 0!\n")

	// Dangling string offsets print "???" instead of reading past the end.
	data, l = readelfStdFixture(64, false)
	l.put64(data, l.secOff[6]+8, 0x10000) // NEEDED value
	readelfWrite(t, dir, "dangling", data)
	r = runBuiltinIn(t, dir, "readelf", "-d", "dangling")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "Shared library: [???]")

	// Tags whose value is a plain address print hex, never "(bytes)":
	// toybox maps ANDROID_RELRENT to 'x', so the value falls through to hex.
	data, l = readelfStdFixture(64, false)
	l.put64(data, l.secOff[6]+2*16, 0x6fffe003) // entry 2 tag -> ANDROID_RELRENT
	l.put64(data, l.secOff[6]+2*16+8, 4)        // value
	readelfWrite(t, dir, "android", data)
	r = runBuiltinIn(t, dir, "readelf", "-d", "android")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "(ANDROID_RELRENT)    0x4")
}

func TestReadelfStringDump(t *testing.T) {
	dir := t.TempDir()
	data, _ := readelfStdFixture(64, false)
	readelfWrite(t, dir, "elf64-le", data)
	r := runBuiltinIn(t, dir, "readelf", "-p", ".shstrtab", "elf64-le")
	assertExitCode(t, r, 0)
	assertStdout(t, r, wantStrings64LE)

	// A section with nothing printable.
	secs := []readelfTestSec{
		{name: "", typ: 0},
		{name: ".bin", typ: 1, data: []byte{0, 1, 2, 3, 4, 5, 6, 7}, addralign: 1},
	}
	data, _ = readelfBuildELF(readelfTestELF{bits: 64, machine: 183, etype: 2, secs: secs})
	readelfWrite(t, dir, "bin", data)
	r = runBuiltinIn(t, dir, "readelf", "-p", ".bin", "bin")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "\nString dump of section '.bin':\n  No strings found in this section.\n\n")

	// A missing section name.
	r = runBuiltinIn(t, dir, "readelf", "-p", "nope", "bin")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "no section 'nope")
}

func TestReadelfHexDump(t *testing.T) {
	dir := t.TempDir()
	data, _ := readelfStdFixture(64, false)
	readelfWrite(t, dir, "elf64-le", data)
	r := runBuiltinIn(t, dir, "readelf", "-x", ".text", "elf64-le")
	assertExitCode(t, r, 0)
	assertStdout(t, r, wantHex64LE)

	// A partial final row is padded to the text column.
	secs := []readelfTestSec{
		{name: "", typ: 0},
		{name: ".bin", typ: 1, data: []byte{0, 1, 2, 3, 4, 5, 6, 7}, addralign: 1},
	}
	data, _ = readelfBuildELF(readelfTestELF{bits: 64, machine: 183, etype: 2, secs: secs})
	readelfWrite(t, dir, "bin", data)
	r = runBuiltinIn(t, dir, "readelf", "-x", ".bin", "bin")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "\nHex dump of section '.bin':\n"+
		"  0x00000000 00010203 04050607                   ........\n\n")
}

func TestReadelfSectionSpec(t *testing.T) {
	dir := t.TempDir()
	data, _ := readelfManySections(22)
	readelfWrite(t, dir, "many", data)

	base := runBuiltinIn(t, dir, "readelf", "-p22", "many")
	assertExitCode(t, base, 0)
	assertStdout(t, base, "\nString dump of section '.s21':\n  [     0]  data21\n\n")

	// -p22 and -p 22, decimal, hex and octal.
	for _, spec := range []string{"22", "0x16", "026"} {
		r := runBuiltinIn(t, dir, "readelf", "-p", spec, "many")
		assertExitCode(t, r, 0)
		assertStdout(t, r, base.stdout)
	}
	// ... and by name.
	assertStdout(t, runBuiltinIn(t, dir, "readelf", "-p", ".s21", "many"), base.stdout)

	r := runBuiltinIn(t, dir, "readelf", "-x22", "many")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "Hex dump of section '.s21':")

	// Out of range numbers fall back to a name lookup.
	r = runBuiltinIn(t, dir, "readelf", "-p", "9999", "many")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "no section")
	r = runBuiltinIn(t, dir, "readelf", "-x", "0xzz", "many")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "no section")
}

func TestReadelfSectionFlags(t *testing.T) {
	cases := []struct {
		machine uint16
		flags   uint64
		want    string
	}{
		{readelfEMARM, 6, "AX"},
		{readelfEMARM, 6 | 1<<29, "AXy"},
		{readelfEMARM64, 6 | 1<<29, "AXy"},
		{readelfEMX8664, 6 | 1<<28, "AXl"},
		{readelfEMX8664, 6, "AX"},
		{8, 6 | 1<<29 | 1<<28, "AXx"},
		{readelfEMARM, 6 | 1<<30, "AXx"},
		{readelfEMARM64, 0xf0000006, "AXyx"},
	}
	for _, c := range cases {
		secs := []readelfTestSec{
			{name: "", typ: 0},
			{name: ".text", typ: 1, flags: c.flags, data: []byte("x"), addralign: 1},
		}
		data, _ := readelfBuildELF(readelfTestELF{bits: 64, machine: c.machine, etype: 2, secs: secs})
		dir := t.TempDir()
		readelfWrite(t, dir, "f", data)
		r := runBuiltinIn(t, dir, "readelf", "-S", "f")
		assertExitCode(t, r, 0)
		line := readelfSectionLine(t, r.stdout, ".text")
		if !strings.Contains(line, " "+c.want+" ") && !strings.HasSuffix(strings.TrimSpace(line), c.want) {
			t.Errorf("flags %#x machine %d: row %q does not contain %q", c.flags, c.machine, line, c.want)
		}
	}
}

func TestReadelfRISCVAttributes(t *testing.T) {
	dir := t.TempDir()
	data, _ := readelfRISCVFixture(1)
	readelfWrite(t, dir, "riscv", data)
	r := runBuiltinIn(t, dir, "readelf", "-A", "riscv")
	assertExitCode(t, r, 0)
	assertStdout(t, r, wantRISCVAttrs)

	// -a includes -A.
	r = runBuiltinIn(t, dir, "readelf", "-a", "riscv")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "Attribute Section: riscv")

	// An unknown sub-subsection tag is an error.
	data, _ = readelfRISCVFixture(3)
	readelfWrite(t, dir, "riscvbad", data)
	r = runBuiltinIn(t, dir, "readelf", "-A", "riscvbad")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "unknown attributes tag=3")

	// -A on a non-RISC-V file prints nothing.
	data, _ = readelfStdFixture(64, false)
	readelfWrite(t, dir, "arm64", data)
	r = runBuiltinIn(t, dir, "readelf", "-A", "arm64")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "")
}

func TestReadelfAllFlags(t *testing.T) {
	dir := t.TempDir()
	data, _ := readelfStdFixture(64, false)
	readelfWrite(t, dir, "elf64-le", data)
	r := runBuiltinIn(t, dir, "readelf", "-a", "elf64-le")
	assertExitCode(t, r, 0)
	for _, want := range []string{
		"ELF Header:", "Section Headers:", "Program Headers:",
		"Dynamic section at offset 0x230", "Symbol table '.dynsym'",
		"Symbol table '.symtab'", "Displaying notes found in: .note.gnu.build-id",
	} {
		assertStdoutContains(t, r, want)
	}
	r = runBuiltinIn(t, dir, "readelf", "-e", "elf64-le")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "Section Headers:")
	assertStdoutContains(t, r, "Program Headers:")
	if strings.Contains(r.stdout, "Dynamic section") {
		t.Errorf("-e should not dump the dynamic section")
	}
}

func TestReadelfTruncation(t *testing.T) {
	dir := t.TempDir()
	full, _ := readelfStdFixture(64, false)
	flags := [][]string{
		{"-h"}, {"-S"}, {"-l"}, {"-n"}, {"-s"}, {"-d"},
		{"-p", ".shstrtab"}, {"-x", ".text"}, {"-a"},
	}
	for n := 0; n <= len(full); n++ {
		readelfWrite(t, dir, "trunc", full[:n])
		for _, fl := range flags {
			args := append(append([]string{}, fl...), "trunc")
			r := runBuiltinIn(t, dir, "readelf", args...)
			if r.code != 0 && r.code != 1 {
				t.Fatalf("prefix %d %v: exit %d", n, fl, r.code)
			}
			if r.code == 1 && r.stderr == "" {
				t.Fatalf("prefix %d %v: exit 1 without a message (stdout %q)", n, fl, r.stdout)
			}
		}
	}

	// Same for a big-endian 32-bit file.
	full, _ = readelfStdFixture(32, true)
	for n := 0; n <= len(full); n += 3 {
		readelfWrite(t, dir, "trunc32", full[:n])
		for _, fl := range flags {
			args := append(append([]string{}, fl...), "trunc32")
			r := runBuiltinIn(t, dir, "readelf", args...)
			if r.code != 0 && r.code != 1 {
				t.Fatalf("prefix %d %v: exit %d", n, fl, r.code)
			}
			if r.code == 1 && r.stderr == "" {
				t.Fatalf("prefix %d %v: exit 1 without a message", n, fl)
			}
		}
	}
}

func TestReadelfHostileHeaders(t *testing.T) {
	type patch struct {
		name  string
		off   uint64
		size  int
		value uint64
		flag  string
		want  string
	}
	cases := []patch{
		{"shnum", 60, 2, 0xffff, "-S", "No shdr"},
		{"shoff", 40, 8, 0xffffffff, "-h", "bad shoff"},
		{"phnum", 56, 2, 0xffff, "-l", "Bad phdr"},
		{"phentsize", 54, 2, 0, "-l", "bad phentsize"},
		{"shentsize", 58, 2, 0, "-S", "bad shentsize"},
		{"shstrndx", 62, 2, 0xffff, "-h", "bad shstrndx"},
		{"class0", 4, 1, 0, "-h", "bad ELF"},
		{"class3", 4, 1, 3, "-h", "bad ELF"},
		{"endian0", 5, 1, 0, "-h", "bad ELF"},
		{"endian3", 5, 1, 3, "-h", "bad ELF"},
		{"version0", 6, 1, 0, "-h", "bad ELF"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			data, l := readelfStdFixture(64, false)
			switch c.size {
			case 1:
				data[c.off] = byte(c.value)
			case 2:
				l.put16(data, c.off, uint16(c.value))
			case 4:
				l.put32(data, c.off, uint32(c.value))
			case 8:
				l.put64(data, c.off, c.value)
			}
			readelfWrite(t, dir, "hostile", data)
			r := runBuiltinIn(t, dir, "readelf", c.flag, "hostile")
			assertExitCode(t, r, 1)
			if !strings.Contains(r.stderr, c.want) {
				t.Errorf("stderr = %q, want it to contain %q", r.stderr, c.want)
			}
		})
	}

	// A 32-bit header with a bogus section table is caught too.
	data, l := readelfStdFixture(32, true)
	l.put16(data, 48, 0xffff) // e_shnum
	dir := t.TempDir()
	readelfWrite(t, dir, "hostile32", data)
	r := runBuiltinIn(t, dir, "readelf", "-S", "hostile32")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "No shdr")
}

// runReadelfNamed runs the builtin the way the shell dispatcher does, with
// CmdName set (which --help prints).
func runReadelfNamed(t *testing.T, dir string, args ...string) runResult {
	t.Helper()
	cmd, ok := Lookup("readelf")
	if !ok {
		t.Fatal("builtin \"readelf\" not registered")
	}
	var stdout, stderr bytes.Buffer
	ctx := &Context{
		Dir: dir, Args: args, CmdName: "readelf",
		Width: 80, Height: 24,
		Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader(""),
	}
	code := cmd.Run(ctx)
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func TestReadelfCLI(t *testing.T) {
	dir := t.TempDir()
	elf, _ := readelfStdFixture(64, false)
	readelfWrite(t, dir, "a.elf", elf)
	readelfWrite(t, dir, "b.elf", elf)
	writeFile(t, dir, "text", "not an elf\n")
	writeFile(t, dir, "empty", "")
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	r := runBuiltinIn(t, dir, "readelf", "-h")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "missing operand")

	r = runBuiltinIn(t, dir, "readelf", "a.elf")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "needs a flag")

	r = runBuiltinIn(t, dir, "readelf", "-h", "nope.elf")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "readelf: nope.elf: No such file or directory")

	r = runBuiltinIn(t, dir, "readelf", "-h", "empty")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "readelf: empty: empty")

	r = runBuiltinIn(t, dir, "readelf", "-h", "sub")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "not a regular file")

	r = runBuiltinIn(t, dir, "readelf", "-h", "text")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "not ELF")

	// Two operands get "File:" headers.
	r = runBuiltinIn(t, dir, "readelf", "-h", "a.elf", "b.elf")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "\nFile: a.elf\n")
	assertStdoutContains(t, r, "\nFile: b.elf\n")
	if strings.Contains(r.stdout, "File: ") && strings.Count(r.stdout, "\nFile: ") != 2 {
		t.Errorf("expected two File: headers:\n%s", r.stdout)
	}

	// One failing operand fails the run but the other still prints.
	r = runBuiltinIn(t, dir, "readelf", "-h", "a.elf", "nope.elf")
	assertExitCode(t, r, 1)
	assertStdoutContains(t, r, "\nFile: a.elf\n")
	assertStderrContains(t, r, "No such file or directory")

	r = runReadelfNamed(t, dir, "--help")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "Usage: readelf")

	r = runReadelfNamed(t, dir, "--version")
	assertExitCode(t, r, 0)

	r = runBuiltinIn(t, dir, "readelf", "-Z", "a.elf")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "unknown option")

	// A relative path resolves through the builtin's working directory.
	sub := t.TempDir()
	readelfWrite(t, sub, "rel.elf", elf)
	r = runBuiltinIn(t, sub, "readelf", "-h", "rel.elf")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "ELF Header:")
}

func TestReadelfNotELF(t *testing.T) {
	dir := t.TempDir()
	// 45+ bytes of non-ELF data.
	writeFile(t, dir, "big", strings.Repeat("x", 100))
	r := runBuiltinIn(t, dir, "readelf", "-h", "big")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "not ELF")

	// Short file.
	writeFile(t, dir, "short", "abc")
	r = runBuiltinIn(t, dir, "readelf", "-h", "short")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "not ELF")

	// ELF magic but a truncated 64-bit header.
	writeFile(t, dir, "cut", "\x7fELF\x02\x01\x01\x00"+strings.Repeat("\x00", 40))
	r = runBuiltinIn(t, dir, "readelf", "-h", "cut")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "Access off end")
}
