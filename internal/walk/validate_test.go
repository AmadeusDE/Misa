package walk

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateDirsSwap(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "victim"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	var sawError, sawVictim bool
	err := Walk(context.Background(), root, Options{ValidateDirs: true}, func(e Entry) error {
		if e.Err != nil {
			sawError = true
			return nil
		}
		if e.Path == filepath.Join(root, "sub") {
			if err := os.Rename(e.Path, e.Path+"-old"); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, e.Path); err != nil {
				t.Fatal(err)
			}
		}
		if filepath.Base(e.Path) == "victim" {
			sawVictim = true
		}
		return nil
	})
	if err != nil || !sawError || sawVictim {
		t.Fatalf("err=%v sawError=%v sawVictim=%v", err, sawError, sawVictim)
	}
}

func TestValidateSnapshotAfterWalk(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "f"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	var validate func() error
	if err := Walk(context.Background(), root, Options{ValidateDirs: true}, func(e Entry) error {
		if filepath.Base(e.Path) == "f" {
			validate = e.Validate
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if validate == nil || validate() != nil {
		t.Fatal("missing or invalid snapshot")
	}
	if err := os.Rename(sub, sub+"-old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := validate(); err == nil {
		t.Fatal("snapshot failed to detect changed directory")
	}
}

func TestLiteralPathsSymlinkDotDot(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(outside, "child"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "child"), filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "victim"), []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "victim"), []byte("local"), 0600); err != nil {
		t.Fatal(err)
	}
	var got int64
	err := Walk(context.Background(), "link/../victim", Options{Base: dir, LiteralPaths: true}, func(e Entry) error {
		if e.Err != nil {
			t.Fatal(e.Err)
		}
		got = e.Info.Size()
		return nil
	})
	if err != nil || got != int64(len("outside")) {
		t.Fatalf("err=%v size=%d", err, got)
	}
}
