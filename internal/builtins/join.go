// `join`: merge the lines of two files on a common field.
//
// Behaviour, option set, diagnostics and output rules follow uutils/coreutils
// src/uu/join/src/join.rs (MIT), pinned commit
// fee61ff5d4f87e0f912a52e639942b5f7ad58663. See docs/CREDITS.md.
//
// Deliberate differences from uutils: `-n`/`--numeric-sort` is not implemented
// (it rejects as an unknown option), `--ignore-case` folds ASCII only, and
// input is always compared in the C locale byte order.

package builtins

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

const joinSpec = "1:2:a:(check-order)e:(header)i(ignore-case)j:(nocheck-order)o:t:v:z(zero-terminated)"

var joinSpecParsed *Spec

func init() {
	var err error
	joinSpecParsed, err = ParseSpec(joinSpec)
	if err != nil {
		panic("builtins: bad join spec: " + err.Error())
	}
	Register(Command{Name: "join", Run: runJoin})
}

type joinFileNum int

const (
	joinFile1 joinFileNum = 1
	joinFile2 joinFileNum = 2
)

type joinCheckOrder int

const (
	joinCheckDefault joinCheckOrder = iota
	joinCheckOff
	joinCheckOn
)

// joinOutField is one element of -o: either the join field (0) or a numbered
// field of one input.
type joinOutField struct {
	key   bool
	file  joinFileNum
	field uint64
}

type joinSettings struct {
	key1, key2     uint64
	printJoined    bool
	printUnpaired1 bool
	printUnpaired2 bool
	ignoreCase     bool
	zeroTerminated bool
	separator      joinSep
	autoFormat     bool
	format         []joinOutField
	empty          []byte
	checkOrder     joinCheckOrder
	header         bool
}

func runJoin(ctx *Context) int {
	flags, err := joinSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "join:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, joinSpecParsed) {
		return 0
	}

	settings, err := joinParseSettings(flags)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "join:", err)
		return 1
	}

	switch len(flags.Operands) {
	case 0:
		fmt.Fprintln(ctx.Stderr, "join: missing operand")
		fmt.Fprintln(ctx.Stderr, "Try 'join --help' for more information.")
		return 1
	case 1:
		fmt.Fprintf(ctx.Stderr, "join: missing operand after '%s'\n", flags.Operands[0])
		fmt.Fprintln(ctx.Stderr, "Try 'join --help' for more information.")
		return 1
	case 2:
	default:
		fmt.Fprintf(ctx.Stderr, "join: extra operand '%s'\n", flags.Operands[2])
		fmt.Fprintln(ctx.Stderr, "Try 'join --help' for more information.")
		return 1
	}

	file1, file2 := flags.Operands[0], flags.Operands[1]
	if file1 == "-" && file2 == "-" {
		fmt.Fprintln(ctx.Stderr, "join: both files cannot be standard input")
		return 1
	}
	return joinExec(ctx, file1, file2, settings)
}

func joinParseSettings(flags *ParseResult) (*joinSettings, error) {
	st := &joinSettings{
		printJoined: true,
		separator:   joinSep{mode: joinSepWhitespace},
	}

	jKey, jSet, err := joinOptionFieldNumber(flags, "j")
	if err != nil {
		return nil, err
	}
	key1, key1Set, err := joinOptionFieldNumber(flags, "1")
	if err != nil {
		return nil, err
	}
	key2, key2Set, err := joinOptionFieldNumber(flags, "2")
	if err != nil {
		return nil, err
	}
	if st.key1, err = joinCombineFieldNumber(jKey, jSet, key1, key1Set); err != nil {
		return nil, err
	}
	if st.key2, err = joinCombineFieldNumber(jKey, jSet, key2, key2Set); err != nil {
		return nil, err
	}

	if err := st.parsePrintSettings(flags); err != nil {
		return nil, err
	}

	st.ignoreCase = flags.Bool("i")
	st.zeroTerminated = flags.Bool("z")
	st.header = flags.Bool("header")

	if v, ok := flags.ExplicitString("t"); ok {
		st.separator, err = joinParseSeparator(v)
		if err != nil {
			return nil, err
		}
	}
	if err := st.parseFormat(flags); err != nil {
		return nil, err
	}
	if err := st.parseEmpty(flags); err != nil {
		return nil, err
	}

	if flags.Bool("nocheck-order") {
		st.checkOrder = joinCheckOff
	}
	if flags.Bool("check-order") {
		st.checkOrder = joinCheckOn
	}
	return st, nil
}

func (st *joinSettings) parsePrintSettings(flags *ParseResult) error {
	vValues := flags.AllStrings("v")
	if len(vValues) > 0 {
		st.printJoined = false
	}
	for _, v := range append(append([]string{}, vValues...), flags.AllStrings("a")...) {
		switch v {
		case "1":
			st.printUnpaired1 = true
		case "2":
			st.printUnpaired2 = true
		default:
			return fmt.Errorf("invalid file number: '%s'", v)
		}
	}
	return nil
}

