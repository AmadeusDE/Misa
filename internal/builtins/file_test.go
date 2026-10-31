package builtins

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

// fileRun runs the file builtin with CmdName set, so --help prints the command
// name, and with the given stdin.
func fileRun(t *testing.T, dir, stdin string, args ...string) runResult {
	t.Helper()
	cmd, ok := Lookup("file")
	if !ok {
		t.Fatal("builtin \"file\" not registered")
	}
	var stdout, stderr bytes.Buffer
	ctx := &Context{
		Dir:     dir,
		Args:    args,
		CmdName: "file",
		Width:   80,
		Height:  24,
		Stdout:  &stdout,
		Stderr:  &stderr,
		Stdin:   strings.NewReader(stdin),
	}
	code := cmd.Run(ctx)
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// fileRunBytes is fileRun with binary stdin.
func fileRunBytes(t *testing.T, dir string, stdin []byte, args ...string) runResult {
	t.Helper()
	cmd, ok := Lookup("file")
	if !ok {
		t.Fatal("builtin \"file\" not registered")
	}
	var stdout, stderr bytes.Buffer
	ctx := &Context{
		Dir:     dir,
		Args:    args,
		CmdName: "file",
		Width:   80,
		Height:  24,
		Stdout:  &stdout,
		Stderr:  &stderr,
		Stdin:   bytes.NewReader(stdin),
	}
	code := cmd.Run(ctx)
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func fileWriteBytes(t *testing.T, dir, name string, data []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// fileTarHeader builds a 512-byte tar header with a valid checksum. An empty
// magic leaves the ustar field empty, which is an "old" tar header.
func fileTarHeader(name, magic string) []byte {
	h := make([]byte, 512)
	copy(h[0:], name)
	copy(h[100:], "0000644\x00")
	copy(h[108:], "0000000\x00")
	copy(h[116:], "0000000\x00")
	copy(h[124:], "00000000000\x00")
	copy(h[136:], "00000000000\x00")
	copy(h[148:], "        ")
	h[156] = '0'
	copy(h[257:], magic)
	// Classic checksum: the checksum field reads as spaces.
	sum := int(8 * ' ')
	for i := 0; i < 512; i++ {
		if i >= 148 && i < 156 {
			continue
		}
		sum += int(h[i])
	}
	copy(h[148:], fmt.Sprintf("%06o\x00 ", sum))
	return h
}

// fileAppend16/32/64 append an integer in the given byte order.
func fileAppend16(order binary.ByteOrder, b []byte, v uint16) []byte {
	var tmp [2]byte
	order.PutUint16(tmp[:], v)
	return append(b, tmp[:]...)
}

func fileAppend32(order binary.ByteOrder, b []byte, v uint32) []byte {
	var tmp [4]byte
	order.PutUint32(tmp[:], v)
	return append(b, tmp[:]...)
}

func fileAppend64(order binary.ByteOrder, b []byte, v uint64) []byte {
	var tmp [8]byte
	order.PutUint64(tmp[:], v)
	return append(b, tmp[:]...)
}

func filePNG(width, height uint32, depth, colour, interlace byte) []byte {
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], width)
	binary.BigEndian.PutUint32(ihdr[4:], height)
	ihdr[8], ihdr[9], ihdr[12] = depth, colour, interlace
	out := []byte("\x89PNG\r\n\x1a\n")
	out = binary.BigEndian.AppendUint32(out, 13)
	out = append(out, "IHDR"...)
	out = append(out, ihdr...)
	return append(out, 0, 0, 0, 0) // CRC
}

func fileGIF(version string, width, height uint16) []byte {
	out := []byte("GIF" + version)
	out = binary.LittleEndian.AppendUint16(out, width)
	out = binary.LittleEndian.AppendUint16(out, height)
	return append(out, make([]byte, 20)...)
}

// fileJPEG builds a JPEG with one APP0 segment followed by an SOFn marker, which
// is what the marker scanner walks.
func fileJPEG(marker, width, height uint16) []byte {
	out := []byte("\xff\xd8")            // SOI
	out = append(out, 0xff, 0xe0, 0, 16) // APP0, 16-byte segment
	out = append(out, "JFIF\x00"...)
	out = append(out, make([]byte, 9)...)
	out = binary.BigEndian.AppendUint16(out, marker)
	out = binary.BigEndian.AppendUint16(out, 17)
	out = append(out, 8) // sample precision
	out = binary.BigEndian.AppendUint16(out, height)
	out = binary.BigEndian.AppendUint16(out, width)
	return append(out, make([]byte, 40)...)
}

func fileWAV(bigEndian bool, format, channels uint16, hz uint32, bits uint16) []byte {
	order := binary.ByteOrder(binary.LittleEndian)
	riff := "RIFF"
	if bigEndian {
		order, riff = binary.BigEndian, "RIFX"
	}
	out := []byte(riff)
	out = fileAppend32(order, out, 36)
	out = append(out, "WAVEfmt "...)
	out = fileAppend32(order, out, 16)
	out = fileAppend16(order, out, format)
	out = fileAppend16(order, out, channels)
	out = fileAppend32(order, out, hz)
	out = fileAppend32(order, out, 0)
	out = fileAppend16(order, out, 0)
	out = fileAppend16(order, out, bits)
	return append(out, make([]byte, 8)...)
}

func fileOgg(tag string) []byte {
	out := []byte("OggS\x00\x02")
	out = append(out, make([]byte, 22)...)
	out = append(out, tag...)
	return append(out, make([]byte, 8)...)
}

func fileMachOUniversal(arches ...uint32) []byte {
	out := []byte("\xca\xfe\xba\xbe")
	out = binary.BigEndian.AppendUint32(out, uint32(len(arches)))
	for _, arch := range arches {
		out = binary.BigEndian.AppendUint32(out, arch)
		out = append(out, make([]byte, 16)...)
	}
	return out
}

func fileJavaClass(major, minor uint16) []byte {
	out := []byte("\xca\xfe\xba\xbe")
	out = binary.BigEndian.AppendUint16(out, minor)
	out = binary.BigEndian.AppendUint16(out, major)
	// The magic needs more than 8 bytes to be considered at all.
	return append(out, 0, 13, 0x0a, 0x00)
}

func fileBMP(width, height uint32, bpp uint16) []byte {
	out := make([]byte, 0x40)
	copy(out, "BM")
	binary.LittleEndian.PutUint32(out[18:], width)
	binary.LittleEndian.PutUint32(out[22:], height)
	binary.LittleEndian.PutUint16(out[28:], bpp)
	return out
}

func filePE(arch, subsystem, magic, osType uint16) []byte {
	out := make([]byte, 0x400)
	copy(out, "MZ")
	binary.LittleEndian.PutUint32(out[0x3c:], 0x80)
	copy(out[0x80:], "PE\x00\x00")
	binary.LittleEndian.PutUint16(out[0x80+4:], arch)
	binary.LittleEndian.PutUint16(out[0x80+20:], 0x70+1)
	binary.LittleEndian.PutUint16(out[0x80+22:], subsystem)
	binary.LittleEndian.PutUint16(out[0x80+24:], magic)
	binary.LittleEndian.PutUint16(out[0x80+92:], osType)
	return out
}

func TestFileMagic(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
		want string
	}{
		{"ar", []byte("!<arch>\nhello/          0           0     0     644     6         `\nworld\n"), "ar archive"},
		{"gzip", append([]byte("\x1f\x8b\x08\x00"), make([]byte, 10)...), "gzip compressed data"},
		{"bzip2", append([]byte("BZh9"), make([]byte, 20)...), "bzip2 compressed data, block size = 900k"},
		{"xz", append([]byte("\xfd7zXZ\x00"), make([]byte, 30)...), "xz compressed data"},
		{"zstd", append([]byte("\x28\xb5\x2f\xfd"), make([]byte, 20)...), "zstd compressed data"},
		{"zip", append([]byte("PK\x03\x04\x0a\x00\x00\x00"), make([]byte, 20)...), "Zip archive data, requires at least v1.0 to extract"},
		{"zip-no-version", append([]byte("PK\x03\x04\x00\x00\x00\x00"), make([]byte, 20)...), "Zip archive data"},
		{"7z", append([]byte("7z\xbc\xaf\x27\x1c\x00\x04"), make([]byte, 20)...), "7-zip archive data, version 0.4"},
		{"cpio-svr4", []byte("07070103344745000081A4000003E800"), "ASCII cpio archive (SVR4 with no CRC)"},
		{"cpio-crc", append([]byte("070702"), bytes.Repeat([]byte("0"), 30)...), "ASCII cpio archive (SVR4 with CRC)"},
		{"cpio-odc", append([]byte("070707"), bytes.Repeat([]byte("0"), 30)...), "ASCII cpio archive (pre-SVR4 or odc)"},
		{"cpio-binary", append([]byte{0xc7, 0x71}, make([]byte, 40)...), "cpio archive"},
		{"cpio-binary-swapped", append([]byte{0x71, 0xc7}, make([]byte, 40)...), "byte-swapped cpio archive"},
		{"tar-ustar-gnu", append(fileTarHeader("hello.txt", "ustar  \x00"), make([]byte, 512)...), "POSIX tar archive (GNU)"},
		{"tar-ustar", append(fileTarHeader("hello.txt", "ustar\x00x"), make([]byte, 512)...), "POSIX tar archive"},
		{"tar-old", append(fileTarHeader("hello.txt", ""), make([]byte, 512)...), "old tar archive"},
		{"pdf", []byte("%PDF-1.7\n"), "PDF document, version 1.7"},
		{"png-rgb", filePNG(8, 6, 8, 2, 0), "PNG image data, 8 x 6, 8-bit/color RGB, non-interlaced"},
		{"png-gray-alpha", filePNG(4, 3, 16, 4, 1), "PNG image data, 4 x 3, 16-bit/grayscale with alpha, interlaced"},
		{"png-indexed", filePNG(2, 2, 1, 3, 0), "PNG image data, 2 x 2, 1-bit/indexed color, non-interlaced"},
		{"png-unknown-colour", filePNG(2, 2, 8, 1, 0), "PNG image data, 2 x 2, 8-bit/unknown, non-interlaced"},
		{"gif89a", fileGIF("89a", 32, 32), "GIF image data, version 89a, 32 x 32"},
		{"gif87a", fileGIF("87a", 10, 20), "GIF image data, version 87a, 10 x 20"},
		{"jpeg-baseline", fileJPEG(0xffc0, 100, 80), "JPEG image data, baseline, 100x80"},
		{"jpeg-progressive", fileJPEG(0xffc2, 640, 480), "JPEG image data, progressive, 640x480"},
		{"jpeg-extended", fileJPEG(0xffc1, 7, 9), "JPEG image data, extended sequential, 7x9"},
		{"java-class", fileJavaClass(53, 0), "Java class file, version 53.0 (Java 1.9)"},
		{"mach-o-universal", fileMachOUniversal(0x01000007, 0x0100000c), "Mach-O universal binary with 2 architectures: [x86_64] [arm64]"},
		{"mach-o-universal-one", fileMachOUniversal(0x00000007), "Mach-O universal binary with 1 architecture: [i386]"},
		{"mach-o-universal-unknown", fileMachOUniversal(0x0000000c, 0x12345678), "Mach-O universal binary with 2 architectures: [arm] [unknown]"},
		{"dex", append([]byte("dex\n035\x00"), make([]byte, 20)...), "Android dex file, version 035"},
		{"wasm", append([]byte("\x00asm\x01\x00\x00\x00"), make([]byte, 8)...), "wasm binary module version 1"},
		{"bitcode", append([]byte("BC\xc0\xde"), make([]byte, 8)...), "LLVM IR bitcode"},
		{"pem", []byte("-----BEGIN CERTIFICATE-----\nMIIB\n"), "PEM certificate"},
		{"bmp", fileBMP(100, 50, 24), "BMP image, 100 x 50, 24 bpp"},
		{"truetype", append([]byte("\x00\x01\x00\x00"), make([]byte, 20)...), "TrueType font"},
		{"truetype-collection", append([]byte("ttcf\x00\x01\x00\x00\x00\x00\x00\x03"), make([]byte, 10)...), "TrueType font collection, version 1, 3 fonts"},
		{"opentype", append([]byte("OTTO"), make([]byte, 20)...), "OpenType font"},
		{"ogg-opus", fileOgg("OpusHead"), "Ogg data, opus audio"},
		{"ogg-vorbis", fileOgg("\x01vorbis"), "Ogg data, vorbis audio"},
		{"ogg-theora", fileOgg("\x80theora"), "Ogg data, theora video"},
		{"ogg-kate", fileOgg("\x80kate\x00\x00\x00"), "Ogg data, kate text"},
		{"ogg-dirac", fileOgg("BBCD\x00"), "Ogg data, dirac video"},
		{"ogg-flac", fileOgg("\x7fFLAC\x00\x00"), "Ogg data, flac audio"},
		{"ogg-unknown", fileOgg("\x00\x00\x00\x00\x00\x00\x00\x00"), "Ogg data"},
		{"wav-le", fileWAV(false, 1, 2, 44100, 16), "WAV audio, LE, 16-bit, stereo, 44100 Hz, PCM"},
		{"wav-be-float", fileWAV(true, 3, 1, 48000, 32), "WAV audio, BE, 32-bit, mono, 48000 Hz, IEEE float"},
		{"wav-multichannel", fileWAV(false, 0xfffe, 6, 96000, 24), "WAV audio, LE, 24-bit, 6-channel, 96000 Hz, extensible"},
		{"wav-unknown-format", fileWAV(false, 0x99, 2, 8000, 8), "WAV audio, LE, 8-bit, stereo, 8000 Hz, unknown format 153"},
		{"perf", append([]byte("PERFILE2"), make([]byte, 100)...), "Linux perf data"},
		{"sparse", func() []byte {
			b := make([]byte, 40)
			binary.LittleEndian.PutUint32(b[0:], 0xed26ff3a)
			binary.LittleEndian.PutUint16(b[4:], 1)
			binary.LittleEndian.PutUint16(b[6:], 0)
			binary.LittleEndian.PutUint32(b[12:], 4096)
			binary.LittleEndian.PutUint32(b[16:], 100)
			binary.LittleEndian.PutUint32(b[20:], 7)
			return b
		}(), "Android sparse image v1.0, 100 4096-byte blocks (7 chunks)"},
		{"boot", func() []byte {
			b := make([]byte, 2048)
			copy(b, "ANDROID!")
			binary.LittleEndian.PutUint32(b[40:], 2)
			return b
		}(), "Android boot image v2"},
		{"dtbo", func() []byte {
			b := make([]byte, 40)
			binary.BigEndian.PutUint32(b[0:], 0xd7b7ab1e)
			binary.BigEndian.PutUint32(b[16:], 5)
			binary.BigEndian.PutUint32(b[28:], 3)
			return b
		}(), "Android DTB/DTBO v3, 5 entries"},
		{"abx", append([]byte("ABX\x03"), make([]byte, 10)...), "Android Binary XML v3"},
		{"pe32", filePE(0x14c, 0x2000, 0x10b, 3), "MS PE32 executable (DLL) (console) x86"},
		{"pe32+", filePE(0x8664, 0, 0x20b, 2), "MS PE32+ executable (GUI) x86-64"},
		{"pe32-no-subsystem", func() []byte {
			b := filePE(0x14c, 0, 0x10b, 3)
			binary.LittleEndian.PutUint16(b[0x80+20:], 0x40)
			return b
		}(), "MS PE32 executable x86"},
		{"arm64-kernel", func() []byte {
			b := filePE(0xaa64, 0, 0x20b, 3)
			copy(b[0x38:], "ARMd")
			return b
		}(), "Linux arm64 kernel image"},
		{"x86-kernel", func() []byte {
			b := filePE(0x14c, 0, 0x10b, 3)
			copy(b[0x202:], "HdrS")
			binary.LittleEndian.PutUint16(b[0x20e:], 0x40)
			copy(b[0x240:], "5.10.0-test")
			return b
		}(), "Linux x86-64 kernel image, version 5.10.0-test"},
		{"macho-64-x86", func() []byte {
			b := make([]byte, 64)
			copy(b, "\xcf\xfa\xed\xfe")
			binary.LittleEndian.PutUint32(b[4:], 7)
			binary.LittleEndian.PutUint32(b[12:], 2)
			return b
		}(), "Mach-O 64-bit x86-64 executable"},
		{"macho-32-arm-object", func() []byte {
			b := make([]byte, 64)
			copy(b, "\xce\xfa\xed\xfe")
			binary.LittleEndian.PutUint32(b[4:], 12)
			binary.LittleEndian.PutUint32(b[12:], 1)
			return b
		}(), "Mach-O 32-bit arm object"},
		{"macho-bad-arch-type", func() []byte {
			b := make([]byte, 64)
			copy(b, "\xcf\xfa\xed\xfe")
			binary.LittleEndian.PutUint32(b[4:], 99)
			b[9] = 9
			b[12] = 9
			return b
		}(), "Mach-O 64-bit (bad arch 99) (bad type 9)"},
		{"ascii", []byte("Hello, world!\n"), "ASCII text"},
		{"utf8", []byte("日本語\n"), "UTF-8 text"},
		{"utf8-combining", []byte("cafe\u0301\n"), "UTF-8 text"},
		{"data-nul", []byte("ab\x00cd"), "data"},
		{"data-invalid", []byte{0xff, 0xfe, 0x00}, "data"},
		{"data-control", []byte{0x01, 0x02, 0x03}, "data"},
		{"script", []byte("#!/bin/bash\n"), "/bin/bash script"},
		{"script-spaces", []byte("#!   /bin/bash\n"), "/bin/bash script"},
		{"script-env", []byte("#! /usr/bin/env python\n"), "python script"},
		{"script-env-args", []byte("#!/usr/bin/env  perl -w\n"), "perl script"},
		{"empty", nil, "empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			fileWriteBytes(t, dir, tc.name, tc.data)
			r := fileRun(t, dir, "", "-b", tc.name)
			assertExitCode(t, r, 0)
			assertStdout(t, r, tc.want+"\n")
			if r.stderr != "" {
				t.Errorf("stderr = %q, want empty", r.stderr)
			}
		})
	}
}

