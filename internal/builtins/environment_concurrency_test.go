package builtins

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func TestConcurrentInvocationEnvironmentIsolation(t *testing.T) {
	before := os.Environ()
	const count = 24
	dirs := make([]string, count)
	for i := range dirs {
		dirs[i] = t.TempDir()
		for name, contents := range map[string]string{"source": "new", "dest": "old", "f03-program": ""} {
			if err := os.WriteFile(filepath.Join(dirs[i], name), []byte(contents), 0o700); err != nil {
				t.Fatal(err)
			}
		}
	}
	var wg sync.WaitGroup
	for i, dir := range dirs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, suffix := fmt.Sprintf("value-%d", i), fmt.Sprintf(".backup-%d", i)
			zone, offset := "Etc/GMT+7", "-0700"
			if i%2 == 0 {
				zone, offset = "Etc/GMT-9", "+0900"
			}
			env := []string{"PATH=" + dir, "TZ=" + zone, "SIMPLE_BACKUP_SUFFIX=" + suffix, "VERSION_CONTROL=simple", "F03_VALUE=" + value}
			run := func(name string, args []string, want string) {
				var out, errout bytes.Buffer
				code := Run(nil, name, &Context{Dir: dir, Env: env, Args: args, Stdout: &out, Stderr: &errout})
				if code != 0 || out.String() != want {
					t.Errorf("worker %d %s %v: status=%d stdout=%q stderr=%q want=%q", i, name, args, code, out.String(), errout.String(), want)
				}
			}
			for range 4 {
				run("printenv", []string{"F03_VALUE"}, value+"\n")
				run("env", []string{"F03_NESTED=" + value, "printenv", "F03_NESTED"}, value+"\n")
				run("env", []string{"-i", "printenv"}, "")
				run("which", []string{"f03-program"}, filepath.Join(dir, "f03-program")+"\n")
				run("date", []string{"-d", "@0", "+%z"}, offset+"\n")
			}
			run("cp", []string{"-b", "source", "dest"}, "")
			contents, err := os.ReadFile(filepath.Join(dir, "dest"+suffix))
			if err != nil || string(contents) != "old" {
				t.Errorf("worker %d backup: %q, %v", i, contents, err)
			}
		}()
	}
	wg.Wait()
	if after := os.Environ(); !reflect.DeepEqual(before, after) {
		t.Fatal("concurrent builtins changed the process environment")
	}
}
