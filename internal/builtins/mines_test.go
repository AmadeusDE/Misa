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
		{[]string{"--width", "2", "--height", "2", "--mines", "9999"}, "Custom", 2, 2, 9999},
		{[]string{"--width", "99", "--height", "99", "--mines", "1"}, "Custom", 99, 99, 1},
		{[]string{"--width", "999", "--height", "999", "--mines", "1"}, "Custom", 999, 999, 1},
		// --percent counts mines against the final board: 200 cells at 15% is 30.
		{[]string{"--width", "20", "--height", "10", "--percent", "15"}, "Custom", 20, 10, 30},
		{[]string{"--width", "20", "--height", "10", "-p", "1"}, "Custom", 20, 10, 2},
		{[]string{"--width", "20", "--height", "10", "--percent", "90"}, "Custom", 20, 10, 180},
		// Argument order must not matter: the percent applies to the resized board.
		{[]string{"-p", "50", "--height", "10", "--width", "20"}, "Custom", 20, 10, 100},
		// A named level's board is the one the percent is measured against.
		{[]string{"-e", "--percent", "15"}, "Custom", 30, 16, 72},
		{[]string{"--percent", "15", "--expert"}, "Custom", 30, 16, 72},
		{[]string{"--percent", "15"}, "Custom", 9, 9, 12},
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
	for _, args := range [][]string{
		{"--width", "1"},
		{"--width", "0"},
		{"--width", "1000"},
		{"--height", "1"},
		{"--height", "0"},
		{"--height", "1000"},
		{"-m", "0"},
		{"-m", "1000000"},
		{"--width", "4294967298"},
		{"-x"},
		{"--help"},
	} {
		if _, err := MinesLevel(args); err == nil {
			t.Errorf("%v: no error", args)
		}
	}
}

func TestMinesLevelRejectsBadPercent(t *testing.T) {
	for _, args := range [][]string{
		{"-p", "0"},
		{"--percent", "91"},
		{"--percent", "-1"},
		{"--percent", "4294967298"},
		{"--percent", "15", "--mines", "10"},
		{"-m", "10", "-p", "15"},
	} {
		if _, err := MinesLevel(args); err == nil {
			t.Errorf("%v: no error", args)
		}
	}
}

func TestMinesLevelPercentErrors(t *testing.T) {
	if _, err := MinesLevel([]string{"--percent", "0"}); err == nil || err.Error() != "--percent must be 1-90" {
		t.Errorf("percent 0: got %v", err)
	}
	if _, err := MinesLevel([]string{"--mines", "10", "--percent", "15"}); err == nil || err.Error() != "--mines and --percent are mutually exclusive" {
		t.Errorf("mines+percent: got %v", err)
	}
}