// fileELFSpec describes a synthetic ELF file.
type fileELFSpec struct {
	class   byte   // 1 = 32-bit, 2 = 64-bit
	data    byte   // 1 = LSB, 2 = MSB
	etype   uint16 // 1 = relocatable, 2 = executable, 3 = shared object, 4 = core
	machine uint16
	flags   uint32
	interp  string // PT_INTERP string, empty for a static binary
	dynamic bool   // PT_DYNAMIC
	symtab  bool   // SHT_SYMTAB, which makes it "not stripped"
	notes   []byte // SHT_NOTE contents
}

func fileELFNote(name string, ntype uint32, desc []byte, bigEndian bool) []byte {
	order := binary.ByteOrder(binary.LittleEndian)
	if bigEndian {
		order = binary.BigEndian
	}
	pad := func(b []byte) []byte {
		for len(b)%4 != 0 {
			b = append(b, 0)
		}
		return b
	}
	out := fileAppend32(order, nil, uint32(len(name)))
	out = fileAppend32(order, out, uint32(len(desc)))
	out = fileAppend32(order, out, ntype)
	out = pad(append(out, name...))
	return pad(append(out, desc...))
}

// fileELFBytes lays out an ELF header, program headers, section contents and
// section headers so that every field the port reads is consistent.
func fileELFBytes(spec fileELFSpec) []byte {
	is64 := spec.class == 2
	bigEndian := spec.data == 2
	bits := 0
	if is64 {
		bits = 1
	}
	order := binary.ByteOrder(binary.LittleEndian)
	if bigEndian {
		order = binary.BigEndian
	}
	put16 := func(b []byte, off int, v uint16) { order.PutUint16(b[off:], v) }
	put32 := func(b []byte, off int, v uint32) { order.PutUint32(b[off:], v) }
	put64 := func(b []byte, off int, v uint64) {
		if is64 {
			order.PutUint64(b[off:], v)
		}
	}

	ehsize := 52 + 12*bits
	phentsize := 32 + 24*bits
	shentsize := 40 + 24*bits

	var interpData, symData []byte
	if spec.interp != "" {
		interpData = append([]byte(spec.interp), 0)
	}
	if spec.symtab {
		symData = make([]byte, 24)
	}

	phnum := 0
	if spec.interp != "" {
		phnum++
	}
	if spec.dynamic {
		phnum++
	}
	if len(spec.notes) > 0 {
		phnum++
	}

	off := ehsize
	phoff := 0
	if phnum > 0 {
		phoff, off = off, off+phnum*phentsize
	}
	interpOff := 0
	if interpData != nil {
		interpOff, off = off, off+len(interpData)
	}
	noteOff := 0
	if len(spec.notes) > 0 {
		noteOff, off = off, off+len(spec.notes)
	}
	symOff := 0
	if symData != nil {
		symOff, off = off, off+len(symData)
	}
	shoff := (off + 7) &^ 7
	shnum := 1
	if interpData != nil {
		shnum++
	}
	if len(spec.notes) > 0 {
		shnum++
	}
	if symData != nil {
		shnum++
	}

	buf := make([]byte, shoff+shnum*shentsize)

	// ELF header
	copy(buf, "\x7fELF")
	buf[4], buf[5], buf[6] = spec.class, spec.data, 1
	put16(buf, 16, spec.etype)
	put16(buf, 18, spec.machine)
	put32(buf, 20, 1)
	if is64 {
		put64(buf, 32, uint64(phoff))
		put64(buf, 40, uint64(shoff))
		put32(buf, 48, spec.flags)
		put16(buf, 52, uint16(ehsize))
		put16(buf, 54, uint16(phentsize))
		put16(buf, 56, uint16(phnum))
		put16(buf, 58, uint16(shentsize))
		put16(buf, 60, uint16(shnum))
	} else {
		put32(buf, 28, uint32(phoff))
		put32(buf, 32, uint32(shoff))
		put32(buf, 36, spec.flags)
		put16(buf, 40, uint16(ehsize))
		put16(buf, 42, uint16(phentsize))
		put16(buf, 44, uint16(phnum))
		put16(buf, 46, uint16(shentsize))
		put16(buf, 48, uint16(shnum))
	}

	// Program headers
	ph := phoff
	writePhdr := func(pType uint32, offset, filesz uint64) {
		put32(buf, ph, pType)
		if is64 {
			put64(buf, ph+8, offset)
			put64(buf, ph+32, filesz)
			put64(buf, ph+40, filesz)
		} else {
			put32(buf, ph+4, uint32(offset))
			put32(buf, ph+16, uint32(filesz))
			put32(buf, ph+20, uint32(filesz))
			put32(buf, ph+24, 4)
		}
		ph += phentsize
	}
	if spec.interp != "" {
		writePhdr(3, uint64(interpOff), uint64(len(interpData)))
	}
	if spec.dynamic {
		writePhdr(2, 0, 0)
	}
	if len(spec.notes) > 0 {
		writePhdr(4, uint64(noteOff), uint64(len(spec.notes)))
	}

	// Section contents
	copy(buf[interpOff:], interpData)
	copy(buf[noteOff:], spec.notes)
	copy(buf[symOff:], symData)

	// Section headers, starting after the null section
	sh := shoff + shentsize
	writeShdr := func(shType uint32, offset, size uint64) {
		put32(buf, sh+4, shType)
		if is64 {
			put64(buf, sh+24, offset)
			put64(buf, sh+32, size)
		} else {
			put32(buf, sh+16, uint32(offset))
			put32(buf, sh+20, uint32(size))
		}
		sh += shentsize
	}
	if interpData != nil {
		writeShdr(1, uint64(interpOff), uint64(len(interpData))) // SHT_PROGBITS
	}
	if len(spec.notes) > 0 {
		writeShdr(7, uint64(noteOff), uint64(len(spec.notes))) // SHT_NOTE
	}
	if symData != nil {
		writeShdr(2, uint64(symOff), uint64(len(symData))) // SHT_SYMTAB
	}
	return buf
}

