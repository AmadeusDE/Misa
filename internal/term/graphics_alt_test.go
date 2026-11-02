package term

import "testing"

func TestGraphicsAlternateEntryClearsOnlyAlternateResources(t *testing.T) {
	tm := gfxTerm(t, 5, 10)
	pixels := gfxPixels(1, 1, 4)
	tm.ProcessOutput(gfxAPC("a=T,f=32,i=1,s=1,v=1,c=1,r=1,C=1;" + pixels))
	tm.ProcessOutput([]byte("\x1b[?1049h"))
	tm.ProcessOutput(gfxAPC("a=T,f=32,i=2,s=1,v=1,c=1,r=1,C=1;" + pixels))
	tm.ProcessOutput([]byte("\x1b[?1049l\x1b[?1049h"))
	if tm.gfx.image(screenAlt, 2) != nil {
		t.Fatal("alternate image survived clearing the alternate cell grid")
	}
	if _, placements := gfxSnapshot(t, tm); len(placements) != 0 {
		t.Fatalf("alternate placements survived re-entry: %+v", placements)
	}
	tm.ProcessOutput([]byte("\x1b[?1049l"))
	if tm.gfx.image(screenMain, 1) == nil {
		t.Fatal("clearing the alternate screen dropped a main-screen image")
	}
	if _, placements := gfxSnapshot(t, tm); len(placements) != 1 || placements[0].ImageID != 1 {
		t.Fatalf("main placement not restored: %+v", placements)
	}
}
