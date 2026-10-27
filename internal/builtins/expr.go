package builtins

import (
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

func init() {
	Register(Command{Name: "expr", Run: runExpr})
}

func runExpr(ctx *Context) int {
	args := ctx.Args
	if len(args) == 0 {
		fmt.Fprintln(ctx.Stderr, "expr: missing operand")
		return 2
	}
	// Strip leading -- (end-of-options marker)
	if args[0] == "--" {
		args = args[1:]
		if len(args) == 0 {
			fmt.Fprintln(ctx.Stderr, "expr: missing operand")
			return 2
		}
	}

	p := &exprParser{tokens: args}
	result, err := p.parseOr()
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "expr: %v\n", err)
		return 2
	}
	if p.pos < len(p.tokens) {
		fmt.Fprintf(ctx.Stderr, "expr: syntax error: unexpected argument '%s'\n", p.tokens[p.pos])
		return 2
	}

	fmt.Fprintln(ctx.Stdout, result)

	if exprIsNull(result) {
		return 1
	}
	return 0
}

type exprParser struct {
	tokens []string
	pos    int
}

func (p *exprParser) peek() string {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return ""
}

func (p *exprParser) next() string {
	t := p.peek()
	p.pos++
	return t
}

func (p *exprParser) parseOr() (string, error) {
	left, err := p.parseAnd()
	if err != nil {
		return "", err
	}
	for p.peek() == "|" {
		op := p.next()
		if exprIsNull(left) {
			right, err := p.parseAnd()
			if err != nil {
				if err.Error() == "missing operand" {
					return "", fmt.Errorf("syntax error: missing argument after '%s'", op)
				}
				return "", err
			}
			if exprIsNull(right) {
				left = "0"
			} else {
				left = right
			}
		} else {
			p.skipAndExpr()
		}
	}
	return left, nil
}

func (p *exprParser) parseAnd() (string, error) {
	left, err := p.parseCompare()
	if err != nil {
		return "", err
	}
	for p.peek() == "&" {
		op := p.next()
		if exprIsNull(left) {
			p.skipCompareExpr()
			left = "0"
		} else {
			right, err := p.parseCompare()
			if err != nil {
				if err.Error() == "missing operand" {
					return "", fmt.Errorf("syntax error: missing argument after '%s'", op)
				}
				return "", err
			}
			if exprIsNull(right) {
				left = "0"
			}
		}
	}
	return left, nil
}

func (p *exprParser) skipAndExpr() {
	for {
		p.skipCompareExpr()
		if p.peek() != "&" {
			break
		}
		p.next()
	}
}

func (p *exprParser) skipCompareExpr() {
	p.skipAddExpr()
	for {
		op := p.peek()
		if op != "=" && op != "!=" && op != "<" && op != "<=" && op != ">" && op != ">=" {
			break
		}
		p.next()
		p.skipAddExpr()
	}
}

func (p *exprParser) skipAddExpr() {
	p.skipMulExpr()
	for {
		op := p.peek()
		if op != "+" && op != "-" {
			break
		}
		p.next()
		p.skipMulExpr()
	}
}

func (p *exprParser) skipMulExpr() {
	p.skipMatchExpr()
	for {
		op := p.peek()
		if op != "*" && op != "/" && op != "%" {
			break
		}
		p.next()
		p.skipPrimary()
	}
}

func (p *exprParser) skipMatchExpr() {
	p.skipPrimary()
	for p.peek() == ":" {
		p.next()
		p.skipPrimary()
	}
}

func (p *exprParser) skipPrimary() {
	tok := p.peek()
	if tok == "(" {
		p.next()
		depth := 1
		for p.pos < len(p.tokens) && depth > 0 {
			t := p.next()
			if t == "(" {
				depth++
			} else if t == ")" {
				depth--
			}
		}
		return
	}
	switch tok {
	case "length":
		p.next()
		p.skipPrimary()
	case "substr":
		p.next()
		p.skipPrimary()
		p.skipPrimary()
		p.skipPrimary()
	case "match", "index":
		p.next()
		p.skipPrimary()
		p.skipPrimary()
	default:
		if p.pos < len(p.tokens) {
			p.next()
		}
	}
}

