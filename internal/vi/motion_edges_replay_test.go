package vi

import "testing"

func TestF04ReviewedEdges(t *testing.T) {
	for _, f := range []replayFixture{
		{name: "failed find does not hide previous undo", input: "abcx\n", keys: "xdfzu", want: replayState{lines: []string{"abcx"}, reg: []string{"a"}}},
		{name: "yank find does not hide previous undo", input: "abcx\n", keys: "xyfxu", want: replayState{lines: []string{"abcx"}, reg: []string{"bcx"}}},
		{name: "failed search does not hide previous undo", input: "abcx\n", keys: "xd/zzz<CR>u", want: replayState{lines: []string{"abcx"}, reg: []string{"a"}}},
		{name: "counted till repeat includes adjacent match", input: "axaxbxcx\n", keys: "tx2;", want: replayState{lines: []string{"axaxbxcx"}, col: 2}},
		{name: "counted backward till repeat", input: "xaxbxcxz\n", keys: "$Tx2;", want: replayState{lines: []string{"xaxbxcxz"}, col: 5}},
		{name: "counted till operator repeat", input: "axaxbxcx\n", keys: "txd2;", want: replayState{lines: []string{"xbxcx"}, reg: []string{"axa"}, modified: true}},
		{name: "cancel search preserves direction", input: "x a x b x\n", keys: "/x<CR>?z<Esc>n", want: replayState{lines: []string{"x a x b x"}, col: 8}},
		{name: "cancel search preserves pattern", input: "x a x b x\n", keys: "/x<CR>/z<Esc>n", want: replayState{lines: []string{"x a x b x"}, col: 8}},
		{name: "repeat bracket delete stays bracket motion", input: "(a)(b)z\ntail\n", keys: "d%.", want: replayState{lines: []string{"z", "tail"}, reg: []string{"(b)"}, modified: true}},
		{name: "change find dot", input: "abcx defx z\n", keys: "ctxQ<Esc>w.", want: replayState{lines: []string{"Qx Qx z"}, col: 3, reg: []string{"def"}, modified: true}},
	} {
		t.Run(f.name, func(t *testing.T) { runReplayFixture(t, f) })
	}
}
