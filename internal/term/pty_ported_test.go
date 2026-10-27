package term

import (
	"os"
	"strings"
	"testing"
	"time"
)

func rowText(t *Terminal, row int) string {
	return cellStr(t, row)
}

func TestCapturedBtopFrameKeepsRows(t *testing.T) {
	data, err := os.ReadFile("testdata/btop-100x30.vt")
	if err != nil {
		t.Fatal(err)
	}
	tm := New(30, 100)
	for len(data) > 0 {
		n := min(8192, len(data))
		tm.ProcessOutput(data[:n])
		data = data[n:]
	}
	tm.Finish()
	if !strings.Contains(rowText(tm, 0), "cpu") {
		t.Errorf("row 0: %q", rowText(tm, 0))
	}
	if r := rowText(tm, 10); !strings.Contains(r, "mem") || !strings.Contains(r, "proc") {
		t.Errorf("row 10: %q", r)
	}
	if !strings.Contains(rowText(tm, 29), "0/") {
		t.Errorf("row 29: %q", rowText(tm, 29))
	}
}

func TestCursorPositionAliasPlacesText(t *testing.T) {
	tm := New(3, 5)
	tm.ProcessOutput([]byte("\x1b[1;1fAAAA\x1b[3;2fZ"))
	if tm.cells[0][0].Ch != 'A' || tm.cells[2][1].Ch != 'Z' {
		t.Errorf("rows: %q %q", rowText(tm, 0), rowText(tm, 2))
	}
}

func TestSynchronizedMarkersSplitAcrossReads(t *testing.T) {
	tm := New(3, 10)
	tm.ProcessOutput([]byte("before\x1b[?202"))
	tm.ProcessOutput([]byte("6h\rAFTER\x1b[?202"))
	if strings.Contains(rowText(tm, 0), "AFTER") {
		t.Fatalf("sync block applied before end marker: %q", rowText(tm, 0))
	}
	tm.ProcessOutput([]byte("6l!"))
	if got := rowText(tm, 0); got != "AFTER!" {
		t.Fatalf("row 0 = %q, want AFTER!", got)
	}
	tm.ProcessOutput([]byte("\rnext"))
	if got := rowText(tm, 0); got != "nextR!" {
		t.Fatalf("row 0 = %q, want nextR!", got)
	}
}

func TestUnterminatedSyncBlockIsFlushedOnFinish(t *testing.T) {
	tm := New(2, 10)
	tm.ProcessOutput([]byte("\x1b[?2026hheld"))
	if rowText(tm, 0) != "" {
		t.Fatalf("held text leaked: %q", rowText(tm, 0))
	}
	tm.Finish()
	if got := rowText(tm, 0); got != "held" {
		t.Fatalf("row 0 = %q, want held", got)
	}
}

func TestOversizedSyncBlockBailsOut(t *testing.T) {
	tm := New(2, 10)
	tm.ProcessOutput([]byte("\x1b[?2026h"))
	tm.ProcessOutput([]byte(strings.Repeat("x", maxSyncPending+1)))
	if rowText(tm, 0) == "" {
		t.Fatal("oversized sync block never flushed")
	}
	tm.ProcessOutput([]byte("\x1b[1;1HZ"))
	if tm.cells[0][0].Ch != 'Z' {
		t.Fatalf("output after bail-out not applied directly: %q", rowText(tm, 0))
	}
}

func TestCursorAliasSpansReadsWithoutChangingOSCText(t *testing.T) {
	tm := New(3, 10)
	tm.ProcessOutput([]byte("\x1b]0;title\x1b[3;2f\x07\x1b[2;"))
	tm.ProcessOutput([]byte("3fX"))
	if tm.cells[1][2].Ch != 'X' {
		t.Errorf("row 1 = %q", rowText(tm, 1))
	}
}

func TestEraseLineReplacesOldCells(t *testing.T) {
	tm := New(2, 5)
	tm.ProcessOutput([]byte("abcde\x1b[1;1H\x1b[2Kxy"))
	if got := rowText(tm, 0); got != "xy" {
		t.Errorf("row 0 = %q, want xy", got)
	}
	tm.ProcessOutput([]byte("\x1b[2;1Hhello\rworld"))
	if tm.Rows != 2 || tm.Cols != 5 {
		t.Errorf("size = %dx%d", tm.Rows, tm.Cols)
	}
}

func TestWideCellAndResizeRemainAddressable(t *testing.T) {
	tm := New(2, 6)
	tm.ProcessOutput([]byte("界a"))
	if tm.cells[0][0].Ch != '界' {
		t.Errorf("cell 0 = %q", tm.cells[0][0].Ch)
	}
	tm.Resize(1, 2)
	if tm.Rows != 1 || tm.Cols != 2 {
		t.Errorf("size = %dx%d", tm.Rows, tm.Cols)
	}
}

func waitExited(t *testing.T, tm *Terminal) {
	t.Helper()
	select {
	case <-tm.Exited():
	case <-time.After(5 * time.Second):
		t.Fatal("child never exited")
	}
}

func TestExitIsReportedOnlyAfterFinalOutputProcessed(t *testing.T) {
	tm := New(5, 40)
	if err := tm.StartArgv("", testBin(t, "sh"), "-c", "printf FINAL_OUTPUT; exit 7"); err != nil {
		t.Fatal(err)
	}
	defer tm.Close()
	tm.StartReader()
	waitExited(t, tm)
	if !strings.Contains(strings.Join(tm.PlainLines(), "\n"), "FINAL_OUTPUT") {
		t.Errorf("final output lost: %q", tm.PlainLines())
	}
	if tm.ExitCode() != 7 {
		t.Errorf("exit code = %d, want 7", tm.ExitCode())
	}
}

func TestStartArgvIsArgvNotShell(t *testing.T) {
	tm := New(3, 40)
	if err := tm.StartArgv("", testBin(t, "echo"), "hello  $HOME;world"); err != nil {
		t.Fatal(err)
	}
	defer tm.Close()
	tm.StartReader()
	waitExited(t, tm)
	if got := tm.PlainLines()[0]; got != "hello  $HOME;world" {
		t.Errorf("output = %q", got)
	}
}

func TestReaderNotifiesAfterOutput(t *testing.T) {
	tm := New(3, 40)
	if err := tm.StartArgv("", testBin(t, "sh"), "-c", "printf hi"); err != nil {
		t.Fatal(err)
	}
	defer tm.Close()
	tm.StartReader()
	select {
	case <-tm.Dirty():
	case <-time.After(5 * time.Second):
		t.Fatal("no notification")
	}
}

func BenchmarkRenderFullBtopFrame(b *testing.B) {
	data, err := os.ReadFile("testdata/btop-100x30.vt")
	if err != nil {
		b.Fatal(err)
	}
	tm := New(30, 100)
	tm.ProcessOutput(data)
	tm.Finish()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tm.RenderFull(true, nil)
	}
}