// fileAndroidNote is the .android.note.ident payload: the API level and, for NDK
// r14 and later, the NDK version and build number, each in a 64-byte field.
func fileAndroidNote(api uint32, ndk, build string) []byte {
	desc := binary.LittleEndian.AppendUint32(nil, api)
	if ndk != "" || build != "" {
		version := make([]byte, 128)
		copy(version[0:], ndk)
		copy(version[64:], build)
		desc = append(desc, version...)
	}
	return desc
}

func TestFileELFDescriptions(t *testing.T) {
	buildID := []byte{0x0c, 0x71, 0x2b, 0x8a, 0xf4, 0x24, 0xd5, 0x70}
	for _, tc := range []struct {
		name string
		spec fileELFSpec
		want string
	}{
		{
			"64-bit-le-dynamic-buildid",
			fileELFSpec{
				class: 2, data: 1, etype: 3, machine: 183, // arm64
				interp: "/system/bin/linker64", dynamic: true, symtab: true,
				notes: fileELFNote("GNU\x00", 3, buildID, false),
			},
			"ELF shared object, 64-bit LSB arm64, dynamic (/system/bin/linker64), BuildID=0c712b8af424d570, not stripped",
		},
		{
			"32-bit-le-android-note",
			fileELFSpec{
				class: 1, data: 1, etype: 3, machine: 40, flags: 0x05000200, // arm, EABI5, soft float
				interp: "/system/bin/linker", dynamic: true,
				notes: fileELFNote("Android\x00", 1, fileAndroidNote(28, "", ""), false),
			},
			"ELF shared object, 32-bit LSB arm, EABI5, soft float, dynamic (/system/bin/linker), for Android 28, stripped",
		},
		{
			"32-bit-le-android-ndk-note",
			fileELFSpec{
				class: 1, data: 1, etype: 3, machine: 40, flags: 0x05000400, // hard float
				interp: "/system/bin/linker", dynamic: true,
				notes: fileELFNote("Android\x00", 1, fileAndroidNote(24, "r19b", "5304403"), false),
			},
			"ELF shared object, 32-bit LSB arm, EABI5, hard float, dynamic (/system/bin/linker), for Android 24, built by NDK r19b (5304403), stripped",
		},
		{
			"32-bit-be-fdpic-static",
			fileELFSpec{class: 1, data: 2, etype: 2, machine: 42, flags: 0x8000}, // sh
			"ELF executable (fdpic), 32-bit MSB sh, static, stripped",
		},
		{
			"64-bit-be-relocatable",
			fileELFSpec{class: 2, data: 2, etype: 1, machine: 21, symtab: true}, // ppc64
			"ELF relocatable, 64-bit MSB ppc64, static, not stripped",
		},
		{
			"64-bit-le-core-dump",
			fileELFSpec{class: 2, data: 1, etype: 4, machine: 62}, // x86-64
			"ELF core dump, 64-bit LSB x86-64, static, stripped",
		},
		{
			"riscv-flags",
			fileELFSpec{class: 2, data: 1, etype: 2, machine: 243, flags: 0x1 | 0x8 | 0x10 | 0x4},
			"ELF executable, 64-bit LSB riscv, C, E, TSO, double float, static, stripped",
		},
		{
			"32-bit-le-mips",
			fileELFSpec{class: 1, data: 1, etype: 2, machine: 8},
			"ELF executable, 32-bit LSB mips, static, stripped",
		},
		{
			"mips-old",
			fileELFSpec{class: 1, data: 2, etype: 2, machine: 10},
			"ELF executable, 32-bit MSB mips-old, static, stripped",
		},
		{
			"32-bit-le-arm-be8",
			fileELFSpec{
				class: 1, data: 1, etype: 2, machine: 40, flags: 0x08000400,
			},
			"ELF executable, 32-bit LSB arm, EABI8, hard float, static, stripped",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			fileWriteBytes(t, dir, "elf", fileELFBytes(tc.spec))
			r := fileRun(t, dir, "", "-b", "elf")
			assertExitCode(t, r, 0)
			assertStdout(t, r, tc.want+"\n")
			if r.stderr != "" {
				t.Errorf("stderr = %q, want empty", r.stderr)
			}
		})
	}
}

