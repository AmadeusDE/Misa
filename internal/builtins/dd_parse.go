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

// ddConv holds the conv= flags. table is the character-set and case
// conversion the flags resolve to, and block/unblock are set only when cbs=
// gives the record length they need.
type ddConv struct {
	notrunc   bool
	sync      bool
	noerror   bool
	fsync     bool
	fdatasync bool
	excl      bool
	nocreat   bool
	swab      bool
	sparse    bool

	ascii, ebcdic, ibm bool
	ucase, lcase       bool
	block, unblock     bool

	table *[256]byte
}

// ddIFlags holds iflag=.
type ddIFlags struct {
	countBytes bool
	skipBytes  bool
	fullblock  bool
	direct     bool     // iflag=direct: buffers must be aligned
	extra      int      // platform open flags such as O_DIRECT
	names      []string // the platform flag names that were given
}

// ddOFlags holds oflag=.
type ddOFlags struct {
	seekBytes bool
	append    bool
	direct    bool     // oflag=direct: buffers must be aligned
	extra     int      // platform open flags such as O_DIRECT
	names     []string // the platform flag names that were given
}

type ddStatus int

const (
	ddStatusDefault ddStatus = iota
	ddStatusNone
	ddStatusNoxfer
	ddStatusProgress
)

func (s ddStatus) records() bool  { return s != ddStatusNone }
func (s ddStatus) xfer() bool     { return s != ddStatusNone && s != ddStatusNoxfer }
func (s ddStatus) progress() bool { return s == ddStatusProgress }

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

	cbs    int // conv=block/unblock record size
	cbsSet bool

	conv   ddConv
	iflag  ddIFlags
	oflag  ddOFlags
	status ddStatus
}

// ddUnbuffered reports whether short reads go straight out as partial records.
// With bs= and no conversion, dd passes short blocks along; otherwise it
// gathers input blocks into obs= sized output blocks.
func (c *ddConfig) ddUnbuffered() bool { return c.bsSet && !c.convSet }

// ddPlatformFlagNames are the iflag=/oflag= names that ask the kernel for
// special open behaviour. Each platform file maps the ones it can honour onto
// os.OpenFile flags; the rest fail with "unsupported on this platform".
var ddPlatformFlagNames = map[string]bool{
	"direct": true, "directory": true, "dsync": true, "sync": true,
	"nonblock": true, "noatime": true, "noctty": true, "nofollow": true,
}

// ddReservedFlags are flags the references define for operations this dd does
// not implement at all, on any platform: the cache advice, the raw tape mode
// and the Windows text/binary modes.
var ddReservedFlags = map[string]bool{
	"nocache": true, "cio": true, "nolinks": true, "binary": true, "text": true,
}

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
			n, err := ddParseBlockSize(key, value)
			if err != nil {
				return nil, err
			}
			cfg.cbs, cfg.cbsSet = int(n), true
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
				cfg.status = ddStatusProgress
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
	if err := cfg.resolveConv(); err != nil {
		return nil, err
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
		case "swab":
			c.conv.swab = true
		case "sparse":
			c.conv.sparse = true
		case "ascii":
			c.conv.ascii = true
		case "ebcdic":
			c.conv.ebcdic = true
		case "ibm":
			c.conv.ibm = true
		case "ucase":
			c.conv.ucase = true
		case "lcase":
			c.conv.lcase = true
		case "block":
			c.conv.block = true
		case "unblock":
			c.conv.unblock = true
		default:
			return ddUsagef("invalid conversion: '%s'", f)
		}
	}
	return nil
}

// resolveConv rejects the conv= combinations the references refuse and works
// out which character table and which blocking step the copy uses.
func (c *ddConfig) resolveConv() error {
	conv := &c.conv
	// The order of these refusals follows the references: the character set
	// first, then the blocking direction, then the case fold, then the output
	// file rules.
	charsets := 0
	for _, set := range []bool{conv.ascii, conv.ebcdic, conv.ibm} {
		if set {
			charsets++
		}
	}
	if charsets > 1 {
		return fmt.Errorf("cannot combine ascii, ebcdic and ibm")
	}

	// conv=ascii implies conv=unblock and conv=ebcdic/ibm imply conv=block;
	// with no cbs= there is no record length and the implied step is skipped.
	blocked := conv.block || conv.ebcdic || conv.ibm
	unblocked := conv.unblock || conv.ascii
	if c.cbsSet && blocked && unblocked {
		return fmt.Errorf("cannot combine block and unblock")
	}
	switch {
	case conv.ucase && conv.lcase:
		return fmt.Errorf("cannot combine ucase and lcase")
	case conv.excl && conv.nocreat:
		return fmt.Errorf("cannot combine excl and nocreat")
	}
	conv.block, conv.unblock = false, false
	if c.cbsSet {
		conv.block, conv.unblock = blocked, unblocked && !blocked
	}
	conv.table = c.convTable()
	return nil
}

// convTable picks the character conversion table conv= asked for, combining
// the character set and the case fold in one lookup the way the references do.
func (c *ddConfig) convTable() *[256]byte {
	conv := &c.conv
	switch {
	case conv.ascii:
		switch {
		case conv.ucase:
			return &ddEBCDICToASCIILowerToUpper
		case conv.lcase:
			return &ddEBCDICToASCIIUpperToLower
		}
		return &ddEBCDICToASCII
	case conv.ebcdic:
		switch {
		case conv.ucase:
			return &ddASCIIToEBCDICLowerToUpper
		case conv.lcase:
			return &ddASCIIToEBCDICUpperToLower
		}
		return &ddASCIIToEBCDIC
	case conv.ibm:
		switch {
		case conv.ucase:
			return &ddASCIIToIBMLowerToUpper
		case conv.lcase:
			return &ddASCIIToIBMUpperToLower
		}
		return &ddASCIIToIBM
	case conv.ucase:
		return ddUpperTable
	case conv.lcase:
		return ddLowerTable
	}
	return nil
}

// syncPad is the byte conv=sync fills a short input block with: an EBCDIC or
// ASCII space when conv=block is active, otherwise a zero byte.
func (c *ddConfig) syncPad() byte {
	if c.conv.block {
		return ' '
	}
	return 0
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
			if ddReservedFlags[f] {
				return ddUnsupported("iflag", f)
			}
			bits, err := ddPlatformOpenFlag(f)
			if err != nil {
				return fmt.Errorf("iflag=%s: %w", f, err)
			}
			if bits == 0 && !ddPlatformFlagNames[f] {
				return ddUsagef("invalid input flag: '%s'", f)
			}
			if f == "direct" {
				c.iflag.direct = true
			}
			c.iflag.extra |= bits
			c.iflag.names = append(c.iflag.names, f)
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
			if ddReservedFlags[f] {
				return ddUnsupported("oflag", f)
			}
			bits, err := ddPlatformOpenFlag(f)
			if err != nil {
				return fmt.Errorf("oflag=%s: %w", f, err)
			}
			if bits == 0 && !ddPlatformFlagNames[f] {
				return ddUsagef("invalid output flag: '%s'", f)
			}
			if f == "direct" {
				c.oflag.direct = true
			}
			c.oflag.extra |= bits
			c.oflag.names = append(c.oflag.names, f)
		}
	}
	return nil
}