func (st *joinSettings) parseFormat(flags *ParseResult) error {
	values := flags.AllStrings("o")
	if len(values) == 0 {
		return nil
	}
	all := true
	for _, v := range values {
		if v != "auto" {
			all = false
			break
		}
	}
	if all {
		st.autoFormat = true
		return nil
	}
	for _, v := range values {
		if v == "auto" {
			continue
		}
		items := joinSplitFormat(v)
		for _, item := range items {
			spec, err := joinParseOutField(item)
			if err != nil {
				return err
			}
			st.format = append(st.format, spec)
		}
	}
	return nil
}

// joinSplitFormat splits one -o value on blanks, commas and tabs, the way
// uutils' list_items does. An empty value is a single empty field, so that
// `-o ”` is rejected instead of silently ignored.
func joinSplitFormat(value string) []string {
	items := strings.FieldsFunc(value, func(r rune) bool {
		return r == ' ' || r == ',' || r == '\t'
	})
	if len(items) == 0 {
		return []string{""}
	}
	return items
}

func joinParseOutField(format string) (joinOutField, error) {
	if format == "0" {
		return joinOutField{key: true}, nil
	}
	file, rest, err := joinOutFieldFile(format)
	if err != nil {
		return joinOutField{}, err
	}
	if !strings.HasPrefix(rest, ".") {
		return joinOutField{}, fmt.Errorf("invalid field specifier: '%s'", format)
	}
	field, err := joinFieldNumber(rest[1:])
	if err != nil {
		return joinOutField{}, err
	}
	return joinOutField{file: file, field: field}, nil
}

func joinOutFieldFile(format string) (joinFileNum, string, error) {
	if strings.HasPrefix(format, "0") {
		return 0, "", fmt.Errorf("invalid field specifier: '%s'", format)
	}
	if strings.HasPrefix(format, "1") {
		return joinFile1, format[1:], nil
	}
	if strings.HasPrefix(format, "2") {
		return joinFile2, format[1:], nil
	}
	return 0, "", fmt.Errorf("invalid file number in field spec: '%s'", format)
}

func (st *joinSettings) parseEmpty(flags *ParseResult) error {
	values := flags.AllStrings("e")
	if len(values) == 0 {
		return nil
	}
	for _, v := range values[1:] {
		if v != values[0] {
			return errors.New("conflicting empty-field replacement strings")
		}
	}
	st.empty = []byte(values[0])
	return nil
}

func joinOptionFieldNumber(flags *ParseResult, name string) (uint64, bool, error) {
	v, ok := flags.ExplicitString(name)
	if !ok {
		return 0, false, nil
	}
	n, err := joinFieldNumber(v)
	if err != nil {
		return 0, false, err
	}
	return n, true, nil
}

func joinCombineFieldNumber(shared uint64, sharedSet bool, own uint64, ownSet bool) (uint64, error) {
	if sharedSet && ownSet && shared != own {
		return 0, fmt.Errorf("incompatible join fields %d, %d", joinOneBased(shared), joinOneBased(own))
	}
	if sharedSet {
		return shared, nil
	}
	if ownSet {
		return own, nil
	}
	return 0, nil
}

// joinOneBased shows a zero-based field number the way the input spelled it.
// Field numbers above the addressable range saturate, so the increment must
// too.
func joinOneBased(field uint64) uint64 {
	if field == math.MaxUint64 {
		return field
	}
	return field + 1
}

// joinFieldNumber parses a one-based field number into a zero-based index.
func joinFieldNumber(value string) (uint64, error) {
	n, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		if errors.Is(err, strconv.ErrRange) {
			return math.MaxUint64, nil
		}
		return 0, fmt.Errorf("invalid field number: '%s'", value)
	}
	if n == 0 {
		return 0, fmt.Errorf("invalid field number: '%s'", value)
	}
	return n - 1, nil
}

func joinParseSeparator(value string) (joinSep, error) {
	if value == "" {
		return joinSep{mode: joinSepLine}, nil
	}
	if len(value) == 1 {
		return joinSep{mode: joinSepBytes, sep: []byte{value[0]}}, nil
	}
	if !utf8.ValidString(value) {
		return joinSep{}, errors.New("non-UTF-8 multi-byte tab")
	}
	if _, size := utf8.DecodeRuneInString(value); size == len(value) {
		return joinSep{mode: joinSepBytes, sep: []byte(value)}, nil
	}
	if value == `\0` {
		return joinSep{mode: joinSepBytes, sep: []byte{0}}, nil
	}
	return joinSep{}, fmt.Errorf("multi-character tab '%s'", value)
}