func TestFileELFBadHeaders(t *testing.T) {
	good := fileELFBytes(fileELFSpec{
		class: 2, data: 1, etype: 2, machine: 183,
		interp: "/lib/ld.so", dynamic: true, symtab: true,
	})
	for _, tc := range []struct {
		name string
		edit func([]byte) []byte
		want string
	}{
		{
			"bad-type",
			func(b []byte) []byte { binary.LittleEndian.PutUint16(b[16:], 99); return b },
			"ELF (bad type 99), 64-bit LSB arm64",
		},
		{
			"bad-class",
			func(b []byte) []byte { b[4] = 0; return b },
			"ELF executable, (bad class -1) LSB arm64",
		},
		{
			"bad-endian",
			func(b []byte) []byte { b[5] = 0; return b },
			"ELF executable, 64-bit (bad endian 0) arm64",
		},
		{
			"unknown-arch",
			func(b []byte) []byte { binary.LittleEndian.PutUint16(b[18:], 0x1234); return b },
			"ELF executable, 64-bit LSB unknown arch 4660, dynamic (/lib/ld.so), not stripped",
		},
		{
			"bad-phentsize",
			func(b []byte) []byte { binary.LittleEndian.PutUint16(b[54:], 33); return b },
			"ELF executable, 64-bit LSB arm64, bad phentsize 33?",
		},
		{
			"bad-phoff",
			func(b []byte) []byte {
				binary.LittleEndian.PutUint64(b[32:], uint64(len(b))+1)
				return b
			},
			"ELF executable, 64-bit LSB arm64, bad phoff %d?",
		},
		{
			"bad-shoff",
			func(b []byte) []byte {
				binary.LittleEndian.PutUint64(b[40:], uint64(len(b))+1)
				return b
			},
			// The C prints phoff in the shoff message; phoff is 64 here.
			"ELF executable, 64-bit LSB arm64, bad shoff 64?",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			data := tc.edit(append([]byte(nil), good...))
			fileWriteBytes(t, dir, "elf", data)
			want := tc.want
			if strings.Contains(want, "%d") {
				want = fmt.Sprintf(want, len(data)+1)
			}
			r := fileRun(t, dir, "", "-b", "elf")
			assertExitCode(t, r, 0)
			assertStdout(t, r, want+"\n")
		})
	}
}

