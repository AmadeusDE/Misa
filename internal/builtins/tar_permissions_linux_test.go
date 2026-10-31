//go:build linux && !android

// Tar permission regression; Toybox tar.c and tar.test are attributed in tarx/NOTICE.
package builtins

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestTarUnreadableSourceContinues(t *testing.T) {
	if dir := os.Getenv("MISA_TAR_PERMISSION_HELPER"); dir != "" {
		code, data, diagnostic := tarRun(dir, nil, "-c", "unreadable", "readable")
		if code != 2 || !strings.Contains(diagnostic, "unreadable") {
			t.Fatalf("status=%d diagnostic=%q", code, diagnostic)
		}
		r := tar.NewReader(bytes.NewReader(data))
		h, err := r.Next()
		if err != nil {
			t.Fatal(err)
		}
		contents, err := io.ReadAll(r)
		if err != nil || h.Name != "readable" || string(contents) != "accessible" {
			t.Fatalf("header=%#v data=%q error=%v", h, contents, err)
		}
		if _, err = r.Next(); err != io.EOF {
			t.Fatalf("unexpected next entry: %v", err)
		}
		return
	}
	dir, err := os.MkdirTemp("", "tar-permission-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	if err = os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "unreadable"), []byte("secret"), 0000); err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(filepath.Join(dir, "unreadable"), 0000); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "readable"), []byte("accessible"), 0644); err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(filepath.Join(dir, "readable"), 0644); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// The go test executable's parent is often mode 0700. Copy it to the
	// accessible fixture directory before dropping privileges.
	src, err := os.Open(exe)
	if err != nil {
		t.Fatal(err)
	}
	dst, err := os.OpenFile(filepath.Join(dir, "testbin"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		src.Close()
		t.Fatal(err)
	}
	_, copyErr := io.Copy(dst, src)
	closeErr := dst.Close()
	src.Close()
	if copyErr != nil {
		t.Fatal(copyErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if err = os.Chmod(filepath.Join(dir, "testbin"), 0755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(filepath.Join(dir, "testbin"), "-test.run=^TestTarUnreadableSourceContinues$")
	cmd.Env = append(os.Environ(), "MISA_TAR_PERMISSION_HELPER="+dir)
	if os.Geteuid() == 0 {
		cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65534, Gid: 65534, NoSetGroups: true}}
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("helper: %v\n%s", err, out)
	}
}
