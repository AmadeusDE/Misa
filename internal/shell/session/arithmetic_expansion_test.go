package session

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestArithmeticExpansion(t *testing.T) {
	runExpansionCases(t, []expansionCase{
		{name: "precedence", src: `printf '%s:%s:%s:%s' "$((1+2*3))" "$(((1+2)*3))" "$((8-3-2))" "$((1+2<<2))"`, out: "7:9:3:12"},
		{name: "nonzero ternary", src: `printf '%s:%s' "$((2?3:4))" "$((0?3:4))"`, out: "3:4"},
		{name: "logical short circuit", src: `printf '%s:%s' "$((0 && 1/0))" "$((1 || 1/0))"`, out: "0:1"},
		{name: "integer bases", src: `printf '%s:%s' "$((010))" "$((0x10))"`, out: "8:16"},
		{name: "variable bases", src: `x=010; printf '%s:%s:' "$((x))" "$((x+=1))"; x=0x10; printf '%s' "$((x+=1))"`, out: "8:9:17"},
		{name: "invalid compound variable", src: `x=09; printf '%s' "$((x+=1))"`, stderr: "invalid arithmetic number: 09\n", status: 1},
		{name: "assignment replaces invalid old value", src: `x=09; printf '%s:%s' "$((x=2))" "$x"`, out: "2:2"},
		{name: "invalid octal", src: `printf '%s' "$((09))"`, stderr: "invalid arithmetic number: 09\n", status: 1},
		{name: "variables", src: `x=7; y=3; printf '%s:%s' "$((x*y+1))" "$((missing+2))"`, out: "22:2"},
		{name: "assignment", src: `x=1; printf '%s:' "$((x=4))" "$((x+=3))" "$x"`, out: "4:7:7:"},
		{name: "division by zero", src: `printf '%s' "$((1/0))"`, stderr: "division by zero\n", status: 1},
		{name: "division aborts submission", src: `printf '%s' "$((1/0))"; printf bad`, stderr: "division by zero\n", status: 1},
		{name: "remainder by zero", src: `printf '%s' "$((1%0))"`, stderr: "division by zero\n", status: 1},
	})
}

func TestArithmeticExpansionSyntaxError(t *testing.T) {
	dir := t.TempDir()
	var out, stderr bytes.Buffer
	s, err := New(Options{Dir: dir, Env: []string{"PATH=", "HOME=" + dir}, Stdout: &out, Stderr: &stderr})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := s.Run(context.Background(), `printf '%s' "$((1+))"`)
	var syntaxErr *SyntaxErr
	if r.Status != 2 || !errors.As(r.Err, &syntaxErr) || r.ParseError != syntaxErr || out.String() != "" || stderr.String() != "" {
		t.Fatalf("status=%d err=%v parse=%v stdout=%q stderr=%q; want status=2, syntax error, empty output", r.Status, r.Err, r.ParseError, out.String(), stderr.String())
	}
}
