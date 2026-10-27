// Behaviour adapted from toybox shuf.c (0BSD) and uutils coreutils shuf (MIT).
package builtins

import (
	"bufio"
	cryptorand "crypto/rand"
	"crypto/sha3"
	"errors"
	"fmt"
	"io"
	"math/bits"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
)

func init() { Register(Command{Name: "shuf", Run: runShuf}) }

func runShuf(ctx *Context) int {
	if err := shufRun(ctx); err != nil {
		fmt.Fprintln(ctx.Stderr, "shuf: "+err.Error())
		return 1
	}
	return 0
}

func shufRun(ctx *Context) error {
	spec, _ := ParseSpec("e(echo)i(input-range):n(head-count):o(output):(random-source):(random-seed):r(repeat)z(zero-terminated)")
	flags, err := spec.ParseArgs(ctx.Args)
	if err != nil {
		return err
	}
	if HandleHelpVersion(ctx, flags, spec) {
		return nil
	}
	for _, flag := range []struct{ key, name string }{{"i", "--input-range <LO-HI>"}, {"o", "--output <FILE>"}, {"random-source", "--random-source <FILE>"}, {"random-seed", "--random-seed <STRING>"}} {
		occurrences := 0
		for _, key := range flags.Order() {
			if key == flag.key {
				occurrences++
			}
		}
		if occurrences > 1 {
			return fmt.Errorf("the argument '%s' cannot be used multiple times", flag.name)
		}
	}
	count := ^uint64(0)
	for _, value := range flags.AllStrings("n") {
		count, err = shufUint(value)
		if err != nil {
			return fmt.Errorf("invalid value '%s' for '--head-count <COUNT>': %s", value, err)
		}
	}
	rangeText, isRange := flags.ExplicitString("i")
	var lo, hi uint64
	if isRange {
		from, to, ok := strings.Cut(rangeText, "-")
		if !ok {
			err = errors.New("missing '-'")
		} else {
			lo, err = shufUint(from)
			if err == nil {
				hi, err = shufUint(to)
			}
			if err == nil && lo > hi && lo-hi != 1 {
				err = errors.New("start exceeds end")
			}
		}
		if err != nil {
			return fmt.Errorf("invalid value '%s' for '--input-range <LO-HI>': %s", rangeText, err)
		}
		if flags.Bool("e") || len(flags.Operands) > 0 {
			return errors.New("the argument '--input-range <LO-HI>' cannot be used with '--echo' or input files")
		}
	}
	if !flags.Bool("e") && !isRange && len(flags.Operands) > 1 {
		return fmt.Errorf("unexpected argument '%s' found", flags.Operands[1])
	}
	seed, hasSeed := flags.ExplicitString("random-seed")
	source, hasSource := flags.ExplicitString("random-source")
	if hasSeed && hasSource {
		return errors.New("the argument '--random-seed <STRING>' cannot be used with '--random-source <FILE>'")
	}
	output, hasOutput := flags.ExplicitString("o")
	openOutput := func() (io.Writer, *os.File, error) {
		if !hasOutput {
			return ctx.Stdout, nil, nil
		}
		f, e := os.Create(resolvePath(ctx, output))
		if e != nil {
			return nil, nil, fmt.Errorf("failed to open '%s' for writing: %s", output, fmtOsErr(e))
		}
		return f, f, nil
	}
	if count == 0 {
		_, f, e := openOutput()
		if f != nil {
			if closeErr := f.Close(); e == nil {
				e = closeErr
			}
		}
		return e
	}
	var rng shufRNG
	if hasSource {
		f, e := os.Open(resolvePath(ctx, source))
		if e != nil {
			return fmt.Errorf("failed to open random source '%s': %s", source, fmtOsErr(e))
		}
		defer f.Close()
		rng.source = bufio.NewReader(f)
	} else {
		var key [32]byte
		if hasSeed {
			key = sha3.Sum256([]byte(seed))
		} else {
			if _, err = cryptorand.Read(key[:]); err != nil {
				return err
			}
		}
		rng.chacha = rand.NewChaCha8(key)
	}
	var lines []string
	if flags.Bool("e") {
		lines = append([]string(nil), flags.Operands...)
	} else if !isRange {
		name := "-"
		if len(flags.Operands) > 0 {
			name = flags.Operands[0]
		}
		var data []byte
		if name == "-" {
			data, err = io.ReadAll(ctx.Stdin)
		} else {
			data, err = os.ReadFile(resolvePath(ctx, name))
		}
		if err != nil {
			return fmt.Errorf("%s: %s", name, fmtOsErr(err))
		}
		sep := "\n"
		if flags.Bool("z") {
			sep = "\x00"
		}
		lines = strings.Split(string(data), sep)
		if lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
	}
	out, f, err := openOutput()
	if err != nil {
		return err
	}
	if f != nil {
		defer f.Close()
	}
	writer := bufio.NewWriter(out)
	sep := "\n"
	if flags.Bool("z") {
		sep = "\x00"
	}
	write := func(s string) error {
		if _, e := writer.WriteString(s + sep); e != nil {
			return fmt.Errorf("write failed: %s", fmtOsErr(e))
		}
		return nil
	}
	var maximum uint64
	empty := false
	if isRange {
		empty = lo > hi
		maximum = hi - lo
	} else {
		empty = len(lines) == 0
		maximum = uint64(len(lines)) - 1
	}
	if empty {
		if flags.Bool("r") {
			return errors.New("no lines to repeat")
		}
		return nil
	}
	if !flags.Bool("r") && count-1 > maximum {
		count = maximum + 1
	}
	if !flags.Bool("r") && isRange && count > uint64(^uint(0)>>1)/32 {
		return errors.New("memory exhausted")
	}
	// Line inputs are shuffled before writing, while numeric ranges stay sparse.
	if !flags.Bool("r") && !isRange {
		for i := uint64(0); i < count; i++ {
			offset, e := rng.atMost(maximum - i)
			if e != nil {
				return e
			}
			j := i + offset
			lines[i], lines[j] = lines[j], lines[i]
		}
	}
	swaps := make(map[uint64]uint64)
	for i := uint64(0); i < count; i++ {
		var value string
		if flags.Bool("r") {
			offset, e := rng.atMost(maximum)
			if e != nil {
				if flushErr := writer.Flush(); flushErr != nil {
					return fmt.Errorf("write failed: %s", fmtOsErr(flushErr))
				}
				return e
			}
			if isRange {
				value = strconv.FormatUint(lo+offset, 10)
			} else {
				value = lines[offset]
			}
		} else if isRange {
			offset, e := rng.atMost(maximum - i)
			if e != nil {
				if flushErr := writer.Flush(); flushErr != nil {
					return fmt.Errorf("write failed: %s", fmtOsErr(flushErr))
				}
				return e
			}
			j := i + offset
			this, ok := swaps[i]
			if !ok {
				this = i
			}
			delete(swaps, i)
			chosen := this
			if i != j {
				var exists bool
				chosen, exists = swaps[j]
				if !exists {
					chosen = j
				}
				swaps[j] = this
			}
			value = strconv.FormatUint(lo+chosen, 10)
		} else {
			value = lines[i]
		}
		if err = write(value); err != nil {
			return err
		}
	}
	if err = writer.Flush(); err != nil {
		return fmt.Errorf("write failed: %s", fmtOsErr(err))
	}
	return nil
}

