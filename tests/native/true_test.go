package native

import (
	"testing"
)

func TestTrue_Basic(t *testing.T) {
	r := run(t, "true")
	wantSuccess(t, r)
	wantOut(t, r, "")
	wantErr(t, r, "")
}

func TestTrue_WithArgs(t *testing.T) {
	r := run(t, "true", "foo", "bar", "--baz")
	wantSuccess(t, r)
	wantOut(t, r, "")
	wantErr(t, r, "")
}
