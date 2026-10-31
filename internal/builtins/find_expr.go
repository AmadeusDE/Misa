// Expression model informed by Toybox find.c at commit 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb.
package builtins

import (
	"io/fs"
	"time"
)

type findNode func(*findEntry) bool

type findEntry struct {
	ctx              *Context
	path, full, root string
	info             fs.FileInfo
	depth            int
	now              time.Time
	follow           bool
	prune, quit      bool
	err              error
	validate         func() error
}

func (e *findEntry) fail(err error) bool {
	if e.err == nil {
		e.err = err
	}
	return false
}

func findAnd(a, b findNode) findNode {
	return func(e *findEntry) bool { v := a(e); return v && !e.quit && b(e) }
}
func findOr(a, b findNode) findNode {
	return func(e *findEntry) bool { v := a(e); return v || (!e.quit && b(e)) }
}
func findComma(a, b findNode) findNode {
	return func(e *findEntry) bool {
		a(e)
		if e.quit {
			return true
		}
		return b(e)
	}
}
func findNot(a findNode) findNode { return func(e *findEntry) bool { return !a(e) } }
