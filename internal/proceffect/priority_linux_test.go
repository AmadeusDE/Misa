//go:build linux

package proceffect

import "testing"

func TestParseProcUmask(t *testing.T) {
	for _, tc := range []struct {
		input string
		value int
		ok    bool
	}{
		{"Name:\ttest\nUmask:\t0027\nState:\tR\n", 0027, true},
		{"Umask:\t0000\n", 0, true},
		{"Name:\ttest\n", 0, false},
		{"Umask:\txyz\n", 0, false},
		{"Umask:\t1000\n", 0, false},
	} {
		n, ok := parseProcUmask(tc.input)
		if n != tc.value || ok != tc.ok {
			t.Errorf("%q: %o %v", tc.input, n, ok)
		}
	}
}
