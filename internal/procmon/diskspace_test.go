package procmon

import (
	"path/filepath"
	"testing"
)

func TestDiskSpaceReportsCapacity(t *testing.T) {
	total, free, err := diskSpace(t.TempDir())
	if err != nil {
		t.Skipf("no disk space provider: %v", err)
	}
	if total == 0 || free > total {
		t.Errorf("total=%d free=%d", total, free)
	}
}

func TestDiskSpaceMissingPath(t *testing.T) {
	if _, _, err := diskSpace(filepath.Join(t.TempDir(), "gone")); err == nil {
		t.Error("want an error for a missing path")
	}
}