// TestFileELFBadBits is the toybox bad-bits case: 45 bytes of ELF-ish junk.
func TestFileELFBadBits(t *testing.T) {
	dir := t.TempDir()
	fileWriteBytes(t, dir, "bad-bits", append([]byte("\x7fELF"), make([]byte, 41)...))
	r := fileRun(t, dir, "", "bad-bits")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "bad-bits: ELF (bad type 0), (bad class -1) (bad endian 0) unknown arch 0\n")

	// tests/file.test's spelling: a NUL followed by '0' digits.
	junk := append([]byte("\x7fELF\x00"), bytes.Repeat([]byte("0"), 42)...)
	fileWriteBytes(t, dir, "bad-bits2", junk)
	r = fileRun(t, dir, "", "-b", "bad-bits2")
	assertExitCode(t, r, 0)
	for _, want := range []string{"ELF", "unknown"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("stdout %q does not contain %q", r.stdout, want)
		}
	}
}

// TestFileELFTruncation runs every prefix of a complete ELF file, and every
// prefix with hostile header fields, through the builtin: it must never panic,
// must exit 0 or 1, and must always finish its line.
func TestFileELFTruncation(t *testing.T) {
	good := fileELFBytes(fileELFSpec{
		class: 2, data: 1, etype: 3, machine: 183,
		interp: "/system/bin/linker64", dynamic: true, symtab: true,
		notes: append(
			fileELFNote("GNU\x00", 3, []byte{1, 2, 3, 4, 5, 6, 7, 8}, false),
			fileELFNote("Android\x00", 1, fileAndroidNote(28, "r19b", "5304403"), false)...),
	})

	variants := []struct {
		name string
		edit func([]byte)
	}{
		{"plain", func([]byte) {}},
		{"shoff-huge", func(b []byte) { binary.LittleEndian.PutUint64(b[40:], 0xffffffff) }},
		{"phoff-huge", func(b []byte) { binary.LittleEndian.PutUint64(b[32:], 0xffffffff) }},
		{"shnum-huge", func(b []byte) { binary.LittleEndian.PutUint16(b[60:], 65535) }},
		{"phnum-huge", func(b []byte) { binary.LittleEndian.PutUint16(b[56:], 65535) }},
		{"note-descsz-huge", func(b []byte) {
			// The second note is the Android one; its n_descsz sits after the
			// first note, but patching any note header works: walk them.
			off := bytes.Index(b, []byte("Android\x00"))
			if off >= 4 {
				binary.LittleEndian.PutUint32(b[off-8:], 0xffffffff)
			}
		}},
	}

	for _, v := range variants {
		data := append([]byte(nil), good...)
		v.edit(data)
		for n := 0; n <= len(data); n++ {
			dir := t.TempDir()
			fileWriteBytes(t, dir, "elf", data[:n])
			r := fileRun(t, dir, "", "elf")
			if r.code != 0 && r.code != 1 {
				t.Fatalf("%s prefix %d: exit code %d", v.name, n, r.code)
			}
			if !strings.HasSuffix(r.stdout, "\n") {
				t.Fatalf("%s prefix %d: stdout %q does not end in a newline", v.name, n, r.stdout)
			}
			if r.stderr != "" {
				t.Fatalf("%s prefix %d: stderr = %q, want empty", v.name, n, r.stderr)
			}
			if strings.Contains(r.stdout, "panic") {
				t.Fatalf("%s prefix %d: stdout = %q", v.name, n, r.stdout)
			}
		}
	}
}

