// Regression scenarios informed by Toybox tests/patch.test at
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb (0BSD), written natively for Misa.
// Every case asserts exit status, streams and the resulting filesystem bytes.
package builtins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type ptWantFile struct {
	content string
	absent  bool
}

type ptCase struct {
	name     string
	args     []string
	patch    string            // stdin
	files    map[string]string // created before the run, relative to the temp dir
	want     map[string]ptWantFile
	wantCode int
	wantOut  string // substring expected on stdout ("" = not checked)
	wantErr  string // substring expected on stderr ("" = only checked for code 0)
}

func ptRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func ptAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("%s should not exist (stat error %v)", path, err)
	}
}

// ptCombined is the union of both streams: "Hunk N FAILED" is diagnostic and the
// spec does not pin it to one stream.
func ptCombined(r runResult) string { return r.stdout + r.stderr }

func ptAssertCombinedContains(t *testing.T, r runResult, substr string) {
	t.Helper()
	if !strings.Contains(ptCombined(r), substr) {
		t.Errorf("output %q does not contain %q", ptCombined(r), substr)
	}
}

func runPatchTable(t *testing.T, cases []ptCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tc.files {
				writeFile(t, dir, name, content)
			}
			r := runBuiltinStdinIn(t, dir, "patch", tc.patch, tc.args...)
			assertExitCode(t, r, tc.wantCode)
			if tc.wantOut != "" {
				assertStdoutContains(t, r, tc.wantOut)
			}
			if tc.wantErr != "" {
				assertStderrContains(t, r, tc.wantErr)
			} else if tc.wantCode == 0 && r.stderr != "" {
				t.Errorf("unexpected stderr %q", r.stderr)
			}
			for name, want := range tc.want {
				path := filepath.Join(dir, name)
				if want.absent {
					ptAbsent(t, path)
					continue
				}
				if got := ptRead(t, path); got != want.content {
					t.Errorf("%s content = %q, want %q", name, got, want.content)
				}
			}
		})
	}
}

func TestPatchRegistered(t *testing.T) {
	if _, ok := Lookup("patch"); !ok {
		t.Fatal("patch not registered")
	}
}

func TestPatchHelpVersion(t *testing.T) {
	r := runBuiltin(t, "patch", "--help")
	assertExitCode(t, r, 0)
	if r.stdout == "" {
		t.Error("--help printed nothing")
	}
	r = runBuiltin(t, "patch", "--version")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "Misa")
}

