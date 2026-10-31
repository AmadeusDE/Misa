// dd operand parsing.
//
// Sizes, suffixes and the operand names follow toybox dd (toys/posix/dd.c,
// 0BSD) and uutils coreutils dd (src/uu/dd/src/parseargs.rs, MIT); both at the
// commits pinned in docs/CREDITS.md.
package builtins

import (
	"errors"
	"fmt"
	"strings"
)

// errDDTooLarge marks a number past what dd can use, as uutils and GNU report
// it: the operand is well formed but too big for the type.
var errDDTooLarge = errors.New("Value too large for defined data type")

// ddUsageError is an operand dd rejected; dd adds the "Try '--help'" hint.
type ddUsageError struct{ msg string }

func (e *ddUsageError) Error() string { return e.msg }

func ddUsagef(format string, args ...any) error {
	return &ddUsageError{msg: fmt.Sprintf(format, args...)}
}

// ddMaxBlockSize bounds bs=/ibs=/obs= so a typo cannot ask for an absurd
// buffer; uutils rejects anything that does not fit.
const ddMaxBlockSize = 1 << 30

// ddConv holds the conv= flags this version implements. U17 (conv=ucase,
// lcase, swab, ascii, ebcdic, block, unblock, sparse) adds fields here and a
// transform call in the copy loop.
type ddConv struct {
	notrunc   bool
	sync      bool
	noerror   bool
	fsync     bool
	fdatasync bool
	excl      bool
	nocreat   bool
}

// ddIFlags holds iflag=.
type ddIFlags struct {
	countBytes bool
	skipBytes  bool
	fullblock  bool
}

// ddOFlags holds oflag=.
type ddOFlags struct {
	seekBytes bool
	append    bool
}

type ddStatus int

const (
	ddStatusDefault ddStatus = iota
	ddStatusNone
	ddStatusNoxfer
)

func (s ddStatus) records() bool { return s != ddStatusNone }
func (s ddStatus) xfer() bool    { return s == ddStatusDefault }

// ddNumber is a parsed count=/skip=/seek= value. A "B" anywhere in the operand
// makes it a byte count rather than a block count, as in uutils and GNU dd.
type ddNumber struct {
	value uint64
	bytes bool
}

// ddConfig is everything the operands said, already resolved to bytes.
type ddConfig struct {
	inSet   bool
	inName  string
	outSet  bool
	outName string

	bsSet    bool
	ibs, obs int
	convSet  bool

	countSet bool
	count    uint64 // bytes to read; only meaningful when countSet

	skip uint64 // bytes to discard from the input
	seek uint64 // bytes to skip on the output

	conv   ddConv
	iflag  ddIFlags
	oflag  ddOFlags
	status ddStatus
}

// ddUnbuffered reports whether short reads go straight out as partial records.
// With bs= and no conversion, dd passes short blocks along; otherwise it
// gathers input blocks into obs= sized output blocks.
func (c *ddConfig) ddUnbuffered() bool { return c.bsSet && !c.convSet }

// knownUnsupported lists the flags the references define but this version does
// not implement, so they fail with a clear message instead of being ignored.
var ddConvUnsupported = map[string]bool{
	"ascii": true, "ebcdic": true, "ibm": true, "lcase": true, "ucase": true,
	"block": true, "unblock": true, "swab": true, "sparse": true,
}

var ddIUnsupported = map[string]bool{
	"direct": true, "directory": true, "dsync": true, "sync": true,
	"nocache": true, "nonblock": true, "noatime": true, "noctty": true,
	"nofollow": true, "cio": true, "nolinks": true, "binary": true, "text": true,
}

var ddOUnsupported = ddIUnsupported

func ddUnsupported(name, value string) error {
	return fmt.Errorf("%s=%s: unsupported in this version", name, value)
}

func ddInvalidNumber(value string) error {
	return fmt.Errorf("invalid number: '%s'", value)
}

