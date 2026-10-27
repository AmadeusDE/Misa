package term

import (
	"strings"
	"testing"
)

func TestSGR4Subparams(t *testing.T) {
	// SGR 4 without subparam enables underline
	tm := New(1, 20)
	tm.ProcessOutput([]byte("\x1b[4mhi"))
	if !tm.attr.Under {
		t.Error("SGR 4 should enable underline")
	}

	// SGR 4:0 disables underline
	tm = New(1, 20)
	tm.ProcessOutput([]byte("\x1b[4mhi\x1b[4:0m"))
	if tm.attr.Under {
		t.Error("SGR 4:0 should disable underline")
	}

	// SGR 4:1 enables underline
	tm = New(1, 20)
	tm.ProcessOutput([]byte("\x1b[4:1mhi"))
	if !tm.attr.Under {
		t.Error("SGR 4:1 should enable underline")
	}

	// SGR 4:3 enables underline (curly)
	tm = New(1, 20)
	tm.ProcessOutput([]byte("\x1b[4:3mhi"))
	if !tm.attr.Under {
		t.Error("SGR 4:3 should enable underline")
	}

	// SGR 4:0 should NOT corrupt colors (old bug: 4:0 → SGR 40 = black bg)
	tm = New(1, 20)
	tm.ProcessOutput([]byte("\x1b[4:0mhi"))
	if tm.attr.HasBG {
		t.Error("SGR 4:0 should not set background color")
	}

	// SGR 4:1 should NOT corrupt colors (old bug: 4:1 → SGR 41 = red bg)
	tm = New(1, 20)
	tm.ProcessOutput([]byte("\x1b[4:1mhi"))
	if tm.attr.HasBG {
		t.Error("SGR 4:1 should not set background color")
	}

	// Render check: underline then 4:0 should remove underline styling
	tm = New(1, 40)
	tm.ProcessOutput([]byte("\x1b[4munder\x1b[4:0m plain\x1b[0m"))
	rendered := tm.Render()
	if !strings.Contains(rendered, "plain") {
		t.Errorf("expected plain text, got %q", rendered)
	}
}