// TestPatchToyboxCases translates every case from Toybox tests/patch.test.
func TestPatchToyboxCases(t *testing.T) {
	const (
		dryRunCreate = "--- /dev/null\n+++ bork\n@@ -0,0 +1,1 @@\n+one\n"
		createFile   = "--- /dev/null\n+++ bork\n@@ -0,0 +1,3 @@\n+one\n+two\n+three\n"
		insertMiddle = "--- bork\n+++ bork\n@@ -1,3 +1,4 @@\n one\n+four\n two\n three\n"
		appendEnd    = "--- bork\n+++ bork\n@@ -2,3 +2,5 @@\n four\n two\n three\n+five\n+six\n"
		insertStart  = "--- bork\n+++ bork\n@@ -1,3 +1,4 @@\n+seven\n one\n four\n two\n"
		deleteEnd    = "--- bork\n+++ bork\n@@ -4,4 +4,3 @@\n two\n three\n five\n-six\n"
		deleteStart  = "--- bork\n+++ bork\n@@ -1,5 +1,3 @@\n-seven\n-one\n four\n two\n three\n"
		timestamps   = "--- bork\t2019-12-20 16:54:35.735630973 -0600\n+++ bork\t2019-12-20 16:57:03.083625706 -0600\n@@ -1,4 +1,5 @@\n four\n two\n three\n+eight\n five\n"
		quotedName   = "--- /dev/null\n+++ \"fruit ba\\164\"\n@@ -0,0 +1 @@\n+hello\n"
		deleteQuoted = "--- \"fruit bat\"\n+++ /dev/null\n@@ -1 +0,0 @@\n-hello\n"
		fuzzPatch    = "--- a/x/input\n+++ b/x/input\n@@ -15,7 +15,6 @@\n  */\n package org.yaml.snakeyaml.representer;\n\n-import java.beans.IntrospectionException;\n import java.util.ArrayList;\n import java.util.Arrays;\n import java.util.Iterator;\n"

		bork123       = "one\ntwo\nthree\n"
		bork4         = "one\nfour\ntwo\nthree\n"
		bork45        = "one\nfour\ntwo\nthree\nfive\nsix\n"
		bork7         = "seven\none\nfour\ntwo\nthree\nfive\nsix\n"
		bork7no6      = "seven\none\nfour\ntwo\nthree\nfive\n"
		borkFour      = "four\ntwo\nthree\nfive\n"
		borkFourEight = "four\ntwo\nthree\neight\nfive\n"
		fuzzBefore    = "blah blah\n */\npackage org.yaml.snakeyaml.representer;\n\nimport java.beans.IntrospectionException;\nimport java.util.ArrayList;\nimport java.util.Arrays;\nimport java.util.Collections;\nimport java.util.Iterator;\n"
		fuzzAfter     = "blah blah\n */\npackage org.yaml.snakeyaml.representer;\n\nimport java.util.ArrayList;\nimport java.util.Arrays;\nimport java.util.Collections;\nimport java.util.Iterator;\n"
	)
	runPatchTable(t, []ptCase{
		{name: "dry run doesn't create file", args: []string{"--dry-run"}, patch: dryRunCreate, wantCode: 0,
			want: map[string]ptWantFile{"bork": {absent: true}}},
		{name: "create file", patch: createFile, wantCode: 0, wantOut: "creating bork",
			want: map[string]ptWantFile{"bork": {content: bork123}}},
		{name: "insert in middle", patch: insertMiddle, files: map[string]string{"bork": bork123}, wantCode: 0,
			wantOut: "patching bork", want: map[string]ptWantFile{"bork": {content: bork4}}},
		{name: "append at end", patch: appendEnd, files: map[string]string{"bork": bork4}, wantCode: 0,
			want: map[string]ptWantFile{"bork": {content: bork45}}},
		{name: "insert at start", patch: insertStart, files: map[string]string{"bork": bork45}, wantCode: 0,
			want: map[string]ptWantFile{"bork": {content: bork7}}},
		{name: "delete at end", patch: deleteEnd, files: map[string]string{"bork": bork7}, wantCode: 0,
			want: map[string]ptWantFile{"bork": {content: bork7no6}}},
		{name: "delete at start", patch: deleteStart, files: map[string]string{"bork": bork7no6}, wantCode: 0,
			want: map[string]ptWantFile{"bork": {content: borkFour}}},
		{name: "filter timestamps", patch: timestamps, files: map[string]string{"bork": borkFour}, wantCode: 0,
			want: map[string]ptWantFile{"bork": {content: borkFourEight}}},
		{name: "quoted name", patch: quotedName, wantCode: 0,
			want: map[string]ptWantFile{"fruit bat": {content: "hello\n"}}},
		{name: "bad quote", patch: "--- \"filename", wantCode: 1, wantErr: "patch: bad \"filename"},
		{name: "dry run doesn't delete file", args: []string{"--dry-run"}, patch: deleteQuoted, wantCode: 0,
			files: map[string]string{"fruit bat": "hello\n"},
			want:  map[string]ptWantFile{"fruit bat": {content: "hello\n"}}},
		{name: "delete file", patch: deleteQuoted, files: map[string]string{"fruit bat": "hello\n"},
			wantCode: 0, wantOut: "removing fruit bat", want: map[string]ptWantFile{"fruit bat": {absent: true}}},
		{name: "fuzz", patch: fuzzPatch, files: map[string]string{"input": fuzzBefore}, wantCode: 0,
			want: map[string]ptWantFile{"input": {content: fuzzAfter}}},
	})
}

