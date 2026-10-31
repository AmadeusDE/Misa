package sysinfo

import (
	"path/filepath"
	"testing"
)

func TestMountSpaceReportsCapacity(t *testing.T) {
	total, free, err := mountSpace(t.TempDir())
	if err != nil {
		t.Skipf("no disk space provider: %v", err)
	}
	if total == 0 || free > total {
		t.Errorf("total=%d free=%d", total, free)
	}
}

func TestMountSpaceMissingPath(t *testing.T) {
	if _, _, err := mountSpace(filepath.Join(t.TempDir(), "gone")); err == nil {
		t.Error("want an error for a missing path")
	}
}
