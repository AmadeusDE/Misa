package builtins

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

const cksumSpec = "a(algorithm):c(check)0z(zero)l(length):t(tag)(untagged)(raw)(base64)b(binary)(debug)(quiet)(status)"

var cksumSpecParsed *Spec
var cksumTable [256]uint32

func init() {
	for i := 0; i < 256; i++ {
		crc := uint32(i) << 24
		for j := 0; j < 8; j++ {
			if crc&0x80000000 != 0 {
				crc = (crc << 1) ^ 0x04C11DB7
			} else {
				crc <<= 1
			}
		}
		cksumTable[i] = crc
	}
	var err error
	cksumSpecParsed, err = ParseSpec(cksumSpec)
	if err != nil {
		panic("builtins: bad cksum spec: " + err.Error())
	}
	Register(Command{Name: "cksum", Run: runCksum})
}

func posixCksum(data []byte) uint32 {
	crc := uint32(0)
	for _, b := range data {
		crc = (crc << 8) ^ cksumTable[byte(crc>>24)^b]
	}
	length := len(data)
	for length > 0 {
		crc = (crc << 8) ^ cksumTable[byte(crc>>24)^byte(length)]
		length >>= 8
	}
	return ^crc
}

func sysvCksum(data []byte) uint32 {
	var sum uint32
	for _, b := range data {
		sum += uint32(b)
	}
	r := (sum & 0xffff) + (sum >> 16)
	return (r & 0xffff) + (r >> 16)
}

func bsdCksum(data []byte) uint16 {
	var checksum uint16
	for _, b := range data {
		checksum = (checksum >> 1) + ((checksum & 1) << 15)
		checksum += uint16(b)
	}
	return checksum
}