// TestPatchMisaGeneratedDiff generates a unified diff with the diff builtin and
// applies it with patch, then reverses it.
func TestPatchMisaGeneratedDiff(t *testing.T) {
	dir := t.TempDir()
	const oldContent = "one\ntwo\nthree\nfour\n"
	const newContent = "one\n2\nthree\nfour\nfive\n"
	writeFile(t, dir, "old", oldContent)
	writeFile(t, dir, "new", newContent)

	r := runBuiltinIn(t, dir, "diff", "-u", "old", "new")
	if r.code != 1 || r.stdout == "" {
		t.Fatalf("diff -u: %+v", r)
	}
	patchText := r.stdout
	if !strings.HasPrefix(patchText, "--- old") {
		t.Fatalf("unexpected diff header: %q", patchText)
	}

	// The FILE operand overrides the header names, so the diff applies to old.
	r = runBuiltinStdinIn(t, dir, "patch", patchText, "old")
	assertExitCode(t, r, 0)
	if got := ptRead(t, filepath.Join(dir, "old")); got != newContent {
		t.Errorf("after forward patch: %q, want %q", got, newContent)
	}
	if got := ptRead(t, filepath.Join(dir, "new")); got != newContent {
		t.Errorf("unrelated file changed: %q", got)
	}

	r = runBuiltinStdinIn(t, dir, "patch", patchText, "-R", "old")
	assertExitCode(t, r, 0)
	if got := ptRead(t, filepath.Join(dir, "old")); got != oldContent {
		t.Errorf("after -R: %q, want %q", got, oldContent)
	}
}

func TestPatchStripLevels(t *testing.T) {
	const text = "--- a/x/file\n+++ b/x/file\n@@ -1 +1 @@\n-old\n+new\n"
	t.Run("default strips to basename", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "file", "old\n")
		r := runBuiltinStdinIn(t, dir, "patch", text)
		assertExitCode(t, r, 0)
		if got := ptRead(t, filepath.Join(dir, "file")); got != "new\n" {
			t.Errorf("file = %q, want %q", got, "new\n")
		}
		ptAbsent(t, filepath.Join(dir, "x"))
		ptAbsent(t, filepath.Join(dir, "b"))
	})
	t.Run("p0 keeps the full name", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "b/x/file", "old\n")
		r := runBuiltinStdinIn(t, dir, "patch", text, "-p", "0")
		assertExitCode(t, r, 0)
		if got := ptRead(t, filepath.Join(dir, "b/x/file")); got != "new\n" {
			t.Errorf("b/x/file = %q, want %q", got, "new\n")
		}
		ptAbsent(t, filepath.Join(dir, "file"))
	})
	t.Run("p1 strips one component", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "x/file", "old\n")
		r := runBuiltinStdinIn(t, dir, "patch", text, "-p1")
		assertExitCode(t, r, 0)
		if got := ptRead(t, filepath.Join(dir, "x/file")); got != "new\n" {
			t.Errorf("x/file = %q, want %q", got, "new\n")
		}
		ptAbsent(t, filepath.Join(dir, "b"))
	})
}

func TestPatchUnsafePaths(t *testing.T) {
	const body = "@@ -0,0 +1 @@\n+evil\n"
	newBase := func(t *testing.T) (base, dir string) {
		t.Helper()
		base = t.TempDir()
		dir = filepath.Join(base, "work")
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatal(err)
		}
		return base, dir
	}
	t.Run("absolute path", func(t *testing.T) {
		base, dir := newBase(t)
		escape := filepath.Join(base, "escaped")
		r := runBuiltinStdinIn(t, dir, "patch", "--- /dev/null\n+++ "+escape+"\n"+body, "-p0")
		assertExitCode(t, r, 1)
		assertStderrContains(t, r, "refusing unsafe path")
		ptAbsent(t, escape)
	})
	t.Run("dot dot escape", func(t *testing.T) {
		base, dir := newBase(t)
		// -p1 leaves "x/../evil", which still carries a ".." component.
		r := runBuiltinStdinIn(t, dir, "patch", "--- a/x/evil\n+++ b/x/../evil\n"+body, "-p1")
		assertExitCode(t, r, 1)
		assertStderrContains(t, r, "refusing unsafe path")
		ptAbsent(t, filepath.Join(base, "evil"))
		ptAbsent(t, filepath.Join(dir, "evil"))
	})
}

