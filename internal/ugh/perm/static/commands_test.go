package static

import (
	"reflect"
	"testing"
)

func TestSimpleCommands(t *testing.T) {
	ok := map[string][][]string{
		"git status":                   {{"git", "status"}},
		"go test ./... 2>&1":           {{"go", "test", "./..."}},
		"cd sub && go build ./cmd/ugh": {{"cd", "sub"}, {"go", "build", "./cmd/ugh"}},
		"git log --oneline | head -5":  {{"git", "log", "--oneline"}, {"head", "-5"}},
		`grep -rn "a b" . >/dev/null`:  {{"grep", "-rn", `a\ b`, "."}},
		"ls *.go; pwd":                 {{"ls", "*.go"}, {"pwd"}},
		"git commit -m 'fix: x'":       {{"git", "commit", "-m", `fix:\ x`}},
		"rm '*' *":                     {{"rm", `\*`, "*"}},
	}
	for cmd, want := range ok {
		got, k := SimpleCommands(cmd)
		if !k || !reflect.DeepEqual(got, want) {
			t.Errorf("%q = %v %v, want %v", cmd, got, k, want)
		}
	}
	for _, cmd := range []string{
		"git status $(rm -rf ~)", "echo $HOME", "git status `id`", "FOO=1 make",
		"(cd x; make)", "{ ls; }", "make &", "ls > out.txt", "cat < secrets", "cat <<EOF\nx\nEOF",
		"for f in a; do rm $f; done", "f() { rm -rf ~; }", "echo $'\\x41'", "! true",
		"diff <(ls a) <(ls b)", "ls 3>file", "",
	} {
		if got, k := SimpleCommands(cmd); k {
			t.Errorf("%q accepted as plain: %v", cmd, got)
		}
	}
}
