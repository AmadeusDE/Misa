//go:build unix && !android

package imgview

import "testing"

func TestStandaloneFocusReportsPauseAndResume(t *testing.T) {
	fake := &fakePlayback{}
	v, _ := animatedViewer(t, 3, 0, fake)
	v.Update(ctrlKey('a'))
	if !fake.visible || !fake.playing {
		t.Fatal("playback did not start")
	}
	d := standaloneDecoder{}
	for _, m := range d.feed([]byte("\x1b[O")) {
		v.Update(m)
	}
	if v.focused || fake.visible {
		t.Fatal("blur did not pause playback")
	}
	for _, m := range d.feed([]byte("\x1b[I")) {
		v.Update(m)
	}
	if !v.focused || !fake.visible || !fake.playing {
		t.Fatal("focus did not resume playback")
	}
}