func TestPatchInputFile(t *testing.T) {
	const text = "--- f\n+++ f\n@@ -1 +1 @@\n-old\n+new\n"
	t.Run("-i patchfile", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "f", "old\n")
		writeFile(t, dir, "p.diff", text)
		r := runBuiltinIn(t, dir, "patch", "-i", "p.diff")
		assertExitCode(t, r, 0)
		if got := ptRead(t, filepath.Join(dir, "f")); got != "new\n" {
			t.Errorf("f = %q, want %q", got, "new\n")
		}
	})
	t.Run("patchfile operand", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "target", "old\n")
		writeFile(t, dir, "p.diff", "--- other\n+++ other\n@@ -1 +1 @@\n-old\n+new\n")
		r := runBuiltinIn(t, dir, "patch", "target", "p.diff")
		assertExitCode(t, r, 0)
		if got := ptRead(t, filepath.Join(dir, "target")); got != "new\n" {
			t.Errorf("target = %q, want %q", got, "new\n")
		}
		ptAbsent(t, filepath.Join(dir, "other"))
	})
	t.Run("missing patch file", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "f", "old\n")
		r := runBuiltinIn(t, dir, "patch", "-i", "nosuch.diff")
		assertExitCode(t, r, 1)
		assertStderrContains(t, r, "No such file or directory")
		if got := ptRead(t, filepath.Join(dir, "f")); got != "old\n" {
			t.Errorf("f = %q, want unchanged", got)
		}
	})
}

func TestPatchDirectoryOption(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "bork", "keep\n")
	writeFile(t, dir, "sub/bork", "old\n")
	const text = "--- bork\n+++ bork\n@@ -1 +1 @@\n-old\n+new\n"
	r := runBuiltinStdinIn(t, dir, "patch", text, "-d", "sub")
	assertExitCode(t, r, 0)
	if got := ptRead(t, filepath.Join(dir, "sub/bork")); got != "new\n" {
		t.Errorf("sub/bork = %q, want %q", got, "new\n")
	}
	if got := ptRead(t, filepath.Join(dir, "bork")); got != "keep\n" {
		t.Errorf("bork outside -d = %q, want untouched", got)
	}
}

func TestPatchIgnoreWhitespace(t *testing.T) {
	const text = "--- f\n+++ f\n@@ -1,3 +1,3 @@\n apple\n-   banana\n+banana2\n cherry\n"
	const before = "apple\n\tbanana\ncherry\n"
	const after = "apple\nbanana2\ncherry\n"
	t.Run("strict rejects", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "f", before)
		r := runBuiltinStdinIn(t, dir, "patch", text)
		assertExitCode(t, r, 1)
		ptAssertCombinedContains(t, r, "Hunk 1 FAILED")
		if got := ptRead(t, filepath.Join(dir, "f")); got != before {
			t.Errorf("f = %q, want untouched", got)
		}
	})
	t.Run("-l applies", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "f", before)
		r := runBuiltinStdinIn(t, dir, "patch", text, "-l")
		assertExitCode(t, r, 0)
		if got := ptRead(t, filepath.Join(dir, "f")); got != after {
			t.Errorf("f = %q, want %q", got, after)
		}
	})
}

func TestPatchSilent(t *testing.T) {
	const text = "--- f\n+++ f\n@@ -1 +1 @@\n-old\n+new\n"
	dir := t.TempDir()
	writeFile(t, dir, "f", "old\n")
	r := runBuiltinStdinIn(t, dir, "patch", text, "-s")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "")
	if got := ptRead(t, filepath.Join(dir, "f")); got != "new\n" {
		t.Errorf("f = %q, want %q", got, "new\n")
	}

	dir = t.TempDir()
	writeFile(t, dir, "f", "old\n")
	r = runBuiltinStdinIn(t, dir, "patch", text, "-s", "-v")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "-s and -v are mutually exclusive")
	if got := ptRead(t, filepath.Join(dir, "f")); got != "old\n" {
		t.Errorf("f = %q, want untouched", got)
	}
}

