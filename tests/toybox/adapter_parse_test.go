package toybox

// Unit tests for the adapter's vendored-file handling: multi-line expected
// blocks, the NOSPACE=1 directive, `$(echo -e …)` fixture content and the
// `echo … | xxd -r -p > name` setup form. These keep the new syntax support
// honest without changing any upstream suite's accounting.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpectedSpansLines(t *testing.T) {
	for _, tc := range []struct {
		line string
		want bool
	}{
		{`testing "-l" "readelf -lW $elf-short" "`, true},
		{`testing "-h" "readelf -hW $full" "ELF Header:`, true},
		{`NOSPACE=1 toyonly testing "-d" "readelf -dW $full" "`, true},
		{`testing "create_hardlink" "ln file hlink && [ file -ef hlink ] &&`, false},
		{`testcmd "750 dir 640 file" "750 dir 640 file &&`, false},
		{`testing "ascii" "file input" "input: ASCII text\n" "" ""`, false},
		{`testcmd "trailing slash" "'abc\'" 'abc\' "" ""`, false},
	} {
		line := tc.line
		line = strings.TrimPrefix(line, "NOSPACE=1 ")
		line = strings.TrimPrefix(line, "toyonly ")
		if got := expectedSpansLines(line); got != tc.want {
			t.Errorf("expectedSpansLines(%q) = %v, want %v", tc.line, got, tc.want)
		}
	}
}

func TestEchoExprContent(t *testing.T) {
	content, ok := echoExprContent("$(echo -e '!<arch>\\nhello')")
	if !ok || content != "!<arch>\nhello\n" {
		t.Fatalf("echoExprContent = %q, %v", content, ok)
	}
	content, ok = echoExprContent("$(echo -e 'world')\n")
	if !ok || content != "world\n\n" {
		t.Fatalf("echoExprContent trailing newline = %q, %v", content, ok)
	}
	if _, ok := echoExprContent("plain text"); ok {
		t.Fatal("plain text must not be treated as an echo expression")
	}
}

func TestSameOutput(t *testing.T) {
	if !sameOutput("a b\n", "a b\n", false) {
		t.Error("identical output must match")
	}
	if sameOutput("a  b\n", "a b\n", false) {
		t.Error("raw comparison must be strict")
	}
	if !sameOutput("a  b\n  c\n", "a b\nc\n", true) {
		t.Error("NOSPACE comparison must ignore whitespace")
	}
	if sameOutput("a b\nc\n", "a b\nd\n", true) {
		t.Error("NOSPACE comparison must still compare non-whitespace")
	}
}

func TestParseDirectives(t *testing.T) {
	path := filepath.Join(t.TempDir(), "readelf.test")
	src := `elf=$FILES/elf/note
NOSPACE=1 testing "-h" "readelf -hW $elf-full" "
ELF Header:
  Class: ELF64
" "" ""
testing "-l" "readelf -lW $elf-short" "one line\n" "" ""
`
	if err := os.WriteFile(path, []byte(src), 0600); err != nil {
		t.Fatal(err)
	}
	suite, err := parseTestFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(suite.unparsed) != 0 {
		t.Fatalf("unexpected unparsed cases: %+v", suite.unparsed)
	}
	if len(suite.cases) != 2 {
		t.Fatalf("got %d cases, want 2: %+v", len(suite.cases), suite.cases)
	}
	if got, want := suite.cases[0].expected, "\nELF Header:\n  Class: ELF64\n"; got != want {
		t.Errorf("multiline expected = %q, want %q", got, want)
	}
	if !suite.cases[0].nospace {
		t.Error("NOSPACE=1 case should be marked nospace")
	}
	if suite.cases[0].args != "-hW $elf-full" {
		t.Errorf("args = %q", suite.cases[0].args)
	}
	if suite.cases[1].nospace {
		t.Error("plain case must not be marked nospace")
	}
}

func TestParseEchoPipelineFixture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.test")
	src := "echo \"cafe\" | xxd -r -p > universal\n" +
		`testing "mach-o" "file universal" "universal: data\n" "" ""` + "\n"
	if err := os.WriteFile(path, []byte(src), 0600); err != nil {
		t.Fatal(err)
	}
	suite, err := parseTestFile(path)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ev := range suite.setupLog {
		if ev.name == "exec" && strings.HasPrefix(ev.content, "echo \"cafe\" | xxd") {
			found = true
		}
	}
	if !found {
		t.Fatalf("echo pipeline was not recorded as setup: %+v", suite.setupLog)
	}
}
