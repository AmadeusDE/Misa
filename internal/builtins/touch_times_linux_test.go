package builtins

import (
	"os"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func fileTimes(t *testing.T, path string) (atime, mtime time.Time) {
	t.Helper()
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		t.Fatal(err)
	}
	return time.Unix(st.Atim.Unix()), time.Unix(st.Mtim.Unix())
}

func touchTimesFixture(t *testing.T) (dir, path string, atime, mtime time.Time) {
	t.Helper()
	dir = t.TempDir()
	path = writeFile(t, dir, "file.txt", "content")
	atime = time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	mtime = time.Date(2002, 3, 4, 5, 6, 7, 0, time.UTC)
	if err := os.Chtimes(path, atime, mtime); err != nil {
		t.Fatal(err)
	}
	return dir, path, atime, mtime
}

func TestTouchTimeSelection(t *testing.T) {
	stamp := time.Date(2010, 1, 2, 3, 4, 0, 0, time.UTC)
	tests := []struct {
		name         string
		args         []string
		changesAtime bool
		changesMtime bool
		toStamp      bool
	}{
		{"access only now", []string{"-a"}, true, false, false},
		{"modify only now", []string{"-m"}, false, true, false},
		{"both now", []string{"-a", "-m"}, true, true, false},
		{"access only stamp", []string{"-a", "-d", "2010-01-02 03:04:00Z"}, true, false, true},
		{"modify only stamp", []string{"-m", "-d", "2010-01-02 03:04:00Z"}, false, true, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir, path, oldA, oldM := touchTimesFixture(t)
			r := runBuiltinIn(t, dir, "touch", append(tc.args, "file.txt")...)
			assertExitCode(t, r, 0)
			gotA, gotM := fileTimes(t, path)
			check := func(which string, got, old time.Time, changed bool) {
				t.Helper()
				switch {
				case !changed && !got.Equal(old):
					t.Errorf("%s changed to %v, want unchanged %v", which, got, old)
				case changed && tc.toStamp && !got.Equal(stamp):
					t.Errorf("%s = %v, want %v", which, got, stamp)
				case changed && !tc.toStamp && time.Since(got) > time.Minute:
					t.Errorf("%s = %v, want about now", which, got)
				}
			}
			check("atime", gotA, oldA, tc.changesAtime)
			check("mtime", gotM, oldM, tc.changesMtime)
		})
	}
}