func TestPatchRemoveEmptyFiles(t *testing.T) {
	const text = "--- f\n+++ f\n@@ -1 +0,0 @@\n-line\n"
	t.Run("without -E keeps an empty file", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "f", "line\n")
		r := runBuiltinStdinIn(t, dir, "patch", text)
		assertExitCode(t, r, 0)
		if got := ptRead(t, filepath.Join(dir, "f")); got != "" {
			t.Errorf("f = %q, want empty", got)
		}
	})
	t.Run("-E unlinks the emptied file", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "f", "line\n")
		r := runBuiltinStdinIn(t, dir, "patch", text, "-E")
		assertExitCode(t, r, 0)
		ptAbsent(t, filepath.Join(dir, "f"))
	})
}

func TestPatchFuzzFactor(t *testing.T) {
	const text = "--- f\n+++ f\n@@ -1,4 +1,4 @@\n w1\n w2\n b\n-c\n+C\n"
	const before = "junk1\njunk2\nb\nc\n"
	const after = "junk1\njunk2\nb\nC\n"
	t.Run("-F 0 disables fuzz", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "f", before)
		r := runBuiltinStdinIn(t, dir, "patch", text, "-F", "0")
		assertExitCode(t, r, 1)
		ptAssertCombinedContains(t, r, "Hunk 1 FAILED")
		if got := ptRead(t, filepath.Join(dir, "f")); got != before {
			t.Errorf("f = %q, want untouched", got)
		}
	})
	t.Run("-F 2 accepts the offset", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "f", before)
		r := runBuiltinStdinIn(t, dir, "patch", text, "-F", "2")
		assertExitCode(t, r, 0)
		if got := ptRead(t, filepath.Join(dir, "f")); got != after {
			t.Errorf("f = %q, want %q", got, after)
		}
	})
}

func TestPatchDryRun(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f", "old\n")
	writeFile(t, dir, "h", "gone\n")
	const text = "--- f\n+++ f\n@@ -1 +1 @@\n-old\n+new\n" +
		"--- /dev/null\n+++ g\n@@ -0,0 +1 @@\n+new file\n" +
		"--- h\n+++ /dev/null\n@@ -1 +0,0 @@\n-gone\n"
	r := runBuiltinStdinIn(t, dir, "patch", text, "--dry-run")
	assertExitCode(t, r, 0)
	if got := ptRead(t, filepath.Join(dir, "f")); got != "old\n" {
		t.Errorf("f = %q, want byte-identical", got)
	}
	if got := ptRead(t, filepath.Join(dir, "h")); got != "gone\n" {
		t.Errorf("h = %q, want byte-identical", got)
	}
	ptAbsent(t, filepath.Join(dir, "g"))
	ptAbsent(t, filepath.Join(dir, "f.rej"))
}

func TestPatchMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f", "old\n")
	const text = "--- f\n+++ f\n@@ -1 +1 @@\n-old\n+new\n" +
		"--- /dev/null\n+++ g\n@@ -0,0 +1 @@\n+created\n"
	r := runBuiltinStdinIn(t, dir, "patch", text)
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "patching f")
	assertStdoutContains(t, r, "creating g")
	if got := ptRead(t, filepath.Join(dir, "f")); got != "new\n" {
		t.Errorf("f = %q, want %q", got, "new\n")
	}
	if got := ptRead(t, filepath.Join(dir, "g")); got != "created\n" {
		t.Errorf("g = %q, want %q", got, "created\n")
	}
}

func TestPatchFileOperandOverride(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "target", "old\n")
	const text = "--- a/newfile\n+++ b/newfile\n@@ -1 +1 @@\n-old\n+new\n"
	r := runBuiltinStdinIn(t, dir, "patch", text, "target")
	assertExitCode(t, r, 0)
	if got := ptRead(t, filepath.Join(dir, "target")); got != "new\n" {
		t.Errorf("target = %q, want %q", got, "new\n")
	}
	ptAbsent(t, filepath.Join(dir, "newfile"))
}

