package aihelp

import (
	"bytes"
	"strings"
	"testing"
)

func TestNextBlockHoldsCodeFencesTogether(t *testing.T) {
	text := "para one\n\n```sh\na\n\nb\n```\n\nlast"
	var blocks []string
	for {
		block, rest, ok := nextBlock(text)
		if !ok {
			break
		}
		blocks = append(blocks, block)
		text = rest
	}
	if len(blocks) != 2 || blocks[0] != "para one" || blocks[1] != "```sh\na\n\nb\n```" || text != "last" {
		t.Errorf("blocks %q, rest %q", blocks, text)
	}
}

func TestPlainPrinterPassesTextThrough(t *testing.T) {
	var out bytes.Buffer
	p := NewPrinter(&out, false, 80)
	p.Write("**hi** ")
	p.Write("there")
	p.Close()
	if out.String() != "**hi** there" {
		t.Errorf("out = %q", out.String())
	}
}

func TestStyledPrinterRendersBlocksAsTheyComplete(t *testing.T) {
	var out bytes.Buffer
	p := NewPrinter(&out, true, 80)
	p.render = strings.ToUpper
	p.Write("first pa")
	p.Write("ragraph\n")
	if out.Len() != 0 {
		t.Fatalf("wrote %q before the block ended", out.String())
	}
	p.Write("\nsecond\n\n\n\nthird")
	if out.String() != "FIRST PARAGRAPH\n\nSECOND" {
		t.Fatalf("out = %q", out.String())
	}
	p.Close()
	if out.String() != "FIRST PARAGRAPH\n\nSECOND\n\nTHIRD\n" {
		t.Errorf("out = %q", out.String())
	}
}

func TestMarkdownRendersWithGlamour(t *testing.T) {
	got := Markdown("# Title\n\nUse `ls`.", 60)
	if !strings.Contains(got, "Title") || strings.Contains(got, "# Title") || strings.HasPrefix(got, "\n") {
		t.Errorf("rendered %q", got)
	}
}

func TestIsTTYIsFalseForBuffers(t *testing.T) {
	if IsTTY(&bytes.Buffer{}) {
		t.Error("buffer is not a terminal")
	}
	t.Setenv("COLUMNS", "120")
	if Width(&bytes.Buffer{}) != 120 {
		t.Error("COLUMNS ignored")
	}
}

func TestTrimPaddingKeepsContentAndStyling(t *testing.T) {
	in := "  \x1b[1mhi\x1b[m\x1b[38;5;252m \x1b[m\x1b[38;5;252m \x1b[m\n\x1b[m  \n  plain   "
	want := "  \x1b[1mhi\x1b[m\n\n  plain"
	if got := trimPadding(in); got != want {
		t.Errorf("trimPadding = %q, want %q", got, want)
	}
}