// parseDDArgs turns the key=value operands into a config, or the first error.
func parseDDArgs(args []string) (*ddConfig, error) {
	cfg := &ddConfig{ibs: 512, obs: 512}

	var bs uint64
	var skip, seek ddNumber
	var count ddNumber

	for _, arg := range args {
		key, value, ok := strings.Cut(arg, "=")
		if !ok {
			return nil, ddUsagef("unrecognized operand '%s'", arg)
		}
		switch key {
		case "if":
			cfg.inSet, cfg.inName = true, value
		case "of":
			cfg.outSet, cfg.outName = true, value
		case "bs":
			n, err := ddParseBlockSize(key, value)
			if err != nil {
				return nil, err
			}
			bs, cfg.bsSet = n, true
		case "ibs":
			n, err := ddParseBlockSize(key, value)
			if err != nil {
				return nil, err
			}
			cfg.ibs = int(n)
		case "obs":
			n, err := ddParseBlockSize(key, value)
			if err != nil {
				return nil, err
			}
			cfg.obs = int(n)
		case "cbs":
			// cbs= only means something with conv=block/unblock, which U17 adds.
			if _, err := ddParseBlockSize(key, value); err != nil {
				return nil, err
			}
			return nil, ddUnsupported(key, value)
		case "count":
			n, err := ddParseNum(value)
			if err != nil {
				return nil, err
			}
			count, cfg.countSet = n, true
		case "skip", "iseek":
			n, err := ddParseNum(value)
			if err != nil {
				return nil, err
			}
			skip = n
		case "seek", "oseek":
			n, err := ddParseNum(value)
			if err != nil {
				return nil, err
			}
			seek = n
		case "conv":
			cfg.convSet = true
			if err := cfg.parseConvFlags(value); err != nil {
				return nil, err
			}
		case "iflag":
			if err := cfg.parseInputFlags(value); err != nil {
				return nil, err
			}
		case "oflag":
			if err := cfg.parseOutputFlags(value); err != nil {
				return nil, err
			}
		case "status":
			switch value {
			case "none":
				cfg.status = ddStatusNone
			case "noxfer":
				cfg.status = ddStatusNoxfer
			case "progress":
				return nil, ddUnsupported(key, value)
			default:
				return nil, ddUsagef("invalid status level: '%s'", value)
			}
		default:
			return nil, ddUsagef("unrecognized operand '%s'", arg)
		}
	}

	if cfg.bsSet {
		cfg.ibs, cfg.obs = int(bs), int(bs)
	}

	if cfg.countSet {
		if count.bytes || cfg.iflag.countBytes {
			cfg.count = count.value
		} else {
			n, ok := ddMul(count.value, uint64(cfg.ibs))
			if !ok {
				return nil, errDDTooLarge
			}
			cfg.count = n
		}
	}
	var err error
	if cfg.skip, err = ddResolve(skip, cfg.iflag.skipBytes, uint64(cfg.ibs)); err != nil {
		return nil, err
	}
	if cfg.seek, err = ddResolve(seek, cfg.oflag.seekBytes, uint64(cfg.obs)); err != nil {
		return nil, err
	}
	return cfg, nil
}

// ddParseBlockSize parses bs=/ibs=/obs=/cbs=, which must fit in a buffer.
func ddParseBlockSize(name, value string) (uint64, error) {
	n, err := ddParseNumber(value)
	if err != nil {
		return 0, err
	}
	switch {
	case n == 0:
		return 0, ddInvalidNumber(value)
	case n > ddMaxBlockSize:
		return 0, fmt.Errorf("%s=N cannot fit into memory", name)
	}
	return n, nil
}

// ddResolve converts a skip=/seek= value to bytes, blocking on the matching
// block size unless the operand counted bytes.
func ddResolve(n ddNumber, bytesFlag bool, blockSize uint64) (uint64, error) {
	if n.bytes || bytesFlag {
		return n.value, nil
	}
	product, ok := ddMul(n.value, blockSize)
	if !ok {
		return 0, errDDTooLarge
	}
	return product, nil
}

func (c *ddConfig) parseConvFlags(value string) error {
	for _, f := range strings.Split(value, ",") {
		switch f {
		case "notrunc":
			c.conv.notrunc = true
		case "sync":
			c.conv.sync = true
		case "noerror":
			c.conv.noerror = true
		case "fsync":
			c.conv.fsync = true
		case "fdatasync":
			c.conv.fdatasync = true
		case "excl":
			c.conv.excl = true
		case "nocreat":
			c.conv.nocreat = true
		default:
			if ddConvUnsupported[f] {
				return ddUnsupported("conv", f)
			}
			return ddUsagef("invalid conversion: '%s'", f)
		}
	}
	if c.conv.excl && c.conv.nocreat {
		return fmt.Errorf("cannot combine excl and nocreat")
	}
	return nil
}

func (c *ddConfig) parseInputFlags(value string) error {
	for _, f := range strings.Split(value, ",") {
		switch f {
		case "count_bytes":
			c.iflag.countBytes = true
		case "skip_bytes":
			c.iflag.skipBytes = true
		case "fullblock":
			c.iflag.fullblock = true
		case "append", "seek_bytes":
			// GNU accepts output flags given as iflag and ignores them.
		default:
			if ddIUnsupported[f] {
				return ddUnsupported("iflag", f)
			}
			return ddUsagef("invalid input flag: '%s'", f)
		}
	}
	return nil
}

func (c *ddConfig) parseOutputFlags(value string) error {
	for _, f := range strings.Split(value, ",") {
		switch f {
		case "seek_bytes":
			c.oflag.seekBytes = true
		case "append":
			c.oflag.append = true
		case "count_bytes", "skip_bytes", "fullblock":
			// Input flags given as oflag are ignored, as GNU does.
		default:
			if ddOUnsupported[f] {
				return ddUnsupported("oflag", f)
			}
			return ddUsagef("invalid output flag: '%s'", f)
		}
	}
	return nil
}
