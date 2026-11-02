package android

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ui"
)

// The image viewer's animation keys are ctrl+a (toggle playback) and
// ctrl+n / ctrl+p (step one frame), see internal/imgview/viewer_keys.go.
// The extra-keys bar offers them as dedicated one-tap buttons.
func TestExtraKeysPlaybackChordsMatchViewer(t *testing.T) {
	// android-device.sh copies ExtraKeys.java to the same relative path.
	src, err := os.ReadFile("../../../android/app/src/main/java/app/misa/ExtraKeys.java")
	if err != nil {
		t.Fatal(err)
	}
	rows := regexp.MustCompile(
		`\{"([^"]+)",\s*KeyEvent\.(KEYCODE_[A-Z0-9_]+),\s*KeyEvent\.(META_[A-Z0-9_]+)\}`,
	).FindAllStringSubmatch(string(src), -1)
	if len(rows) != 3 {
		t.Fatalf("playback table has %d rows, want 3 (ctrl+a, ctrl+p, ctrl+n)", len(rows))
	}
	codes := map[string]int{
		"KEYCODE_A": letter('a'),
		"KEYCODE_N": letter('n'),
		"KEYCODE_P": letter('p'),
	}
	metas := map[string]int{"META_CTRL_ON": MetaCtrlOn}
	want := map[string]bool{"\x01": true, "\x0e": true, "\x10": true}
	labels := map[string]bool{}
	for _, row := range rows {
		label, code, meta := row[1], row[2], row[3]
		if label == "" || labels[label] {
			t.Fatalf("playback label %q is empty or repeated", label)
		}
		labels[label] = true
		c, ok := codes[code]
		if !ok {
			t.Fatalf("playback button %q uses %s, not a viewer animation key", label, code)
		}
		m, ok := metas[meta]
		if !ok {
			t.Fatalf("playback button %q uses %s: dedicated chords must be ctrl only", label, meta)
		}
		ev, ok := TranslateKey(c, m, true)
		if !ok {
			t.Fatalf("playback button %q does not translate: %s|%s", label, code, meta)
		}
		got := string(ui.KeyBytes(ev.Key, false))
		if !want[got] {
			t.Fatalf("playback button %q sends %q, want ctrl+a, ctrl+n or ctrl+p", label, got)
		}
		delete(want, got)
	}
	if len(want) != 0 {
		t.Fatalf("playback table misses %d animation keys", len(want))
	}
}

func TestExtraKeysPlaybackIgnoresLatch(t *testing.T) {
	src, err := os.ReadFile("../../../android/app/src/main/java/app/misa/ExtraKeys.java")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	body := javaMethodBody(t, text, `private void chord\(int code, int meta\)`)
	if strings.Contains(body, "latchedMeta") {
		t.Fatal("a dedicated chord must not pick up the latched Ctrl/Alt")
	}
	if !strings.Contains(body, "listener.key(code, meta)") || !strings.Contains(body, "clearLatch()") {
		t.Fatalf("dedicated chord sends %q, want listener.key(code, meta) and clearLatch()", body)
	}
	if !strings.Contains(text, "for (Object[] item : PLAYBACK)") ||
		!strings.Contains(text, "chord((Integer) item[1], (Integer) item[2])") {
		t.Fatal("playback table does not build one-tap buttons")
	}
}

func javaMethodBody(t *testing.T, src, signature string) string {
	t.Helper()
	m := regexp.MustCompile(`(?s)` + signature + `\s*\{(.*?)\n    \}`).FindStringSubmatch(src)
	if m == nil {
		t.Fatalf("no %s in ExtraKeys.java", signature)
	}
	return m[1]
}
