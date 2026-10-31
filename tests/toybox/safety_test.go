package toybox

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafetyClassifier(t *testing.T) {
	for _, tc := range []struct {
		script string
		safe   bool
	}{
		{`echo hi > file; cat file | wc -c`, true},
		{`mkdir -p dir && touch dir/file`, true},
		{`echo hi > ../outside`, false},
		{`echo hi > /tmp/outside`, false},
		{`sudo touch file`, false},
		{`curl example.com`, false},
		{`/usr/bin/curl example.com`, false},
		{`cat /dev/zero | wc -c`, false},
		{`touch $(echo file)`, true},
		{`eval "$X"`, false},
		{`touch "$OUT"`, true},
		{`echo hi > "$OUT"`, true},
		{`mount -t tmpfs tmpfs x`, false},
		{`echo hi 2>/dev/null`, true},
		{`sed s/a/b/ file`, true},
		{`for U in one two; do`, true},
		{`env LC_ALL=C printf ok`, true},
		{`timeout 1 printf ok`, true},
		{`touch /tmp/bad`, false},
		{`truncate -s /3k file`, true},
		{`mknod file p`, false},
		{`PATH=/tmp evil`, false},
	} {
		err := validateScript(tc.script)
		if (err == nil) != tc.safe {
			t.Errorf("%q: err=%v, safe=%v", tc.script, err, tc.safe)
		}
	}
}

func TestSafetySetupPaths(t *testing.T) {
	for _, p := range []string{"../outside", "/tmp/outside", "dir/../../outside"} {
		if err := validateSetup([]setupEvent{{line: 1, name: p}}, 1); err == nil {
			t.Errorf("accepted %q", p)
		}
	}
}

func TestSafetySymlinkSetupPath(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if err := validateSetupPath(dir, "link/file"); err == nil {
		t.Fatal("accepted symlink escape")
	}
	if err := validateSetupPath(dir, "new/file"); err != nil {
		t.Fatal(err)
	}
}