func (p *exprParser) parseCompare() (string, error) {
	left, err := p.parseAdd()
	if err != nil {
		return "", err
	}
	for {
		op := p.peek()
		switch op {
		case "=", "!=", "<", "<=", ">", ">=":
			p.next()
			right, err := p.parseAdd()
			if err != nil {
				if err.Error() == "missing operand" {
					return "", fmt.Errorf("syntax error: missing argument after '%s'", op)
				}
				return "", err
			}
			left = exprCompare(left, right, op)
		default:
			return left, nil
		}
	}
}

func (p *exprParser) parseAdd() (string, error) {
	left, err := p.parseMul()
	if err != nil {
		return "", err
	}
	for {
		op := p.peek()
		if op != "+" && op != "-" {
			return left, nil
		}
		p.next()
		right, err := p.parseMul()
		if err != nil {
			if err.Error() == "missing operand" {
				return "", fmt.Errorf("syntax error: missing argument after '%s'", op)
			}
			return "", err
		}
		l, ok1 := new(big.Int).SetString(left, 10)
		r, ok2 := new(big.Int).SetString(right, 10)
		if !ok1 || !ok2 {
			return "", fmt.Errorf("non-integer argument")
		}
		result := new(big.Int)
		if op == "+" {
			result.Add(l, r)
		} else {
			result.Sub(l, r)
		}
		left = result.String()
	}
}

func (p *exprParser) parseMul() (string, error) {
	left, err := p.parseMatch()
	if err != nil {
		return "", err
	}
	for {
		op := p.peek()
		if op != "*" && op != "/" && op != "%" {
			return left, nil
		}
		p.next()
		right, err := p.parseUnary()
		if err != nil {
			if err.Error() == "missing operand" {
				return "", fmt.Errorf("syntax error: missing argument after '%s'", op)
			}
			return "", err
		}
		l, ok1 := new(big.Int).SetString(left, 10)
		r, ok2 := new(big.Int).SetString(right, 10)
		if !ok1 || !ok2 {
			return "", fmt.Errorf("non-integer argument")
		}
		result := new(big.Int)
		switch op {
		case "*":
			result.Mul(l, r)
		case "/":
			if r.Sign() == 0 {
				return "", fmt.Errorf("division by zero")
			}
			result.Quo(l, r)
		case "%":
			if r.Sign() == 0 {
				return "", fmt.Errorf("division by zero")
			}
			result.Rem(l, r)
		}
		left = result.String()
	}
}

func (p *exprParser) parseMatch() (string, error) {
	left, err := p.parseUnary()
	if err != nil {
		return "", err
	}
	for p.peek() == ":" {
		op := p.next()
		right, err := p.parseUnary()
		if err != nil {
			if err.Error() == "missing operand" {
				return "", fmt.Errorf("syntax error: missing argument after '%s'", op)
			}
			return "", err
		}
		left, err = exprMatch(left, right)
		if err != nil {
			return "", err
		}
	}
	return left, nil
}