func TestPatchFailures(t *testing.T) {
	const goodBody = "@@ -1 +1 @@\n-old\n+new\n"
	runPatchTable(t, []ptCase{
		{name: "missing target file", patch: "--- f\n+++ f\n" + goodBody, wantCode: 1,
			wantErr: "No such file or directory", want: map[string]ptWantFile{"f": {absent: true}}},
		{name: "malformed hunk header", patch: "--- f\n+++ f\n@@ -q,1 +1,1 @@\n-old\n+new\n",
			files: map[string]string{"f": "old\n"}, wantCode: 1,
			wantErr: "malformed hunk header", want: map[string]ptWantFile{"f": {content: "old\n"}}},
		{name: "truncated hunk", patch: "--- f\n+++ f\n@@ -1,3 +1,3 @@\n a\n",
			files: map[string]string{"f": "a\nb\nc\n"}, wantCode: 1,
			wantErr: "malformed hunk", want: map[string]ptWantFile{"f": {content: "a\nb\nc\n"}}},
		{name: "creation over an existing file", patch: "--- /dev/null\n+++ f\n@@ -0,0 +1 @@\n+new\n",
			files: map[string]string{"f": "keep\n"}, wantCode: 1,
			wantErr: "file already exists", want: map[string]ptWantFile{"f": {content: "keep\n"}}},
		{name: "three operands", args: []string{"a", "b", "c"}, wantCode: 1, wantErr: "extra operand"},
	})

	t.Run("hunk does not apply writes rejects", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "f", "a\nb\nc\n")
		r := runBuiltinStdinIn(t, dir, "patch", "--- f\n+++ f\n@@ -1,3 +1,3 @@\n a\n-zzz\n+yyy\n c\n")
		assertExitCode(t, r, 1)
		ptAssertCombinedContains(t, r, "Hunk 1 FAILED")
		if got := ptRead(t, filepath.Join(dir, "f")); got != "a\nb\nc\n" {
			t.Errorf("f = %q, want untouched", got)
		}
		rej := ptRead(t, filepath.Join(dir, "f.rej"))
		for _, want := range []string{"--- f", "+++ f", "@@ -1,3 +1,3 @@", "-zzz", "+yyy"} {
			if !strings.Contains(rej, want) {
				t.Errorf("f.rej %q does not contain %q", rej, want)
			}
		}
	})

	t.Run("stale rej removed on success", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "f", "old\n")
		writeFile(t, dir, "f.rej", "stale\n")
		r := runBuiltinStdinIn(t, dir, "patch", "--- f\n+++ f\n"+goodBody)
		assertExitCode(t, r, 0)
		if got := ptRead(t, filepath.Join(dir, "f")); got != "new\n" {
			t.Errorf("f = %q, want %q", got, "new\n")
		}
		ptAbsent(t, filepath.Join(dir, "f.rej"))
	})
}

func TestPatchUnsupportedOptions(t *testing.T) {
	const text = "--- f\n+++ f\n@@ -1 +1 @@\n-old\n+new\n"
	for _, opt := range []string{"-b", "-N", "-o"} {
		t.Run(opt, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "f", "old\n")
			r := runBuiltinStdinIn(t, dir, "patch", text, opt)
			assertExitCode(t, r, 1)
			assertStderrContains(t, r, "Try 'patch --help' for more information.")
			if got := ptRead(t, filepath.Join(dir, "f")); got != "old\n" {
				t.Errorf("f = %q, want unchanged", got)
			}
		})
	}
}

func TestPatchUnsupportedFormats(t *testing.T) {
	cases := []struct {
		name  string
		patch string
		err   string
	}{
		{"context diff", "*** f\t2020-01-01\n--- f\t2020-01-01\n********\n", "not supported"},
		{"binary diff", "Binary files a and b differ\n", "binary diffs are not supported"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "f", "old\n")
			r := runBuiltinStdinIn(t, dir, "patch", tc.patch)
			assertExitCode(t, r, 1)
			assertStderrContains(t, r, tc.err)
			if got := ptRead(t, filepath.Join(dir, "f")); got != "old\n" {
				t.Errorf("f = %q, want unchanged", got)
			}
		})
	}
}

