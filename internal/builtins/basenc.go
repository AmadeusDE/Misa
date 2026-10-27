// Behaviour adapted from uutils coreutils basenc (MIT).
package builtins

import (
	"bufio"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"math/bits"
	"os"
	"strconv"
	"strings"
)

const basencSpec = "d(decode)Di(ignore-garbage)w(wrap):(base64)(base64url)(base32)(base32hex)(base16)(base2msbf)(base2lsbf)(z85)(base58)"
const basencZ85 = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ.-:+=^!/*?&<>()[]{}@%$#"
const basencBase58 = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

func init() { Register(Command{Name: "basenc", Run: runBasenc}) }

func runBasenc(ctx *Context) int {
	spec, err := ParseSpec(basencSpec)
	if err != nil {
		panic(err)
	}
	flags, err := spec.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "basenc: %s\n", err)
		return 1
	}
	if flags.HelpRequested {
		fmt.Fprintln(ctx.Stdout, "Usage: basenc [OPTION]... [FILE]\nEncode or decode FILE, or standard input.\n  --base64 --base64url --base32 --base32hex --base16\n  --base2msbf --base2lsbf --z85 --base58\n  -d, -D, --decode          decode data\n  -i, --ignore-garbage      ignore non-alphabet characters\n  -w, --wrap=COLS           wrap encoded lines (default 76; 0 disables)\n      --help               display this help\n      --version            display version")
		return 0
	}
	if HandleHelpVersion(ctx, flags, spec) {
		return 0
	}
	encoding := ""
	for _, flag := range flags.Order() {
		switch flag {
		case "base64", "base64url", "base32", "base32hex", "base16", "base2msbf", "base2lsbf", "z85", "base58":
			encoding = flag
		}
	}
	if encoding == "" {
		fmt.Fprintln(ctx.Stderr, "basenc: missing encoding type")
		return 1
	}
	wrap := uint64(76)
	if val, ok := flags.String("w"); ok {
		wrap, err = strconv.ParseUint(val, 10, 64)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "basenc: invalid wrap size: '%s'\n", val)
			return 1
		}
	}
	if len(flags.Operands) > 1 {
		fmt.Fprintf(ctx.Stderr, "basenc: extra operand '%s'\n", flags.Operands[1])
		return 1
	}
	input := ctx.Stdin
	if len(flags.Operands) == 1 && flags.Operands[0] != "-" {
		name := flags.Operands[0]
		file, openErr := os.Open(resolvePath(ctx, name))
		if openErr != nil {
			fmt.Fprintf(ctx.Stderr, "basenc: %s: %s\n", name, fmtOsErr(openErr))
			return 1
		}
		defer file.Close()
		input = file
	}
	if input == nil {
		input = strings.NewReader("")
	}
	output := bufio.NewWriter(ctx.Stdout)
	if flags.Bool("d") || flags.Bool("D") {
		err = basencDecode(input, output, encoding, flags.Bool("i"))
	} else {
		var data []byte
		data, err = io.ReadAll(input)
		if err != nil {
			err = fmt.Errorf("read error: %s", fmtOsErr(err))
		} else {
			var encoded string
			encoded, err = basencEncode(data, encoding)
			if err == nil {
				err = basencWriteEncoded(output, encoded, wrap)
			}
		}
	}
	if flushErr := output.Flush(); err == nil {
		err = flushErr
	}
	if err != nil {
		message := err.Error()
		if _, ok := err.(*os.PathError); ok {
			message = fmtOsErr(err)
		}
		fmt.Fprintf(ctx.Stderr, "basenc: %s\n", message)
		return 1
	}
	return 0
}

func basencWriteEncoded(output io.Writer, encoded string, wrap uint64) error {
	for len(encoded) > 0 {
		n := len(encoded)
		if wrap > 0 && uint64(n) > wrap {
			n = int(wrap)
		}
		if _, err := io.WriteString(output, encoded[:n]); err != nil {
			return err
		}
		if wrap > 0 {
			if _, err := io.WriteString(output, "\n"); err != nil {
				return err
			}
		}
		encoded = encoded[n:]
	}
	return nil
}

func basencEncode(data []byte, encoding string) (string, error) {
	switch encoding {
	case "base64":
		return base64.StdEncoding.EncodeToString(data), nil
	case "base64url":
		return base64.URLEncoding.EncodeToString(data), nil
	case "base32":
		return base32.StdEncoding.EncodeToString(data), nil
	case "base32hex":
		return base32.HexEncoding.EncodeToString(data), nil
	case "base16":
		return strings.ToUpper(hex.EncodeToString(data)), nil
	case "base2msbf", "base2lsbf":
		var result strings.Builder
		result.Grow(len(data) * 8)
		for _, b := range data {
			if encoding == "base2lsbf" {
				b = bits.Reverse8(b)
			}
			for shift := 7; shift >= 0; shift-- {
				result.WriteByte('0' + (b >> shift & 1))
			}
		}
		return result.String(), nil
	case "z85":
		if len(data)%4 != 0 {
			return "", fmt.Errorf("error: invalid input (length must be multiple of 4 characters)")
		}
		result := make([]byte, len(data)/4*5)
		for i := 0; i < len(data); i += 4 {
			v := binary.BigEndian.Uint32(data[i:])
			for j := 4; j >= 0; j-- {
				result[i/4*5+j] = basencZ85[v%85]
				v /= 85
			}
		}
		return string(result), nil
	case "base58":
		zeroes := 0
		for zeroes < len(data) && data[zeroes] == 0 {
			zeroes++
		}
		n := new(big.Int).SetBytes(data)
		radix, remainder := big.NewInt(58), new(big.Int)
		var result []byte
		for n.Sign() > 0 {
			n.QuoRem(n, radix, remainder)
			result = append(result, basencBase58[remainder.Int64()])
		}
		for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
			result[left], result[right] = result[right], result[left]
		}
		return strings.Repeat("1", zeroes) + string(result), nil
	}
	return "", basencInvalid()
}

func basencInvalid() error { return fmt.Errorf("error: invalid input") }

func basencAlphabet(encoding string) (string, int, int) {
	switch encoding {
	case "base64":
		return "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/", 6, 4
	case "base64url":
		return "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_", 6, 4
	case "base32":
		return "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567", 5, 8
	case "base32hex":
		return "0123456789ABCDEFGHIJKLMNOPQRSTUV", 5, 8
	case "base16":
		return "0123456789ABCDEF", 4, 2
	case "base2msbf", "base2lsbf":
		return "01", 1, 8
	case "z85":
		return basencZ85, 0, 5
	case "base58":
		return basencBase58, 0, 1
	}
	return "", 0, 0
}

func basencDecode(input io.Reader, output *bufio.Writer, encoding string, ignore bool) error {
	alphabet, width, quantum := basencAlphabet(encoding)
	reader := bufio.NewReader(input)
	var value uint64
	var bitCount, count, padding int
	var base58 []byte
	for {
		c, err := reader.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read error: %s", fmtOsErr(err))
		}
		if c == '\n' || c == '\r' {
			continue
		}
		if encoding == "base16" && c >= 'a' && c <= 'f' {
			c -= 'a' - 'A'
		}
		digit := strings.IndexByte(alphabet, c)
		if digit < 0 && !(c == '=' && (width == 5 || width == 6)) {
			if ignore {
				continue
			}
			return basencInvalid()
		}
		if c == '=' && (width == 5 || width == 6) {
			if padding == 0 {
				if !basencValidRemainder(count, width) || count == 0 || value != 0 {
					return basencInvalid()
				}
				padding = quantum - count
			}
			padding--
			if padding == 0 {
				count, bitCount, value = 0, 0, 0
			}
			continue
		}
		if padding > 0 {
			return basencInvalid()
		}
		if encoding == "base58" {
			base58 = append(base58, c)
			continue
		}
		if encoding == "z85" {
			value = value*85 + uint64(digit)
			count++
			if count == quantum {
				if value > 0xffffffff {
					return basencInvalid()
				}
				var decoded [4]byte
				binary.BigEndian.PutUint32(decoded[:], uint32(value))
				if _, err := output.Write(decoded[:]); err != nil {
					return err
				}
				value, count = 0, 0
			}
			continue
		}
		value = value<<width | uint64(digit)
		bitCount += width
		count = (count + 1) % quantum
		if bitCount >= 8 {
			bitCount -= 8
			b := byte(value >> bitCount)
			if encoding == "base2lsbf" {
				b = bits.Reverse8(b)
			}
			if err := output.WriteByte(b); err != nil {
				return err
			}
			value &= (1 << bitCount) - 1
		}
	}
	if encoding == "base58" {
		n := new(big.Int)
		radix, digit := big.NewInt(58), new(big.Int)
		zeroes := 0
		for zeroes < len(base58) && base58[zeroes] == '1' {
			zeroes++
		}
		for _, c := range base58 {
			n.Mul(n, radix)
			n.Add(n, digit.SetInt64(int64(strings.IndexByte(alphabet, c))))
		}
		if _, err := output.Write(make([]byte, zeroes)); err != nil {
			return err
		}
		_, err := output.Write(n.Bytes())
		return err
	}
	if padding != 0 || value != 0 || !basencValidRemainder(count, width) {
		return basencInvalid()
	}
	return nil
}

func basencValidRemainder(count, width int) bool {
	if count == 0 {
		return true
	}
	if width == 6 {
		return count == 2 || count == 3
	}
	if width == 5 {
		return count == 2 || count == 4 || count == 5 || count == 7
	}
	return false
}