func TestFileSymlinks(t *testing.T) {
	dir := t.TempDir()
	target := fileWriteBytes2(t, dir, "java.class", fileJavaClass(53, 0))
	if err := os.Symlink(target, filepath.Join(dir, "symlink")); err != nil {
		t.Skipf("symlinks are not permitted: %v", err)
	}
	if err := os.Symlink(filepath.Join(dir, "missing"), filepath.Join(dir, "dangler")); err != nil {
		t.Skipf("symlinks are not permitted: %v", err)
	}

	// -h is the default: the link itself is described.
	for _, args := range [][]string{{"-b", "symlink"}, {"-b", "-h", "symlink"}, {"--brief", "-h", "symlink"}} {
		r := fileRun(t, dir, "", args...)
		assertExitCode(t, r, 0)
		assertStdout(t, r, "symbolic link to "+target+"\n")
	}

	r := fileRun(t, dir, "", "dangler")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "dangler: broken symbolic link to "+filepath.Join(dir, "missing")+"\n")

	r = fileRun(t, dir, "", "-L", "symlink")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "symlink: Java class file, version 53.0 (Java 1.9)\n")

	r = fileRun(t, dir, "", "-L", "dangler")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "dangler: cannot open: No such file or directory\n")
}

// fileWriteBytes2 is fileWriteBytes that returns the full path.
func fileWriteBytes2(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	fileWriteBytes(t, dir, name, data)
	return filepath.Join(dir, name)
}

func TestFileSpecialFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "directory"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := fileRun(t, dir, "", "directory")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "directory: directory\n")

	skipWithoutFifos(t)
	if err := syscall.Mkfifo(filepath.Join(dir, "pipe"), 0o600); err != nil {
		t.Skipf("fifos are not permitted: %v", err)
	}
	r = fileRun(t, dir, "", "pipe")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "pipe: fifo\n")

	if runtime.GOOS != "linux" {
		t.Skip("device numbers are only checked on Linux")
	}
	for _, tc := range []struct{ name, want string }{
		{"/dev/zero", "/dev/zero: character special (1/5)\n"},
		{"/dev/null", "/dev/null: character special (1/3)\n"},
	} {
		if _, err := os.Stat(tc.name); err != nil {
			t.Skipf("%s: %v", tc.name, err)
		}
		r := fileRun(t, dir, "", tc.name)
		assertExitCode(t, r, 0)
		assertStdout(t, r, tc.want)
	}

	// -s reads the device instead of printing its numbers.
	if f, err := os.Open("/dev/zero"); err == nil {
		f.Close()
		r := fileRun(t, dir, "", "-s", "/dev/zero")
		assertExitCode(t, r, 0)
		assertStdout(t, r, "/dev/zero: data\n")
	}
}