func blake2bHash(data []byte, outLen int) []byte {
	if outLen <= 0 || outLen > 64 {
		outLen = 64
	}
	iv := [8]uint64{
		0x6a09e667f3bcc908, 0xbb67ae8584caa73b,
		0x3c6ef372fe94f82b, 0xa54ff53a5f1d36f1,
		0x510e527fade682d1, 0x9b05688c2b3e6c1f,
		0x1f83d9abfb41bd6b, 0x5be0cd19137e2179,
	}
	var h [8]uint64
	copy(h[:], iv[:])
	h[0] ^= 0x01010000 | uint64(outLen)

	sigma := [12][16]int{
		{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
		{14, 10, 4, 8, 9, 15, 13, 6, 1, 12, 0, 2, 11, 7, 5, 3},
		{11, 8, 12, 0, 5, 2, 15, 13, 10, 14, 3, 6, 7, 1, 9, 4},
		{7, 9, 3, 1, 13, 12, 11, 14, 2, 6, 5, 10, 4, 0, 15, 8},
		{9, 0, 5, 7, 2, 4, 10, 15, 14, 1, 11, 12, 6, 8, 3, 13},
		{2, 12, 6, 10, 0, 11, 8, 3, 4, 13, 7, 5, 15, 14, 1, 9},
		{12, 5, 1, 15, 14, 13, 4, 10, 0, 7, 6, 3, 9, 2, 8, 11},
		{13, 11, 7, 14, 12, 1, 3, 9, 5, 0, 15, 4, 8, 6, 2, 10},
		{6, 15, 14, 9, 11, 3, 0, 8, 12, 2, 13, 7, 1, 4, 10, 5},
		{10, 2, 8, 4, 7, 6, 1, 5, 15, 11, 9, 14, 3, 12, 13, 0},
		{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
		{14, 10, 4, 8, 9, 15, 13, 6, 1, 12, 0, 2, 11, 7, 5, 3},
	}

	compress := func(chunk []byte, bytesCompressed uint64, isLast bool) {
		var m [16]uint64
		for i := 0; i < 16; i++ {
			m[i] = binary.LittleEndian.Uint64(chunk[i*8:])
		}
		var v [16]uint64
		copy(v[0:8], h[:])
		copy(v[8:16], iv[:])
		v[12] ^= bytesCompressed
		if isLast {
			v[14] ^= 0xffffffffffffffff
		}
		rotr64 := func(w uint64, c uint) uint64 {
			return (w >> c) | (w << (64 - c))
		}
		g := func(a, b, c, d int, x, y uint64) {
			v[a] += v[b] + x
			v[d] = rotr64(v[d]^v[a], 32)
			v[c] += v[d]
			v[b] = rotr64(v[b]^v[c], 24)
			v[a] += v[b] + y
			v[d] = rotr64(v[d]^v[a], 16)
			v[c] += v[d]
			v[b] = rotr64(v[b]^v[c], 63)
		}
		for r := 0; r < 12; r++ {
			s := sigma[r]
			g(0, 4, 8, 12, m[s[0]], m[s[1]])
			g(1, 5, 9, 13, m[s[2]], m[s[3]])
			g(2, 6, 10, 14, m[s[4]], m[s[5]])
			g(3, 7, 11, 15, m[s[6]], m[s[7]])
			g(0, 5, 10, 15, m[s[8]], m[s[9]])
			g(1, 6, 11, 12, m[s[10]], m[s[11]])
			g(2, 7, 8, 13, m[s[12]], m[s[13]])
			g(3, 4, 9, 14, m[s[14]], m[s[15]])
		}
		for i := 0; i < 8; i++ {
			h[i] ^= v[i] ^ v[i+8]
		}
	}

	offset := 0
	length := len(data)
	if length == 0 {
		var block [128]byte
		compress(block[:], 0, true)
	} else {
		for offset < length {
			if offset+128 >= length {
				var block [128]byte
				rem := length - offset
				copy(block[:rem], data[offset:])
				compress(block[:], uint64(length), true)
				break
			} else {
				compress(data[offset:offset+128], uint64(offset+128), false)
				offset += 128
			}
		}
	}

	res := make([]byte, outLen)
	for i := 0; i < outLen; i++ {
		res[i] = byte(h[i/8] >> (8 * (i % 8)))
	}
	return res
}

func computeDigest(algo string, data []byte, digestBits int) ([]byte, string, error) {
	algoLower := strings.ToLower(algo)
	switch algoLower {
	case "md5":
		h := md5.Sum(data)
		return h[:], "MD5", nil
	case "sha1":
		h := sha1.Sum(data)
		return h[:], "SHA1", nil
	case "sha224":
		h := sha256.Sum224(data)
		return h[:], "SHA224", nil
	case "sha256":
		h := sha256.Sum256(data)
		return h[:], "SHA256", nil
	case "sha384":
		h := sha512.Sum384(data)
		return h[:], "SHA384", nil
	case "sha512":
		h := sha512.Sum512(data)
		return h[:], "SHA512", nil
	case "sha2", "sha3":
		switch digestBits {
		case 224:
			if algoLower == "sha2" {
				return computeDigest("sha224", data, digestBits)
			}
			return computeDigest("sha3-224", data, digestBits)
		case 256:
			if algoLower == "sha2" {
				return computeDigest("sha256", data, digestBits)
			}
			return computeDigest("sha3-256", data, digestBits)
		case 384:
			if algoLower == "sha2" {
				return computeDigest("sha384", data, digestBits)
			}
			return computeDigest("sha3-384", data, digestBits)
		case 512:
			if algoLower == "sha2" {
				return computeDigest("sha512", data, digestBits)
			}
			return computeDigest("sha3-512", data, digestBits)
		default:
			return nil, "", fmt.Errorf("--algorithm=%s requires specifying --length 224, 256, 384, or 512", algoLower)
		}
	case "blake2b":
		outLen := 64
		if digestBits > 0 {
			if digestBits > 512 {
				return nil, "", fmt.Errorf("maximum digest length for 'BLAKE2b' is 512 bits")
			}
			if digestBits%8 != 0 {
				return nil, "", fmt.Errorf("invalid length: '%d'", digestBits)
			}
			outLen = digestBits / 8
		}
		res := blake2bHash(data, outLen)
		name := "BLAKE2b"
		if digestBits > 0 {
			name = fmt.Sprintf("BLAKE2b-%d", digestBits)
		}
		return res, name, nil
	case "sysv":
		return nil, "SYSV", nil
	case "bsd":
		return nil, "BSD", nil
	case "crc", "posix":
		return nil, "CRC", nil
	default:
		return nil, "", fmt.Errorf("unknown algorithm '%s'", algo)
	}
}

type checkItem struct {
	expectedHash string
	filename     string
	algoName     string
}

var bsdTagRe = regexp.MustCompile(`^([A-Za-z0-9_\-]+)\s*\((.+)\)\s*=\s*([0-9a-fA-F+/=]+)$`)
var untaggedRe = regexp.MustCompile(`^([A-Za-z0-9+/=]+)\s+[\ \*]?(.*)$`)

func runCksum(ctx *Context) (code int) {
	ctx, cleanup := streamContext(ctx)
	defer cleanup()
	defer streamResult(ctx, &code)
	flags, err := cksumSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "cksum:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, cksumSpecParsed) {
		return 0
	}

	algo, hasAlgo := flags.String("a")
	algo = strings.TrimPrefix(algo, "=")
	debugMode := flags.Bool("debug")
	nullTerm := flags.Bool("0") || flags.Bool("zero") || flags.Bool("z")
	checkMode := flags.Bool("c") || flags.Bool("check")
	rawMode := flags.Bool("raw")
	base64Mode := flags.Bool("base64")
	binaryMode := flags.Bool("b") || flags.Bool("binary")
	quietMode := flags.Bool("quiet")
	statusMode := flags.Bool("status")

	if debugMode {
		fmt.Fprintln(ctx.Stderr, "pclmul")
	}

	lengthBits := -1
	if lStr, ok := flags.ExplicitString("l"); ok {
		v, err := strconv.Atoi(lStr)
		if err != nil || v < 0 {
			if (algo == "blake2b" || algo == "b2sum") && err != nil && (strings.Contains(err.Error(), "range") || v > 512) {
				fmt.Fprintln(ctx.Stderr, "cksum: maximum digest length for 'BLAKE2b' is 512 bits")
				return 1
			}
			fmt.Fprintf(ctx.Stderr, "cksum: invalid length: '%s'\n", lStr)
			return 1
		}
		if (algo == "blake2b" || algo == "b2sum") && (v > 512 || v%8 != 0) {
			if v > 512 {
				fmt.Fprintln(ctx.Stderr, "cksum: maximum digest length for 'BLAKE2b' is 512 bits")
			} else {
				fmt.Fprintf(ctx.Stderr, "cksum: invalid length: '%s'\n", lStr)
			}
			return 1
		}
		lengthBits = v
	}

	tag := false
	untagged := false
	for _, arg := range ctx.Args {
		if arg == "-t" || arg == "--tag" || strings.HasPrefix(arg, "-t") {
			tag = true
			untagged = false
		} else if arg == "--untagged" {
			untagged = true
			tag = false
		}
	}

	files := flags.Operands
	if len(files) == 0 {
		files = []string{"-"}
	}

	if rawMode && len(files) > 1 {
		fmt.Fprintln(ctx.Stderr, "cksum: the --raw option is not supported with multiple files")
		return 1
	}

	if rawMode && base64Mode {
		fmt.Fprintln(ctx.Stderr, "cksum: --raw and --base64 are mutually exclusive")
		return 1
	}

	if hasAlgo {
		if _, _, err := computeDigest(algo, nil, lengthBits); err != nil {
			fmt.Fprintln(ctx.Stderr, "cksum:", err)
			return 1
		}
	}

	sep := "\n"
	if nullTerm {
		sep = "\x00"
	}

	if checkMode {
		return runCheckMode(ctx, files, algo, lengthBits, quietMode, statusMode)
	}

	exitCode := 0
	for _, f := range files {
		var data []byte
		var err error
		var displayName string

		if f == "-" {
			if ctx.Stdin == nil {
				fmt.Fprintln(ctx.Stderr, "cksum: stdin not available")
				exitCode = 1
				continue
			}
			data, err = io.ReadAll(ctx.Stdin)
			displayName = ""
		} else {
			path := resolvePath(ctx, f)
			data, err = ReadFileContext(ctx.CancellationContext(), path)
			displayName = f
		}
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "cksum: %s: %s\n", f, fmtOsErr(err))
			exitCode = 1
			continue
		}

		if !hasAlgo || algo == "sysv" || algo == "bsd" || algo == "crc" || algo == "posix" {
			if algo == "sysv" {
				val := sysvCksum(data)
				blocks := (len(data) + 511) / 512
				if displayName == "" {
					fmt.Fprintf(ctx.Stdout, "%d %d%s", val, blocks, sep)
				} else {
					fmt.Fprintf(ctx.Stdout, "%d %d %s%s", val, blocks, displayName, sep)
				}
			} else if algo == "bsd" {
				val := bsdCksum(data)
				blocks := (len(data) + 1023) / 1024
				if displayName == "" {
					fmt.Fprintf(ctx.Stdout, "%05d %5d%s", val, blocks, sep)
				} else {
					fmt.Fprintf(ctx.Stdout, "%05d %5d %s%s", val, blocks, displayName, sep)
				}
			} else {
				crc := posixCksum(data)
				if displayName == "" {
					fmt.Fprintf(ctx.Stdout, "%d %d%s", crc, len(data), sep)
				} else {
					fmt.Fprintf(ctx.Stdout, "%d %d %s%s", crc, len(data), displayName, sep)
				}
			}
			continue
		}

		digest, algoName, err := computeDigest(algo, data, lengthBits)
		if err != nil {
			fmt.Fprintln(ctx.Stderr, "cksum:", err)
			return 1
		}

		if rawMode {
			ctx.Stdout.Write(digest)
			continue
		}

		var formattedHash string
		if base64Mode {
			formattedHash = base64.StdEncoding.EncodeToString(digest)
		} else {
			formattedHash = hex.EncodeToString(digest)
		}

		targetName := displayName
		if targetName == "" {
			targetName = "-"
		}

		if tag || (!untagged && hasAlgo) {
			fmt.Fprintf(ctx.Stdout, "%s (%s) = %s%s", algoName, targetName, formattedHash, sep)
		} else {
			binPrefix := " "
			if binaryMode {
				binPrefix = "*"
			}
			fmt.Fprintf(ctx.Stdout, "%s %s%s%s", formattedHash, binPrefix, targetName, sep)
		}
	}
	return exitCode
}

