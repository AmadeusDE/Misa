package ui

import (
	"testing"
)

// TestRootViewRequestsFocusReporting is the regression guard for the host half
// of the visibility contract: a terminal only sends FocusMsg/BlurMsg after the
// program asks for them with DECSET 1004, which the renderer derives from
// View.ReportFocus. Without it the root would never see a focus transition, so
// an in-process image viewer could not pause when the terminal loses focus.
func TestRootViewRequestsFocusReporting(t *testing.T) {
	m := newRoot(t, 80, 24)
	if v := m.View(); !v.ReportFocus {
		t.Fatal("main UI View does not request focus reporting: FocusMsg/BlurMsg never arrive")
	}
}
