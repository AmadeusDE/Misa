package term

func (t *Terminal) handleSGR() {
	for i := 0; i < len(t.params); i++ {
		v := t.params[i]
		subs := t.subParams[i]
		switch {
		case v == 0:
			t.attr = Cell{}
		case v == 1:
			t.attr.Bold = true
		case v == 2:
			t.attr.Dim = true
		case v == 3:
			t.attr.Italic = true
		case v == 4:
			t.attr.Under = len(subs) == 0 || subs[0] != 0
		case v == 7:
			t.attr.Reverse = true
		case v == 8:
			t.attr.Hidden = true
		case v == 9:
			t.attr.Strike = true
		case v == 21:
			t.attr.Under = true
		case v == 22:
			t.attr.Bold = false
			t.attr.Dim = false
		case v == 23:
			t.attr.Italic = false
		case v == 24:
			t.attr.Under = false
		case v == 27:
			t.attr.Reverse = false
		case v == 28:
			t.attr.Hidden = false
		case v == 29:
			t.attr.Strike = false
		case v >= 30 && v <= 37:
			t.attr.HasFG = true
			t.attr.FG = IndexedColor(byte(v - 30))
		case v == 38:
			var ok bool
			if t.attr.FG, ok, i = t.extendedColor(i); ok {
				t.attr.HasFG = true
			}
		case v == 39:
			t.attr.HasFG = false
			t.attr.FG = DefaultColor()
		case v >= 40 && v <= 47:
			t.attr.HasBG = true
			t.attr.BG = IndexedColor(byte(v - 40))
		case v == 48:
			var ok bool
			if t.attr.BG, ok, i = t.extendedColor(i); ok {
				t.attr.HasBG = true
			}
		case v == 49:
			t.attr.HasBG = false
			t.attr.BG = DefaultColor()
		case v == 58:
			_, _, i = t.extendedColor(i)
		case v >= 90 && v <= 97:
			t.attr.HasFG = true
			t.attr.FG = IndexedColor(byte(v - 90 + 8))
		case v >= 100 && v <= 107:
			t.attr.HasBG = true
			t.attr.BG = IndexedColor(byte(v - 100 + 8))
		}
	}
}

// extendedColor decodes the color after SGR 38/48/58 at params[i], in either
// the colon form (38:5:n, 38:2:[cs]:r:g:b) or the legacy semicolon form
// (38;5;n, 38;2;r;g;b). It returns the index of the last parameter consumed.
func (t *Terminal) extendedColor(i int) (Color, bool, int) {
	if subs := t.subParams[i]; len(subs) > 0 {
		vals := make([]int, len(subs))
		for k, s := range subs {
			vals[k] = max(s, 0)
		}
		switch {
		case vals[0] == 5 && len(vals) >= 2:
			return IndexedColor(byte(vals[1])), true, i
		case vals[0] == 2 && len(vals) >= 5:
			return RGBColor(byte(vals[2]), byte(vals[3]), byte(vals[4])), true, i
		case vals[0] == 2 && len(vals) == 4:
			return RGBColor(byte(vals[1]), byte(vals[2]), byte(vals[3])), true, i
		}
		return Color{}, false, i
	}
	ps := t.params
	switch {
	case i+2 < len(ps) && ps[i+1] == 5:
		return IndexedColor(byte(ps[i+2])), true, i + 2
	case i+4 < len(ps) && ps[i+1] == 2:
		return RGBColor(byte(ps[i+2]), byte(ps[i+3]), byte(ps[i+4])), true, i + 4
	}
	return Color{}, false, i
}
