// Toybox tar suite adaptation; upstream toybox is 0BSD licensed, pinned at
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb. No host archive tools are run.
package toybox

import (
	"archive/tar"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/builtins"
)

// Enumerate even cases the generic parser cannot execute. The final if false
// block is upstream's unfinished TODO, not executable tests.
var tarCaseLine = regexp.MustCompile(`^\s*(?:(?:toyonly|NOSPACE=1)\s+)?(?:testing|testcmd)\s+(['"])([^'"]+)['"]`)

func TestToyboxTarSuite(t *testing.T) {
	f, err := os.Open("tar.test")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var names []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "if false" {
			break
		}
		if m := tarCaseLine.FindStringSubmatch(line); m != nil {
			names = append(names, m[2])
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(names) != 81 {
		t.Fatalf("tar case inventory changed: got %d; review adapter and known failures", len(names))
	}
	t.Logf("Toybox tar: %d upstream cases inventoried", len(names))
	t.Run("tar", func(t *testing.T) {
		for _, name := range names {
			t.Run(name, func(t *testing.T) {
				key := "tar/" + strings.ReplaceAll(name, " ", "_")
				if reason, ok := knownFailures()[key]; ok {
					if os.Getenv("MISA_KNOWN_FAILURES") == "" {
						t.Skip("known failure: " + reason)
					}
					if name != "./file bug" {
						t.Skip("adapter cannot execute this case even with MISA_KNOWN_FAILURES: " + reason)
					}
				}
				switch name {
				case "longname":
					tarLongname(t)
				case "links":
					tarLinks(t, 1)
				case "links2":
					tarLinks(t, 13)
				case "./file bug":
					tarDotFile(t)
				default:
					t.Fatalf("unclassified upstream tar case %q", name)
				}
			})
		}
	})
}

func tarRun(t *testing.T, dir string, input []byte, args ...string) []byte {
	t.Helper()
	cmd, ok := builtins.Lookup("tar")
	if !ok {
		t.Fatal("tar builtin not registered")
	}
	var out, stderr bytes.Buffer
	status := cmd.Run(&builtins.Context{Dir: dir, CmdName: "tar", Args: args, Stdin: bytes.NewReader(input), Stdout: &out, Stderr: &stderr})
	if status != 0 {
		t.Fatalf("tar %q: status %d, stderr %s", args, status, &stderr)
	}
	return out.Bytes()
}

// Upstream's external long_path.tar fixture is not shipped; generate an
// equivalent ustar prefix/name path with the standard library instead.
func tarLongname(t *testing.T) {
	name := "long_file_name" + strings.Repeat("_", 86) + "TRAILING"
	var data bytes.Buffer
	w := tar.NewWriter(&data)
	if err := w.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: 0}); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	got := tarRun(t, t.TempDir(), data.Bytes(), "t")
	if string(got) != name+"\n" {
		t.Fatalf("list got %q, want %q", got, name+"\n")
	}
}

// The upstream links/links2 cases check creation exit status. Inspect the
// resulting entries too, without requiring a particular directory order.
func tarLinks(t *testing.T, n int) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "links"), 0755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		suffix := ""
		if n > 1 {
			suffix = fmt.Sprint(i)
		}
		orig := filepath.Join(dir, "links", "orig"+suffix)
		if err := os.WriteFile(orig, nil, 0644); err != nil {
			t.Fatal(err)
		}
		count := 1
		if n == 1 {
			count = 2
		}
		for j := 1; j <= count; j++ {
			link := filepath.Join(dir, "links", fmt.Sprintf("link%d", j))
			if n > 1 {
				link = filepath.Join(dir, "links", "link"+suffix)
			}
			if err := os.Link(orig, link); err != nil {
				t.Fatal(err)
			}
		}
	}
	tarRun(t, dir, nil, "cf", "test.tar", "links")
	archive, err := os.Open(filepath.Join(dir, "test.tar"))
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	r := tar.NewReader(archive)
	count := 0
	for {
		h, err := r.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeLink {
			count++
		}
	}
	want := n
	if n == 1 {
		want = 2
	}
	if count != want {
		t.Fatalf("got %d hard links, want %d", count, want)
	}
}

func tarDotFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	data := tarRun(t, dir, nil, "c", "./file")
	got := tarRun(t, dir, data, "t", "./file")
	if string(got) != "./file\n" {
		t.Fatalf("got %q, want ./file newline", got)
	}
}
