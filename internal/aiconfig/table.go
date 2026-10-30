package aiconfig

// cursorList is a selection in a scrolling list whose rows start at screen line top.
type cursorList struct {
	n, cur, off int
	top, rows   int
}

func (c *cursorList) setN(n int) {
	c.n = n
	c.fit()
}

func (c *cursorList) place(top, rows int) {
	c.top, c.rows = top, max(1, rows)
	c.fit()
}

func (c *cursorList) fit() {
	c.cur = max(0, min(c.cur, c.n-1))
	if c.rows <= 0 {
		return
	}
	if c.cur < c.off {
		c.off = c.cur
	}
	if c.cur >= c.off+c.rows {
		c.off = c.cur - c.rows + 1
	}
	c.off = max(0, min(c.off, c.n-c.rows))
}

func (c *cursorList) move(d int) {
	c.cur += d
	c.fit()
}

func (c *cursorList) key(k string) bool {
	switch k {
	case "up", "ctrl+p":
		c.move(-1)
	case "down", "ctrl+n":
		c.move(1)
	case "pgup":
		c.move(-c.rows)
	case "pgdown":
		c.move(c.rows)
	case "home":
		c.move(-c.n)
	case "end":
		c.move(c.n)
	default:
		return false
	}
	return true
}

func (c *cursorList) window() (lo, hi int) {
	return c.off, min(c.n, c.off+c.rows)
}

// wheel scrolls the view by three rows per notch, taking the selection along when it leaves the window.
func (c *cursorList) wheel(d int) {
	c.off = max(0, min(c.off+3*d, c.n-c.rows))
	c.cur = max(c.off, min(c.cur, c.off+c.rows-1))
}

// click selects the row under line y. It reports whether a row was hit and whether it was already selected.
func (c *cursorList) click(y int) (hit, again bool) {
	i := c.off + y - c.top
	if y < c.top || i >= c.n || i >= c.off+c.rows {
		return false, false
	}
	again = i == c.cur
	c.cur = i
	return true, again
}