func (p *exprParser) parseUnary() (string, error) {
	tok := p.peek()

	if tok == "+" {
		p.next()
		return p.parseUnary()
	}

	if tok == "(" {
		p.next()
		val, err := p.parseOr()
		if p.peek() != ")" {
			if p.pos < len(p.tokens) {
				return "", fmt.Errorf("syntax error: expecting ')' instead of '%s'", p.tokens[p.pos])
			}
			if p.pos > 0 {
				return "", fmt.Errorf("syntax error: expecting ')' after '%s'", p.tokens[p.pos-1])
			}
			return "", fmt.Errorf("syntax error: expecting ')' after '('")
		}
		if err != nil {
			return "", err
		}
		p.next()
		return val, nil
	}

	if tok == "length" {
		p.next()
		arg, err := p.parsePrimary()
		if err != nil {
			if err.Error() == "missing operand" {
				return "", fmt.Errorf("syntax error: missing argument after 'length'")
			}
			return "", err
		}
		return strconv.Itoa(utf8.RuneCountInString(arg)), nil
	}

	if tok == "substr" {
		p.next()
		str, err := p.parsePrimary()
		if err != nil {
			if err.Error() == "missing operand" {
				return "", fmt.Errorf("syntax error: missing argument after 'substr'")
			}
			return "", err
		}
		posStr, err := p.parsePrimary()
		if err != nil {
			if err.Error() == "missing operand" {
				return "", fmt.Errorf("syntax error: missing argument after 'substr'")
			}
			return "", err
		}
		lenStr, err := p.parsePrimary()
		if err != nil {
			if err.Error() == "missing operand" {
				return "", fmt.Errorf("syntax error: missing argument after 'substr'")
			}
			return "", err
		}
		pos, err1 := strconv.Atoi(posStr)
		length, err2 := strconv.Atoi(lenStr)
		if err1 != nil || err2 != nil {
			return "", fmt.Errorf("non-integer argument")
		}
		runes := []rune(str)
		if pos < 1 || length <= 0 {
			return "", nil
		}
		pos-- // 1-indexed
		if pos >= len(runes) {
			return "", nil
		}
		end := pos + length
		if end > len(runes) {
			end = len(runes)
		}
		return string(runes[pos:end]), nil
	}

	if tok == "match" {
		p.next()
		str, err := p.parsePrimary()
		if err != nil {
			if err.Error() == "missing operand" {
				return "", fmt.Errorf("syntax error: missing argument after 'match'")
			}
			return "", err
		}
		pat, err := p.parsePrimary()
		if err != nil {
			if err.Error() == "missing operand" {
				return "", fmt.Errorf("syntax error: missing argument after 'match'")
			}
			return "", err
		}
		return exprMatch(str, pat)
	}

	if tok == "index" {
		p.next()
		str, err := p.parsePrimary()
		if err != nil {
			if err.Error() == "missing operand" {
				return "", fmt.Errorf("syntax error: missing argument after 'index'")
			}
			return "", err
		}
		chars, err := p.parsePrimary()
		if err != nil {
			if err.Error() == "missing operand" {
				return "", fmt.Errorf("syntax error: missing argument after 'index'")
			}
			return "", err
		}
		for i, c := range []rune(str) {
			if strings.ContainsRune(chars, c) {
				return strconv.Itoa(i + 1), nil
			}
		}
		return "0", nil
	}

	return p.parsePrimary()
}

func (p *exprParser) parsePrimary() (string, error) {
	if p.pos >= len(p.tokens) {
		return "", fmt.Errorf("missing operand")
	}
	return p.next(), nil
}

func exprIsNull(s string) bool {
	if s == "" {
		return true
	}
	n, ok := new(big.Int).SetString(s, 10)
	return ok && n.Sign() == 0
}

func exprCompare(left, right, op string) string {
	lNum, lok := new(big.Int).SetString(left, 10)
	rNum, rok := new(big.Int).SetString(right, 10)

	var result bool
	if lok && rok {
		cmp := lNum.Cmp(rNum)
		switch op {
		case "=":
			result = cmp == 0
		case "!=":
			result = cmp != 0
		case "<":
			result = cmp < 0
		case "<=":
			result = cmp <= 0
		case ">":
			result = cmp > 0
		case ">=":
			result = cmp >= 0
		}
	} else {
		cmp := strings.Compare(left, right)
		switch op {
		case "=":
			result = cmp == 0
		case "!=":
			result = cmp != 0
		case "<":
			result = cmp < 0
		case "<=":
			result = cmp <= 0
		case ">":
			result = cmp > 0
		case ">=":
			result = cmp >= 0
		}
	}

	if result {
		return "1"
	}
	return "0"
}

