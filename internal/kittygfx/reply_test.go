package kittygfx

import "testing"

func TestReply(t *testing.T) {
	tests := []struct {
		name    string
		cmd     Command
		code    string
		message string
		want    string
	}{
		{"ok", Command{ID: 7}, "OK", "", "\x1b_Gi=7;OK\x1b\\"},
		{"number only", Command{Number: 5}, "OK", "", "\x1b_GI=5;OK\x1b\\"},
		{"id and number", Command{ID: 7, Number: 5}, "OK", "", "\x1b_Gi=7,I=5;OK\x1b\\"},
		{"id and placement", Command{ID: 7, PlacementID: 3}, "OK", "", "\x1b_Gi=7,p=3;OK\x1b\\"},
		{"placement only", Command{PlacementID: 3}, "OK", "", "\x1b_Gp=3;OK\x1b\\"},
		{"all ids", Command{ID: 7, Number: 5, PlacementID: 3}, "OK", "", "\x1b_Gi=7,I=5,p=3;OK\x1b\\"},
		{"no ids", Command{}, "OK", "", "\x1b_G;OK\x1b\\"},
		{"error", Command{ID: 7}, "EINVAL", "bad key", "\x1b_Gi=7;EINVAL:bad key\x1b\\"},
		{"quiet 1 ok", Command{ID: 7, Quiet: 1}, "OK", "", ""},
		{"quiet 1 error", Command{ID: 7, Quiet: 1}, "EINVAL", "bad key", "\x1b_Gi=7;EINVAL:bad key\x1b\\"},
		{"quiet 2 ok", Command{ID: 7, Quiet: 2}, "OK", "", ""},
		{"quiet 2 error", Command{ID: 7, Quiet: 2}, "EINVAL", "bad key", ""},
		{"empty code", Command{ID: 1}, "", "", "\x1b_Gi=1;OK\x1b\\"},
		{"sanitized", Command{ID: 1}, "EINVAL", "a\x1b;b\x07c", "\x1b_Gi=1;EINVAL:abc\x1b\\"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(Reply(tt.cmd, tt.code, tt.message)); got != tt.want {
				t.Fatalf("Reply = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReplyBoundsMessage(t *testing.T) {
	message := ""
	for i := 0; i < 1000; i++ {
		message += "x"
	}
	if got := len(Reply(Command{ID: 1}, "EINVAL", message)); got > 256 {
		t.Fatalf("reply length %d is unbounded", got)
	}
}