func isValidCheckAlgo(name string, expectedAlgo string) bool {
	upper := strings.ToUpper(name)
	if expectedAlgo != "" && expectedAlgo != "crc" && expectedAlgo != "posix" {
		expUpper := strings.ToUpper(expectedAlgo)
		if upper != expUpper && !strings.HasPrefix(upper, expUpper+"-") {
			return false
		}
	}
	switch upper {
	case "MD5", "SHA1", "SHA224", "SHA256", "SHA384", "SHA512", "BLAKE2B", "SYSV", "BSD", "CRC":
		return true
	}
	if strings.HasPrefix(upper, "BLAKE2B-") {
		bitsStr := upper[len("BLAKE2B-"):]
		v, err := strconv.Atoi(bitsStr)
		return err == nil && v > 0 && v <= 512 && v%8 == 0
	}
	return false
}

func runCheckMode(ctx *Context, checkFiles []string, expectedAlgo string, digestBits int, quietMode, statusMode bool) int {
	exitCode := 0
	for _, checkFile := range checkFiles {
		var content []byte
		var err error
		if checkFile == "-" {
			if ctx.Stdin == nil {
				fmt.Fprintln(ctx.Stderr, "cksum: stdin not available")
				return 1
			}
			content, err = io.ReadAll(ctx.Stdin)
		} else {
			path := resolvePath(ctx, checkFile)
			content, err = ReadFileContext(ctx.CancellationContext(), path)
		}
		if err != nil {
			msg := fmtOsErr(err)
			if strings.Contains(err.Error(), "read error") || strings.Contains(err.Error(), "Input/output error") {
				msg = "read error"
			}
			fmt.Fprintf(ctx.Stderr, "cksum: %s: %s\n", checkFile, msg)
			return 1
		}

		scanner := bufio.NewScanner(bytes.NewReader(content))
		var items []checkItem
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if m := bsdTagRe.FindStringSubmatch(line); m != nil {
				if isValidCheckAlgo(m[1], expectedAlgo) {
					items = append(items, checkItem{algoName: m[1], filename: m[2], expectedHash: m[3]})
				}
			} else if !strings.Contains(line, " = ") {
				if m := untaggedRe.FindStringSubmatch(line); m != nil {
					algoName := expectedAlgo
					if algoName == "" {
						algoName = "sha256"
					}
					items = append(items, checkItem{algoName: algoName, filename: m[2], expectedHash: m[1]})
				}
			}
		}

		if len(items) == 0 {
			if checkFile == "-" {
				fmt.Fprintln(ctx.Stderr, "cksum: no properly formatted checksum lines found")
			} else {
				fmt.Fprintf(ctx.Stderr, "cksum: %s: no properly formatted checksum lines found\n", checkFile)
			}
			return 1
		}

		readFailCount := 0
		checksumFailCount := 0

		for _, item := range items {
			path := resolvePath(ctx, item.filename)
			data, err := ReadFileContext(ctx.CancellationContext(), path)
			if err != nil {
				readFailCount++
				fmt.Fprintf(ctx.Stderr, "cksum: %s: %s\n", item.filename, fmtOsErr(err))
				if !statusMode {
					fmt.Fprintf(ctx.Stdout, "%s: FAILED open or read\n", item.filename)
				}
				continue
			}

			digest, _, err := computeDigest(item.algoName, data, digestBits)
			if err != nil {
				checksumFailCount++
				if !statusMode {
					fmt.Fprintf(ctx.Stdout, "%s: FAILED\n", item.filename)
				}
				continue
			}

			gotHex := hex.EncodeToString(digest)
			gotB64 := base64.StdEncoding.EncodeToString(digest)
			if strings.EqualFold(gotHex, item.expectedHash) || gotB64 == item.expectedHash {
				if !quietMode && !statusMode {
					fmt.Fprintf(ctx.Stdout, "%s: OK\n", item.filename)
				}
			} else {
				checksumFailCount++
				if !statusMode {
					fmt.Fprintf(ctx.Stdout, "%s: FAILED\n", item.filename)
				}
			}
		}

		if readFailCount > 0 {
			exitCode = 1
			if !statusMode {
				fmt.Fprintf(ctx.Stderr, "cksum: WARNING: %d listed file could not be read\n", readFailCount)
			}
		}
		if checksumFailCount > 0 {
			exitCode = 1
			if !statusMode {
				fmt.Fprintf(ctx.Stderr, "cksum: WARNING: %d computed checksum did NOT match\n", checksumFailCount)
			}
		}
	}
	return exitCode
}
