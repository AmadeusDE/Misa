package webui

import (
	"strings"
	"testing"
)

func TestQRTextIsSquareAndLight(t *testing.T) {
	out, err := QRText("otpauth://totp/ughbot:ughbot?secret=JBSWY3DPEHPK3PXP&issuer=ughbot")
	if err != nil {
		t.Fatal(err)
	}
	rows := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	width := len([]rune(rows[0]))
	for _, r := range rows {
		if len([]rune(r)) != width {
			t.Fatalf("ragged row %q", r)
		}
	}
	if abs := width - len(rows)*2; abs < -2 || abs > 2 {
		t.Fatalf("not square: %d columns, %d rows", width, len(rows))
	}
	if !strings.HasPrefix(rows[0], strings.Repeat("█", width)) {
		t.Fatalf("missing quiet zone: %q", rows[0])
	}
}