// TestPatchNoNewlineMarker covers "\ No newline at end of file": a hunk whose
// old side ends unterminated must not match a terminated last line.
func TestPatchNoNewlineMarker(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a", "one\ntwo")
	writeFile(t, dir, "b", "one\ntwo\nthree")
	r := runBuiltinIn(t, dir, "diff", "-u", "a", "b")
	if r.code != 1 || !strings.Contains(r.stdout, "\\ No newline at end of file") {
		t.Fatalf("diff -u: %+v", r)
	}
	patchText := r.stdout

	r = runBuiltinStdinIn(t, dir, "patch", patchText, "a")
	assertExitCode(t, r, 0)
	if got := ptRead(t, filepath.Join(dir, "a")); got != "one\ntwo\nthree" {
		t.Errorf("forward: a = %q, want %q", got, "one\ntwo\nthree")
	}

	r = runBuiltinStdinIn(t, dir, "patch", patchText, "-R", "a")
	assertExitCode(t, r, 0)
	if got := ptRead(t, filepath.Join(dir, "a")); got != "one\ntwo" {
		t.Errorf("reverse: a = %q, want %q", got, "one\ntwo")
	}

	// The post-image must not be silently re-patched into a duplicated tail.
	before := "one\ntwo\nthree"
	writeFile(t, dir, "twice", before)
	r = runBuiltinStdinIn(t, dir, "patch", patchText, "twice")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "Possibly reversed hunk")
	if got := ptRead(t, filepath.Join(dir, "twice")); got != before {
		t.Errorf("already applied: twice = %q, want untouched", got)
	}
}

// TestPatchCRLF checks the DOS newline notice and that CRLF patches apply.
func TestPatchCRLF(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f", "old\n")
	const text = "--- f\r\n+++ f\r\n@@ -1 +1 @@\r\n-old\r\n+new\r\n"
	r := runBuiltinStdinIn(t, dir, "patch", text)
	assertExitCode(t, r, 0)
	assertStderrContains(t, r, "Removing DOS newlines")
	if got := ptRead(t, filepath.Join(dir, "f")); got != "new\n" {
		t.Errorf("f = %q, want %q", got, "new\n")
	}

	dir = t.TempDir()
	writeFile(t, dir, "f", "old\n")
	r = runBuiltinStdinIn(t, dir, "patch", text, "-s")
	assertExitCode(t, r, 0)
	if r.stderr != "" {
		t.Errorf("stderr = %q, want silence with -s", r.stderr)
	}
}

// TestPatchSymlinkTarget writes through a symlink instead of replacing it.
func TestPatchSymlinkTarget(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "real", "old\n")
	if err := os.Symlink("real", filepath.Join(dir, "link")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	r := runBuiltinStdinIn(t, dir, "patch", "--- link\n+++ link\n@@ -1 +1 @@\n-old\n+new\n")
	assertExitCode(t, r, 0)
	if fi, err := os.Lstat(filepath.Join(dir, "link")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("link is no longer a symlink (err %v)", err)
	}
	if got := ptRead(t, filepath.Join(dir, "real")); got != "new\n" {
		t.Errorf("real = %q, want %q", got, "new\n")
	}
}

