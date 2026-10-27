package native

import (
	"testing"
)

func TestChown_MissingOperand(t *testing.T) {
	r := run(t, "chown")
	wantCode(t, r, 1)
	wantErrContains(t, r, "missing operand")
}

func TestChown_NonExistingFile(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "chown", "root", "does-not-exist")
	wantCode(t, r, 1)
}

func TestChown_SilentFlag(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "chown", "-f", "root", "does-not-exist")
	wantCode(t, r, 1)
	wantErr(t, r, "")
}

func TestChown_InvalidUserGroup(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "content")
	r := runIn(t, dir, "chown", "nonexistent_user_xyz_123:nonexistent_group_xyz_123", "f.txt")
	wantCode(t, r, 1)
}
