package vi

import "testing"

// Original public-event fixtures, Vim 9.1 nocompatible defaults.
func TestF04FindReplay(t *testing.T) {
	for _, f := range []replayFixture{
		{name: "normal macro find delete replay", input: "abc)def)z\n", keys: "qadf)q@a", want: replayState{lines: []string{"z"}, reg: []string{"def)"}, modified: true}},
		{name: "counted delete repeat find", input: "axbxcdxexz\n", keys: "fxd2;", want: replayState{lines: []string{"aexz"}, col: 1, reg: []string{"xbxcdx"}, modified: true}},
		{name: "counted delete reverse find", input: "axbxcdxexz\n", keys: "fx;;d2,", want: replayState{lines: []string{"axexz"}, col: 1, reg: []string{"xbxcd"}, modified: true}},
		{name: "forward count", input: "axbxcx\n", keys: "2fx", want: replayState{lines: []string{"axbxcx"}, col: 3}},
		{name: "backward count", input: "xaxbxc\n", keys: "$2Fx", want: replayState{lines: []string{"xaxbxc"}, col: 2}},
		{name: "counted till distinct hits", input: "axxxz\n", keys: "2tx", want: replayState{lines: []string{"axxxz"}, col: 1}},
		{name: "backward till count", input: "xaxbxc\n", keys: "$2Tx", want: replayState{lines: []string{"xaxbxc"}, col: 3}},
		{name: "repeat forward till does not stick", input: "axxbx\n", keys: "tx;", want: replayState{lines: []string{"axxbx"}, col: 1}},
		{name: "repeat backward find direction", input: "xaxbxc\n", keys: "$Fx;", want: replayState{lines: []string{"xaxbxc"}, col: 2}},
		{name: "reverse backward find direction", input: "xaxbxc\n", keys: "$2Fx,", want: replayState{lines: []string{"xaxbxc"}, col: 4}},
		{name: "reverse then repeat backward find", input: "xaxbxcxdxez\n", keys: "$3Fx,;", want: replayState{lines: []string{"xaxbxcxdxez"}, col: 4}},
		{name: "reverse then repeat backward till", input: "xaxbxcxdxez\n", keys: "$3Tx,;", want: replayState{lines: []string{"xaxbxcxdxez"}, col: 5}},
		{name: "multiply operator find count", input: "axbxcxdxexfxz\n", keys: "2d3fx", want: replayState{lines: []string{"z"}, reg: []string{"axbxcxdxexfx"}, modified: true}},
		{name: "repeat backward till direction", input: "xaxbxc\n", keys: "$Tx;", want: replayState{lines: []string{"xaxbxc"}, col: 3}},
		{name: "missing find", input: "abc\n", keys: "fz", want: replayState{lines: []string{"abc"}}},
		{name: "missing delete no undo", input: "abc\n", keys: "dfzu", want: replayState{lines: []string{"abc"}}},
		{name: "delete find inclusive", input: "abc)z\n", keys: "df)", want: replayState{lines: []string{"z"}, reg: []string{"abc)"}, modified: true}},
		{name: "change till", input: "abc\"z\n", keys: "ct\"Q<Esc>", want: replayState{lines: []string{"Q\"z"}, reg: []string{"abc"}, modified: true}},
		{name: "delete backward find", input: "abxcde\n", keys: "$dFx", want: replayState{lines: []string{"abe"}, col: 2, reg: []string{"xcd"}, modified: true}},
		{name: "delete backward till", input: "abxcde\n", keys: "$dTx", want: replayState{lines: []string{"abxe"}, col: 3, reg: []string{"cd"}, modified: true}},
		{name: "delete undo redo", input: "abc)z\n", keys: "df)u<C-r>", want: replayState{lines: []string{"z"}, reg: []string{"abc)"}, modified: true}},
		{name: "named register delete and put", input: "abc)z\n", keys: "\"adf)\"ap", want: replayState{lines: []string{"zabc)"}, col: 4, reg: []string{"abc)"}, modified: true}},
		{name: "cancel register isolation", input: "abc\n", keys: "\"ayy\"adf<Esc>x\"ap", want: replayState{lines: []string{"bc", "abc"}, row: 1, reg: []string{"abc"}, linewise: true, modified: true}},
		{name: "cancel count", input: "abcdef\n", keys: "2df<Esc>x", want: replayState{lines: []string{"bcdef"}, reg: []string{"a"}, modified: true}},
	} {
		t.Run(f.name, func(t *testing.T) { runReplayFixture(t, f) })
	}
}