// TestPatchModePreserved keeps the target's permissions across a rewrite.
func TestPatchModePreserved(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f", "old\n")
	if err := os.Chmod(filepath.Join(dir, "f"), 0741); err != nil {
		t.Fatal(err)
	}
	r := runBuiltinStdinIn(t, dir, "patch", "--- f\n+++ f\n@@ -1 +1 @@\n-old\n+new\n")
	assertExitCode(t, r, 0)
	fi, err := os.Stat(filepath.Join(dir, "f"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0741 {
		t.Errorf("mode = %o, want 741", fi.Mode().Perm())
	}
}

// TestPatchRejectCoversLaterHunks: every hunk that is not applied goes to the
// reject file, and the target is left byte-identical.
func TestPatchRejectCoversLaterHunks(t *testing.T) {
	dir := t.TempDir()
	const before = "a\nb\nc\nd\ne\nf\n"
	writeFile(t, dir, "f", before)
	const text = "--- f\n+++ f\n@@ -1,2 +1,2 @@\n a\n-nope\n+X\n" +
		"@@ -5,2 +5,2 @@\n e\n-f\n+Y\n"
	r := runBuiltinStdinIn(t, dir, "patch", text)
	assertExitCode(t, r, 1)
	ptAssertCombinedContains(t, r, "Hunk 1 FAILED")
	if got := ptRead(t, filepath.Join(dir, "f")); got != before {
		t.Errorf("f = %q, want untouched", got)
	}
	rej := ptRead(t, filepath.Join(dir, "f.rej"))
	if !strings.Contains(rej, "@@ -1,2 +1,2 @@") || !strings.Contains(rej, "@@ -5,2 +5,2 @@") {
		t.Errorf("f.rej = %q, want both hunks", rej)
	}
	if !strings.Contains(rej, "+Y") {
		t.Errorf("f.rej = %q, want the unapplied hunk body", rej)
	}
}

// TestPatchRejectReapplies: a reject file is a valid unified patch on its own.
func TestPatchRejectReapplies(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f", "a\nb\nc\n")
	r := runBuiltinStdinIn(t, dir, "patch", "--- f\n+++ f\n@@ -1,3 +1,3 @@\n a\n-zzz\n+yyy\n c\n")
	assertExitCode(t, r, 1)
	rej := ptRead(t, filepath.Join(dir, "f.rej"))
	writeFile(t, dir, "f", "a\nzzz\nc\n")
	r = runBuiltinStdinIn(t, dir, "patch", rej)
	assertExitCode(t, r, 0)
	if got := ptRead(t, filepath.Join(dir, "f")); got != "a\nyyy\nc\n" {
		t.Errorf("f = %q, want %q", got, "a\nyyy\nc\n")
	}
}

// TestPatchCreateInSubdir creates missing parent directories for a new file.
func TestPatchCreateInSubdir(t *testing.T) {
	dir := t.TempDir()
	const text = "--- /dev/null\n+++ b/sub/dir/new\n@@ -0,0 +1,2 @@\n+hi\n+there\n"
	r := runBuiltinStdinIn(t, dir, "patch", text, "-p1")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "creating sub/dir/new")
	if got := ptRead(t, filepath.Join(dir, "sub/dir/new")); got != "hi\nthere\n" {
		t.Errorf("sub/dir/new = %q", got)
	}
}

// TestPatchAbsoluteFileOperand: an explicit FILE operand is used literally,
// including an absolute path the caller typed.
func TestPatchAbsoluteFileOperand(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	writeFile(t, dir, "target", "old\n")
	r := runBuiltinStdinIn(t, dir, "patch", "--- other\n+++ other\n@@ -1 +1 @@\n-old\n+new\n", target)
	assertExitCode(t, r, 0)
	if got := ptRead(t, target); got != "new\n" {
		t.Errorf("target = %q, want %q", got, "new\n")
	}
}

// TestPatchBadDirectory: -d must name an existing directory and must not change
// the process working directory.
func TestPatchBadDirectory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f", "old\n")
	r := runBuiltinStdinIn(t, dir, "patch", "--- f\n+++ f\n@@ -1 +1 @@\n-old\n+new\n", "-d", "missing")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "No such file or directory")
	if got := ptRead(t, filepath.Join(dir, "f")); got != "old\n" {
		t.Errorf("f = %q, want untouched", got)
	}
}

// TestPatchDeleteLeavesRemainder: hunks that do not empty a file the header
// deletes must leave the target untouched.
func TestPatchDeleteLeavesRemainder(t *testing.T) {
	dir := t.TempDir()
	const before = "a\nb\nc\n"
	writeFile(t, dir, "f", before)
	const text = "--- f\n+++ /dev/null\n@@ -1 +0,0 @@\n-a\n"
	r := runBuiltinStdinIn(t, dir, "patch", text)
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "left it non-empty")
	if got := ptRead(t, filepath.Join(dir, "f")); got != before {
		t.Errorf("f = %q, want untouched", got)
	}
}