// breToERE converts a POSIX BRE pattern to a Go RE2-compatible pattern.
func breToERE(pattern string) (string, map[int]int, error) {
	var out strings.Builder
	n := len(pattern)
	atExprStart := true
	afterCaretAnchor := false
	parenDepth := 0
	prevWasQuantifier := false
	lastAtomStart := 0

	groupCount := 0
	groupPatterns := make(map[int]string)
	groupStack := []int{}
	groupStartPos := make(map[int]int)
	backrefChecks := make(map[int]int)

	appendAtom := func(s string) {
		lastAtomStart = out.Len()
		out.WriteString(s)
		atExprStart = false
		afterCaretAnchor = false
		prevWasQuantifier = false
	}

	appendQuantifier := func(quant string) {
		if prevWasQuantifier {
			prev := out.String()[lastAtomStart:]
			s := out.String()[:lastAtomStart]
			out.Reset()
			out.WriteString(s)
			out.WriteString("(?:")
			out.WriteString(prev)
			out.WriteString(")")
			lastAtomStart = out.Len() - len(prev) - 4
		}
		out.WriteString(quant)
		prevWasQuantifier = true
		atExprStart = false
		afterCaretAnchor = false
	}

	for i := 0; i < n; i++ {
		c := pattern[i]

		if c == '\\' {
			if i+1 >= n {
				return "", nil, fmt.Errorf("Trailing backslash")
			}
			next := pattern[i+1]
			i++
			switch next {
			case '(':
				groupCount++
				groupStack = append(groupStack, groupCount)
				groupStartPos[groupCount] = out.Len()
				appendAtom("(")
				parenDepth++
				atExprStart = true
				afterCaretAnchor = false
			case ')':
				if parenDepth == 0 {
					return "", nil, fmt.Errorf("Unmatched ) or \\)")
				}
				parenDepth--
				if len(groupStack) > 0 {
					g := groupStack[len(groupStack)-1]
					groupStack = groupStack[:len(groupStack)-1]
					start := groupStartPos[g]
					if start+1 <= out.Len() {
						groupPatterns[g] = out.String()[start+1:]
					}
				}
				appendAtom(")")
			case '{':
				if atExprStart {
					appendAtom(`\{`)
					break
				}
				// Collect interval content until \}
				i++
				var interval strings.Builder
				foundEnd := false
				for i < n {
					if pattern[i] == '\\' && i+1 < n && pattern[i+1] == '}' {
						i++ // skip }
						foundEnd = true
						break
					}
					interval.WriteByte(pattern[i])
					i++
				}
				if !foundEnd {
					return "", nil, fmt.Errorf("Unmatched \\{")
				}
				content := interval.String()
				if content == "" {
					return "", nil, fmt.Errorf("Invalid content of \\{\\}")
				}
				if strings.HasPrefix(content, ",") {
					content = "0" + content
				}
				parts := strings.Split(content, ",")
				if len(parts) > 2 {
					return "", nil, fmt.Errorf("Invalid content of \\{\\}")
				}
				for _, p := range parts {
					if p != "" {
						val, err := strconv.Atoi(p)
						if err != nil {
							if errors.Is(err, strconv.ErrRange) {
								return "", nil, fmt.Errorf("Regular expression too big")
							}
							return "", nil, fmt.Errorf("Invalid content of \\{\\}")
						}
						if val > 32767 {
							return "", nil, fmt.Errorf("Regular expression too big")
						}
					}
				}
				appendQuantifier("{" + content + "}")
			case '}':
				appendAtom(`\}`)
			case '[':
				appendAtom(`\[`)
			case '|':
				out.WriteByte('|')
				atExprStart = true
				afterCaretAnchor = false
				prevWasQuantifier = false
			case '+':
				if atExprStart {
					return "", nil, fmt.Errorf("Invalid preceding regular expression")
				}
				appendQuantifier("+")
			case '?':
				if atExprStart {
					return "", nil, fmt.Errorf("Invalid preceding regular expression")
				}
				appendQuantifier("?")
			case '^', '$', '.', '*':
				appendAtom(`\` + string(next))
			case '1', '2', '3', '4', '5', '6', '7', '8', '9':
				gNum := int(next - '0')
				pat, ok := groupPatterns[gNum]
				if !ok || pat == "" {
					return "", nil, fmt.Errorf("Invalid back reference")
				}
				groupCount++
				backrefChecks[groupCount] = gNum
				appendAtom("(" + pat + ")")
			default:
				if isRegexMeta(next) {
					appendAtom(`\` + string(next))
				} else {
					appendAtom(string(next))
				}
			}
			continue
		}

		switch c {
		case '(':
			appendAtom(`\(`)
		case ')':
			appendAtom(`\)`)
		case '{':
			appendAtom(`\{`)
		case '}':
			appendAtom(`\}`)
		case '+':
			appendAtom(`\+`)
		case '?':
			appendAtom(`\?`)
		case '|':
			appendAtom(`\|`)
		case '*':
			if atExprStart || afterCaretAnchor {
				appendAtom(`\*`)
			} else {
				appendQuantifier("*")
			}
		case '^':
			if atExprStart {
				lastAtomStart = out.Len()
				out.WriteByte('^')
				atExprStart = false
				afterCaretAnchor = true
				prevWasQuantifier = false
			} else {
				appendAtom(`\^`)
			}
		case '$':
			if breDollarIsAnchor(pattern, i) {
				lastAtomStart = out.Len()
				out.WriteByte('$')
				atExprStart = false
				afterCaretAnchor = false
				prevWasQuantifier = false
			} else {
				appendAtom(`\$`)
			}
		case '[':
			startPos := out.Len()
			out.WriteByte('[')
			i++
			if i < n && pattern[i] == '^' {
				out.WriteByte('^')
				i++
			}
			if i < n && pattern[i] == ']' {
				out.WriteByte(']')
				i++
			}
			for i < n && pattern[i] != ']' {
				if pattern[i] == '[' && i+1 < n && pattern[i+1] == ':' {
					out.WriteByte(pattern[i])
					i++
				} else {
					out.WriteByte(pattern[i])
					i++
				}
			}
			if i < n {
				out.WriteByte(']')
			}
			lastAtomStart = startPos
			atExprStart = false
			afterCaretAnchor = false
			prevWasQuantifier = false
		default:
			appendAtom(string(c))
		}
	}
	if parenDepth > 0 {
		return "", nil, fmt.Errorf("Unmatched ( or \\(")
	}
	return out.String(), backrefChecks, nil
}

func breDollarIsAnchor(pattern string, i int) bool {
	n := len(pattern)
	j := i + 1
	if j == n {
		return true
	}
	if j+1 < n && pattern[j] == '\\' && (pattern[j+1] == '|' || pattern[j+1] == ')') {
		return true
	}
	return false
}

func isRegexMeta(c byte) bool {
	return strings.IndexByte(`\.+*?()|[]{}^$`, c) >= 0
}

func exprMatch(str, pattern string) (string, error) {
	ere, backrefChecks, err := breToERE(pattern)
	if err != nil {
		return "", err
	}
	anchored := "(?s)^(?:" + ere + ")"
	re, err := regexp.Compile(anchored)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "missing closing )") || strings.Contains(msg, "unexpected )") {
			return "", fmt.Errorf("Unmatched ( or \\(")
		}
		if strings.Contains(msg, "missing argument to repetition") {
			return "", fmt.Errorf("Invalid preceding regular expression")
		}
		if strings.Contains(msg, "invalid repeat count") || strings.Contains(msg, "invalid escape") {
			return "", fmt.Errorf("Invalid content of \\{\\}")
		}
		return "", fmt.Errorf("Invalid regular expression: %v", err)
	}
	m := re.FindStringSubmatch(str)
	if m == nil {
		if re.NumSubexp() > 0 {
			return "", nil
		}
		return "0", nil
	}
	if len(backrefChecks) > 0 {
		valid := true
		for refGroup, targetGroup := range backrefChecks {
			if refGroup >= len(m) || targetGroup >= len(m) || m[refGroup] != m[targetGroup] {
				valid = false
				break
			}
		}
		if !valid {
			if re.NumSubexp() > 0 {
				return "", nil
			}
			return "0", nil
		}
	}
	if len(m) > 1 {
		return m[1], nil
	}
	return strconv.Itoa(utf8.RuneCountInString(m[0])), nil
}
