package lscore

import "testing"

func TestParseLSColorsExtensionFold(t *testing.T) {
	cases := []struct {
		name    string
		lsc     string
		queries map[string]string // ext (with dot) -> expected code, "" means no match
	}{
		{
			name: "case-insensitive when values agree",
			lsc:  "*.jpg=01;35:*.Z=01;31",
			queries: map[string]string{
				".jpg": "01;35",
				".JPG": "01;35", // fold fallback
				".z":   "01;31", // fold fallback
				".Z":   "01;31",
			},
		},
		{
			name: "case-sensitive disabled when values disagree",
			lsc:  "*.jpg=01;35:*.JPG=01;35;46",
			queries: map[string]string{
				".jpg": "01;35",
				".JPG": "01;35;46",
				".JpG": "", // ambiguous, no fold
			},
		},
		{
			name: "later same-case entries win, fold enabled when final values agree",
			lsc:  "*.jpg=01;35:*.jpg=01;35;46:*.JPG=01;35;46",
			queries: map[string]string{
				".jpg": "01;35;46",
				".JPG": "01;35;46",
				".JpG": "01;35;46",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, fold := parseLSColors(c.lsc)
			savedM, savedFold := extColors, extFold
			extColors, extFold = m, fold
			defer func() { extColors, extFold = savedM, savedFold }()

			for ext, want := range c.queries {
				got := lookupExtColor(ext)
				if got != want {
					t.Errorf("lookupExtColor(%q) = %q, want %q", ext, got, want)
				}
			}
		})
	}
}

func TestFileColorMultiHardLinkPrecedence(t *testing.T) {
	savedM, savedFold := extColors, extFold
	extColors, extFold = parseLSColors("mh=44;37:*.png=01;35:ex=01;32")
	defer func() { extColors, extFold = savedM, savedFold }()

	notHardLinked := &FileInfo{Name: "file.png", Nlink: 1}
	if got := fileColor(notHardLinked); got != "01;35" {
		t.Errorf("non-hardlinked png = %q, want extension color", got)
	}

	hardLinked := &FileInfo{Name: "file.png", Nlink: 2}
	if got := fileColor(hardLinked); got != "44;37" {
		t.Errorf("hardlinked png = %q, want mh color to take precedence", got)
	}

	executableHardLinked := &FileInfo{Name: "file.png", Nlink: 2, Mode: 0111}
	if got := fileColor(executableHardLinked); got != "01;32" {
		t.Errorf("executable hardlinked png = %q, want ex color to take precedence", got)
	}
}

func TestFileColorMultiHardLinkDisabledFallsBackToExtension(t *testing.T) {
	savedM, savedFold := extColors, extFold
	extColors, extFold = parseLSColors("mh=00:*.png=01;35")
	defer func() { extColors, extFold = savedM, savedFold }()

	hardLinked := &FileInfo{Name: "file.png", Nlink: 2}
	if got := fileColor(hardLinked); got != "01;35" {
		t.Errorf("mh=00 should fall back to extension color, got %q", got)
	}
}

func TestColorNameLeadingResetOnlyOnce(t *testing.T) {
	savedM, savedFold := extColors, extFold
	extColors, extFold = parseLSColors("*.jpg=01;35")
	defer func() { extColors, extFold = savedM, savedFold }()

	resetColorState()
	opts := FormatOpts{}

	first := ColorName(&FileInfo{Name: "a.jpg"}, opts, "a.jpg")
	if want := "\033[0m\033[01;35ma.jpg\033[0m"; first != want {
		t.Errorf("first colored name = %q, want %q", first, want)
	}

	second := ColorName(&FileInfo{Name: "b.jpg"}, opts, "b.jpg")
	if want := "\033[01;35mb.jpg\033[0m"; second != want {
		t.Errorf("second colored name = %q, want %q (no leading reset)", second, want)
	}
}

func TestColorNameNoColorPassthrough(t *testing.T) {
	resetColorState()
	got := ColorName(&FileInfo{Name: "a.jpg", Mode: 0111}, FormatOpts{NoColor: true}, "a.jpg")
	if got != "a.jpg" {
		t.Errorf("NoColor should pass through unchanged, got %q", got)
	}
}
