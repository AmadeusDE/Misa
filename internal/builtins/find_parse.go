// Parser informed by Toybox find.c, commit 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb.
package builtins

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type findOptions struct {
	follow, followRoot, depth, xdev bool
	maxDepth                        *int
	minDepth                        int
}
type findParser struct {
	ctx                 *Context
	execs               []*findExec
	hasDelete, hasPrune bool
	args                []string
	pos                 int
	options             findOptions
	action              bool
	now                 time.Time
}

func (p *findParser) argument(name string) (string, error) {
	if p.pos >= len(p.args) {
		return "", fmt.Errorf("%s requires an argument", name)
	}
	s := p.args[p.pos]
	p.pos++
	return s, nil
}
func (p *findParser) peek() string {
	if p.pos >= len(p.args) {
		return ""
	}
	return p.args[p.pos]
}
func (p *findParser) parse() (findNode, error) {
	if len(p.args) == 0 {
		return func(*findEntry) bool { return true }, nil
	}
	n, err := p.comma()
	if err != nil {
		return nil, err
	}
	if p.pos != len(p.args) {
		return nil, fmt.Errorf("unexpected %q", p.peek())
	}
	if p.hasDelete && (p.options.follow || p.options.followRoot) {
		return nil, fmt.Errorf("-delete cannot be combined with -L, -H or -follow")
	}
	if p.hasDelete && p.hasPrune {
		return nil, fmt.Errorf("-delete cannot be combined with -prune")
	}
	return n, nil
}
func (p *findParser) comma() (findNode, error) {
	n, err := p.or()
	if err != nil {
		return nil, err
	}
	for p.peek() == "," {
		p.pos++
		var r findNode
		r, err = p.or()
		if err != nil {
			return nil, err
		}
		n = findComma(n, r)
	}
	return n, nil
}
func (p *findParser) or() (findNode, error) {
	n, err := p.and()
	if err != nil {
		return nil, err
	}
	for p.peek() == "-o" || p.peek() == "-or" {
		p.pos++
		var r findNode
		r, err = p.and()
		if err != nil {
			return nil, err
		}
		n = findOr(n, r)
	}
	return n, nil
}
func (p *findParser) and() (findNode, error) {
	n, err := p.unary()
	if err != nil {
		return nil, err
	}
	for p.pos < len(p.args) && p.peek() != ")" && p.peek() != "," && p.peek() != "-o" && p.peek() != "-or" {
		if p.peek() == "-a" || p.peek() == "-and" {
			p.pos++
		}
		var r findNode
		r, err = p.unary()
		if err != nil {
			return nil, err
		}
		n = findAnd(n, r)
	}
	return n, nil
}
func (p *findParser) unary() (findNode, error) {
	name := p.peek()
	if name == "" {
		return nil, fmt.Errorf("missing expression")
	}
	p.pos++
	switch name {
	case "!", "-not":
		n, err := p.unary()
		if err != nil {
			return nil, err
		}
		return findNot(n), nil
	case "(":
		n, err := p.comma()
		if err != nil {
			return nil, err
		}
		if p.peek() != ")" {
			return nil, fmt.Errorf("missing closing parenthesis")
		}
		p.pos++
		return n, nil
	case ")", ",", "-a", "-and", "-o", "-or":
		return nil, fmt.Errorf("unexpected %q", name)
	case "-delete":
		p.hasDelete = true
		p.options.depth = true
		p.action = true
		return findDelete, nil
	case "-exec", "-execdir":
		return p.execAction(name)
	case "-ok", "-okdir":
		return nil, fmt.Errorf("%s not supported", name)
	case "-regextype":
		return nil, fmt.Errorf("-regextype is not supported")
	case "-depth":
		p.options.depth = true
	case "-xdev", "-mount":
		p.options.xdev = true
	case "-follow":
		p.options.follow = true
	case "-maxdepth", "-mindepth":
		s, err := p.argument(name)
		if err != nil {
			return nil, err
		}
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 || strings.HasPrefix(s, "+") {
			return nil, fmt.Errorf("%s: invalid depth %q", name, s)
		}
		if name == "-maxdepth" {
			p.options.maxDepth = &n
		} else {
			p.options.minDepth = n
		}
	case "-print", "-print0":
		p.action = true
		suffix := "\n"
		if name == "-print0" {
			suffix = "\x00"
		}
		return func(e *findEntry) bool {
			_, err := fmt.Fprint(e.ctx.Stdout, e.path, suffix)
			if err != nil {
				return e.fail(err)
			}
			return true
		}, nil
	case "-printf":
		p.action = true
		s, err := p.argument(name)
		if err != nil {
			return nil, err
		}
		return findPrintf(s)
	case "-prune":
		p.hasPrune = true
		return func(e *findEntry) bool {
			if p.options.depth {
				return false
			}
			e.prune = true
			return true
		}, nil
	case "-quit":
		return func(e *findEntry) bool { e.quit = true; return true }, nil
	default:
		return p.predicate(name)
	}
	return func(*findEntry) bool { return true }, nil
}
