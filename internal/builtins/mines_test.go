package builtins

import "testing"

func TestMinesLevel(t *testing.T) {
	tests := []struct {
		args        []string
		name        string
		w, h, mines int
	}{
		{nil, "Beginner", 9, 9, 10},
		{[]string{"-i"}, "Intermediate", 16, 16, 40},
		{[]string{"--expert"}, "Expert", 30, 16, 99},
		{[]string{"--width", "20", "--height=10", "-m", "30"}, "Custom", 20, 10, 30},
		{[]string{"-e", "--mines", "120"}, "Custom", 30, 16, 120},
	}
	for _, tt := range tests {
		l, err := MinesLevel(tt.args)
		if err != nil {
			t.Fatalf("%v: %v", tt.args, err)
		}
		if l.Name != tt.name || l.W != tt.w || l.H != tt.h || l.Mines != tt.mines {
			t.Errorf("%v: got %+v", tt.args, l)
		}
	}
}

func TestMinesLevelRejectsBadSize(t *testing.T) {
	for _, args := range [][]string{{"--width", "1"}, {"--height", "500"}, {"-m", "0"}, {"-x"}, {"--help"}} {
		if _, err := MinesLevel(args); err == nil {
			t.Errorf("%v: no error", args)
		}
	}
}