func TestFileStdin(t *testing.T) {
	class := fileJavaClass(53, 0)
	r := fileRunBytes(t, t.TempDir(), class, "-")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "-: Java class file, version 53.0 (Java 1.9)\n")

	dir := t.TempDir()
	fileWriteBytes(t, dir, "ascii.txt", []byte("hello\n"))
	r = fileRunBytes(t, dir, class, "-", "ascii.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "-:         Java class file, version 53.0 (Java 1.9)\nascii.txt: ASCII text\n")

	// An ELF file on stdin is described from the bytes that are there.
	elf := fileELFBytes(fileELFSpec{class: 2, data: 1, etype: 2, machine: 62, symtab: true})
	r = fileRunBytes(t, t.TempDir(), elf, "-")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "-: ELF executable, 64-bit LSB x86-64, static, not stripped\n")

	r = fileRunBytes(t, t.TempDir(), nil, "-")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "-: empty\n")
}

func TestFileMultipleOperands(t *testing.T) {
	dir := t.TempDir()
	fileWriteBytes(t, dir, "a", []byte("hello\n"))
	fileWriteBytes(t, dir, "longer-name.bin", []byte{0xff, 0x00})
	r := fileRun(t, dir, "", "a", "longer-name.bin")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a:"+strings.Repeat(" ", 15)+"ASCII text\nlonger-name.bin: data\n")

	// -b drops the name column entirely.
	r = fileRun(t, dir, "", "-b", "a", "longer-name.bin")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "ASCII text\ndata\n")

	// One missing operand still prints the other, but the exit status is 1.
	r = fileRun(t, dir, "", "a", "missing")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "a:"+strings.Repeat(" ", 7)+"ASCII text\nmissing: cannot open: No such file or directory\n")
}

func TestFileOptionErrors(t *testing.T) {
	dir := t.TempDir()
	fileWriteBytes(t, dir, "a", []byte("hello\n"))

	for _, args := range [][]string{{"-i", "a"}, {"--mime", "a"}, {"-z", "a"}, {"-bq", "a"}} {
		r := fileRun(t, dir, "", args...)
		assertExitCode(t, r, 1)
		if r.stdout != "" {
			t.Errorf("file %v: stdout = %q, want empty", args, r.stdout)
		}
		if !strings.Contains(r.stderr, "file:") {
			t.Errorf("file %v: stderr = %q, want a file: error", args, r.stderr)
		}
	}
	if r := fileRun(t, dir, "", "-i", "a"); !strings.Contains(r.stderr, "i") {
		t.Errorf("stderr = %q, want it to name the option", r.stderr)
	}

	r := fileRun(t, dir, "", "-h", "-L", "a")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "-h and -L cannot be used together")

	r = fileRun(t, dir, "", "-b")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "missing operand")

	r = fileRun(t, dir, "")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "missing operand")
}

func TestFileHelp(t *testing.T) {
	r := fileRun(t, t.TempDir(), "", "--help")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "Usage: file")
	for _, want := range []string{"-b", "--brief", "-h", "-L", "-s"} {
		assertStdoutContains(t, r, want)
	}

	r = fileRun(t, t.TempDir(), "", "--version")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "file")

	// The standalone binary can dispatch without a CmdName; the help text must
	// still name the command.
	r = runBuiltin(t, "file", "--help")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "Usage: file")

	r = runBuiltin(t, "file", "--version")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "file (Misa)")
}

func TestFileEmptyAndUnreadable(t *testing.T) {
	dir := t.TempDir()
	fileWriteBytes(t, dir, "empty", nil)
	r := fileRun(t, dir, "", "empty")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "empty: empty\n")

	path := fileWriteBytes2(t, dir, "unreadable", []byte("secret\n"))
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o644) })
	if f, err := os.Open(path); err == nil {
		f.Close()
		t.Skip("running with permission to read a mode 000 file")
	}
	// The stat works, so the C warns on stderr from openro() and then falls
	// through to its "unknown" default.
	r = fileRun(t, dir, "", "unreadable")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "unreadable: unknown\n")
	assertStderrContains(t, r, "file: unreadable: Permission denied")

	// A missing operand is a stat error, and later operands still run.
	r = fileRun(t, dir, "", "nope", "empty")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "nope:  cannot open: No such file or directory\nempty: empty\n")
}

func TestFileTarChecksum(t *testing.T) {
	h := fileTarHeader("hello.txt", "ustar  \x00")
	if !fileIsTarHeader(h) {
		t.Fatal("synthetic tar header was not recognised")
	}
	// The checksum field itself is left out of the sum, so rewriting it in
	// another octal spelling of the same value still matches.
	sum := fileTarCksum(h)
	copy(h[148:], fmt.Sprintf("%07o ", sum))
	if !fileIsTarHeader(h) {
		t.Error("tar header with a rewritten checksum field was not recognised")
	}
	// Any other byte changing is a mismatch.
	h = fileTarHeader("hello.txt", "ustar  \x00")
	h[0] = 'j'
	if fileIsTarHeader(h) {
		t.Error("tar header with a wrong checksum was recognised")
	}
	if fileIsTarHeader(make([]byte, 100)) {
		t.Error("short buffer was recognised as a tar header")
	}
	// The ustar magic is optional, but a non-ustar magic is not accepted.
	h = fileTarHeader("hello.txt", "XXXXX  \x00")
	if fileIsTarHeader(h) {
		t.Error("tar header with a bogus magic was recognised")
	}
}
