package builtins

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestFindInvocationIsolation(t *testing.T) {
	fixed := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	type invocation struct {
		ctx         *Context
		out, stderr bytes.Buffer
		want        string
	}
	runs := make([]*invocation, 2)
	for i, zone := range []string{"UTC", "Asia/Tokyo"} {
		dir := t.TempDir()
		name := fmt.Sprintf("file-%d", i)
		file := filepath.Join(dir, name)
		if err := os.WriteFile(file, nil, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(file, fixed, fixed); err != nil {
			t.Fatal(err)
		}
		r := &invocation{}
		r.ctx = &Context{Dir: dir, Env: []string{"TZ=" + zone}, Args: []string{"-type", "f", "-mmin", "0", "-printf", "%f %t"}, Stdout: &r.out, Stderr: &r.stderr, Clock: func() time.Time { return fixed }}
		r.want = name + " " + fixed.In(r.ctx.Location()).Format("Mon Jan _2 15:04:05.0000000000 2006")
		runs[i] = r
	}
	var wg sync.WaitGroup
	for _, r := range runs {
		wg.Go(func() {
			if code := runFind(r.ctx); code != 0 || r.stderr.Len() != 0 || r.out.String() != r.want {
				t.Errorf("code=%d stdout=%q stderr=%q want=%q", code, r.out.String(), r.stderr.String(), r.want)
			}
		})
	}
	wg.Wait()
}
