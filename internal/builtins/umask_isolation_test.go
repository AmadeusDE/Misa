package builtins

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/AmadeusDE/Misa/internal/proceffect"
)

// Reading the creation policy must not open a permissive window for another pane.
func TestUmaskReadersPreserveCreationPolicy(t *testing.T) {
	mask, err := proceffect.Umask()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var out, stderr bytes.Buffer
			path := fmt.Sprintf("tree%d/leaf", i)
			if code := Run(context.Background(), "mkdir", &Context{Dir: dir, Args: []string{"-p", path}, Stdout: &out, Stderr: &stderr}); code != 0 {
				t.Errorf("mkdir: %d %s", code, &stderr)
				return
			}
			file := filepath.Join(dir, fmt.Sprintf("file%d", i))
			if err := os.WriteFile(file, nil, 0666); err != nil {
				t.Error(err)
				return
			}
			if info, err := os.Stat(file); err != nil {
				t.Error(err)
			} else if info.Mode().Perm() != os.FileMode(0666&^mask) {
				t.Errorf("creation mode: %04o, mask %04o", info.Mode().Perm(), mask)
			}
			if code := Run(context.Background(), "chmod", &Context{Dir: dir, Args: []string{"=rw", file}, Stderr: &stderr}); code != 0 {
				t.Errorf("chmod: %d %s", code, &stderr)
			}
			if got, err := proceffect.Umask(); err != nil || got != mask {
				t.Errorf("mask %04o, %v; want %04o", got, err, mask)
			}
		}(i)
	}
	wg.Wait()
}