func shufUint(s string) (uint64, error) {
	v, err := strconv.ParseUint(strings.TrimPrefix(s, "+"), 10, 64)
	if err == nil {
		return v, nil
	}
	if s == "" {
		return 0, errors.New("cannot parse integer from empty string")
	}
	if errors.Is(err, strconv.ErrRange) {
		return 0, errors.New("number too large to fit in target type")
	}
	return 0, errors.New("invalid digit found in string")
}

type shufRNG struct {
	chacha         *rand.ChaCha8
	source         *bufio.Reader
	state, entropy uint64
}

func (r *shufRNG) atMost(maximum uint64) (uint64, error) {
	if r.source != nil {
		for {
			for r.entropy < maximum {
				b, err := r.source.ReadByte()
				if err == io.EOF {
					return 0, errors.New("end of random source")
				}
				if err != nil {
					return 0, fmt.Errorf("read random bytes: %s", fmtOsErr(err))
				}
				r.state = r.state*256 + uint64(b)
				r.entropy = r.entropy*256 + 255
			}
			if maximum == ^uint64(0) {
				v := r.state
				r.state = 0
				r.entropy = 0
				return v, nil
			}
			n := maximum + 1
			margin := (r.entropy%n + 1) % n
			if r.state <= r.entropy-margin {
				v := r.state % n
				r.state /= n
				r.entropy = (r.entropy - maximum) / n
				return v, nil
			}
			r.state %= n
			r.entropy %= n
		}
	}
	if maximum == ^uint64(0) {
		return r.chacha.Uint64(), nil
	}
	n := maximum + 1
	hi, lo := bits.Mul64(r.chacha.Uint64(), n)
	if lo < n {
		threshold := -n % n
		for lo < threshold {
			hi, lo = bits.Mul64(r.chacha.Uint64(), n)
		}
	}
	return hi, nil
}
