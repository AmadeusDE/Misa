package interp

import (
	"context"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/sh/expand"
	"github.com/AmadeusDE/Misa/internal/sh/syntax"
)

func TestSessionExportKeepValue(t *testing.T) {
	r, _ := New(Env(expand.ListEnviron()), ExecHandler(func(context.Context, []string) error { return nil }))
	f, err := syntax.NewParser(syntax.Variant(syntax.LangPOSIX)).Parse(strings.NewReader("x=hello; f() { :; }; export x; cd /"), "")
	if err != nil {
		t.Fatal(err)
	}
	if err = r.Run(context.Background(), f); err != nil {
		t.Fatal(err)
	}
	v := r.Vars["x"]
	if !v.Set || !v.Exported || v.Str != "hello" {
		t.Fatalf("%+v", v)
	}
}
