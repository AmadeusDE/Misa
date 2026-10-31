package mounts

import (
	"errors"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
)

// mountinfoFixture is a realistic Linux /proc/self/mountinfo table: plain
// rows, optional tag fields, a bind mount, a tmpfs, duplicate mount points and
// octal escapes in mount points and sources.
const mountinfoFixture = `22 1 8:1 / / rw,relatime shared:1 - ext4 /dev/sda1 rw,errors=remount-ro
23 22 0:21 / /proc rw,nosuid,nodev,noexec,relatime - proc proc rw
24 22 0:22 / /sys rw,nosuid,nodev,noexec,relatime shared:2 - sysfs sysfs rw
25 22 0:23 / /tmp rw,noatime master:1 - tmpfs tmpfs rw,size=1024k

36 35 98:0 /mnt1 /mnt2 rw,noatime master:1 shared:2 - ext3 /dev/root rw,errors=continue
38 22 8:1 /home /mnt/backup rw,relatime - ext4 /dev/sda1 rw
40 22 8:4 / /mnt/with\040space rw,relatime - vfat /dev/disk/by-label/My\011Disk rw
41 22 0:31 / /mnt/back\134slash\040and\012newline rw,relatime shared:7 unbindable - tmpfs tmp\040fs rw
42 22 8:5 / /mnt/dup rw,relatime - ext4 /dev/sdb1 rw
43 22 8:6 / /mnt/dup rw,relatime - ext4 /dev/sdb2 rw`

func TestParseMountInfo(t *testing.T) {
	got, err := ParseMountInfo(strings.NewReader(mountinfoFixture))
	if err != nil {
		t.Fatalf("ParseMountInfo: %v", err)
	}
	want := []Mount{
		{Device: "/dev/sda1", Dir: "/", FSType: "ext4", Options: "rw,relatime", Root: "/"},
		{Device: "proc", Dir: "/proc", FSType: "proc", Options: "rw,nosuid,nodev,noexec,relatime", Root: "/"},
		{Device: "sysfs", Dir: "/sys", FSType: "sysfs", Options: "rw,nosuid,nodev,noexec,relatime", Root: "/"},
		{Device: "tmpfs", Dir: "/tmp", FSType: "tmpfs", Options: "rw,noatime", Root: "/"},
		{Device: "/dev/root", Dir: "/mnt2", FSType: "ext3", Options: "rw,noatime", Root: "/mnt1"},
		{Device: "/dev/sda1", Dir: "/mnt/backup", FSType: "ext4", Options: "rw,relatime", Root: "/home"},
		{Device: "/dev/disk/by-label/My\tDisk", Dir: "/mnt/with space", FSType: "vfat", Options: "rw,relatime", Root: "/"},
		{Device: "tmp fs", Dir: "/mnt/back\\slash and\nnewline", FSType: "tmpfs", Options: "rw,relatime", Root: "/"},
		{Device: "/dev/sdb1", Dir: "/mnt/dup", FSType: "ext4", Options: "rw,relatime", Root: "/"},
		{Device: "/dev/sdb2", Dir: "/mnt/dup", FSType: "ext4", Options: "rw,relatime", Root: "/"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d mounts, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("mount %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

const mountsFixture = `/dev/sda1 / ext4 rw,relatime 0 0
proc /proc proc rw,nosuid,nodev,noexec,relatime 0 0

tmpfs /run/user/1000 tmpfs rw,nosuid,nodev,relatime,size=1630300k 0 0
/dev/disk/by-label/My\040Disk /mnt/My\040Disk vfat rw,errors=remount-ro 0 0
server:/export /mnt/nfs\040share\011one nfs4 rw,vers=4.2 0 0
overlay /mnt/back\134slash overlay rw,lowerdir=/a\040b 0 0`

func TestParseMounts(t *testing.T) {
	got, err := ParseMounts(strings.NewReader(mountsFixture))
	if err != nil {
		t.Fatalf("ParseMounts: %v", err)
	}
	want := []Mount{
		{Device: "/dev/sda1", Dir: "/", FSType: "ext4", Options: "rw,relatime"},
		{Device: "proc", Dir: "/proc", FSType: "proc", Options: "rw,nosuid,nodev,noexec,relatime"},
		{Device: "tmpfs", Dir: "/run/user/1000", FSType: "tmpfs", Options: "rw,nosuid,nodev,relatime,size=1630300k"},
		{Device: "/dev/disk/by-label/My Disk", Dir: "/mnt/My Disk", FSType: "vfat", Options: "rw,errors=remount-ro"},
		{Device: "server:/export", Dir: "/mnt/nfs share\tone", FSType: "nfs4", Options: "rw,vers=4.2"},
		{Device: "overlay", Dir: "/mnt/back\\slash", FSType: "overlay", Options: "rw,lowerdir=/a\\040b"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d mounts, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("mount %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestUnescape(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"plain", "/mnt/data", "/mnt/data"},
		{"empty", "", ""},
		{"space", "/mnt/with\\040space", "/mnt/with space"},
		{"tab", "a\\011b", "a\tb"},
		{"newline", "a\\012b", "a\nb"},
		{"backslash", `C:\134path`, `C:\path`},
		{"mixed", "/mnt/a\\040b\\011c\\134d", "/mnt/a b\tc\\d"},
		{"trailing lone backslash", "abc\\", "abc\\"},
		{"short escape kept literal", "a\\0b", "a\\0b"},
		{"two digits kept literal", "a\\04b", "a\\04b"},
		{"three digits then digit", "a\\0404", "a 4"},
		{"non-octal digit kept literal", "a\\089b", "a\\089b"},
		{"double backslash then escape", "\\\\040", "\\ "},
		{"utf8 passthrough", "naïve/日本", "naïve/日本"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Unescape(tt.in); got != tt.want {
				t.Errorf("Unescape(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseMalformed(t *testing.T) {
	tests := []struct {
		name    string
		parse   func(io.Reader) ([]Mount, error)
		input   string
		wantSub string
	}{
		{
			name:    "mountinfo too few fields",
			parse:   ParseMountInfo,
			input:   "22 1 8:1 / / rw,relatime - ext4\n",
			wantSub: "mountinfo line 1",
		},
		{
			name:    "mountinfo missing separator",
			parse:   ParseMountInfo,
			input:   "22 1 8:1 / / rw,relatime shared:1 ext4 /dev/sda1 rw\n",
			wantSub: "mountinfo line 1",
		},
		{
			name:    "mountinfo separator without super options",
			parse:   ParseMountInfo,
			input:   "22 1 8:1 / / rw,relatime - ext4 /dev/sda1\n",
			wantSub: "mountinfo line 1",
		},
		{
			name:    "mountinfo second line bad",
			parse:   ParseMountInfo,
			input:   "22 1 8:1 / / rw - ext4 /dev/sda1 rw\n23 22 0:21 / /proc rw\n",
			wantSub: "mountinfo line 2",
		},
		{
			name:    "mounts too few fields",
			parse:   ParseMounts,
			input:   "/dev/sda1 / ext4 rw,relatime 0\n",
			wantSub: "mounts line 1",
		},
		{
			name:    "mounts second line bad",
			parse:   ParseMounts,
			input:   "/dev/sda1 / ext4 rw,relatime 0 0\nproc\n",
			wantSub: "mounts line 2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.parse(strings.NewReader(tt.input))
			if err == nil {
				t.Fatalf("expected an error, got %d mounts: %+v", len(got), got)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error %q does not mention %q", err, tt.wantSub)
			}
		})
	}
}

func TestParseEmpty(t *testing.T) {
	for name, parse := range map[string]func(io.Reader) ([]Mount, error){
		"mountinfo": ParseMountInfo,
		"mounts":    ParseMounts,
	} {
		t.Run(name, func(t *testing.T) {
			for _, in := range []string{"", "\n", "\n\n\n", "   \n"} {
				got, err := parse(strings.NewReader(in))
				if err != nil {
					t.Fatalf("parse(%q): %v", in, err)
				}
				if len(got) != 0 {
					t.Errorf("parse(%q) = %d mounts, want 0", in, len(got))
				}
			}
		})
	}
}

func TestMatch(t *testing.T) {
	table := []Mount{
		{Device: "/dev/root", Dir: "/"},
		{Device: "/dev/sda2", Dir: "/mnt2"},
		{Device: "/dev/sda3", Dir: "/mnt"},
		{Device: "/dev/sda5", Dir: "/mnt/data"},
		{Device: "/dev/sda6", Dir: "/var/"},
		{Device: "/dev/sdb1", Dir: "/srv/dup"},
		{Device: "/dev/sdb2", Dir: "/srv/dup"},
	}
	tests := []struct {
		name   string
		mounts []Mount
		path   string
		want   string // device of the expected mount, "" for no match
	}{
		{"exact root", table, "/", "/dev/root"},
		{"exact dir", table, "/mnt/data", "/dev/sda5"},
		{"under dir", table, "/mnt/data/file.txt", "/dev/sda5"},
		{"parent only", table, "/mnt/x", "/dev/sda3"},
		{"prefix is not under", table, "/mnt2/x", "/dev/sda2"},
		{"dir with trailing slash", table, "/var/log/dmesg", "/dev/sda6"},
		{"dir with trailing slash exact", table, "/var", "/dev/sda6"},
		{"duplicate dir picks later entry", table, "/srv/dup/a", "/dev/sdb2"},
		{"falls back to root", table, "/nowhere", "/dev/root"},
		{"relative path has no mount", table, "mnt/data", ""},
		{"empty path has no mount", table, "", ""},
		{"empty table", nil, "/", ""},
		{"no root mount", []Mount{{Device: "/dev/sda3", Dir: "/mnt"}}, "/srv", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Match(tt.mounts, tt.path)
			if tt.want == "" {
				if ok {
					t.Fatalf("Match(%q) = %+v, want no match", tt.path, got)
				}
				return
			}
			if !ok {
				t.Fatalf("Match(%q) found nothing, want %s", tt.path, tt.want)
			}
			if got.Device != tt.want {
				t.Errorf("Match(%q) = %s, want %s", tt.path, got.Device, tt.want)
			}
		})
	}
}

func TestListLive(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skipf("List reads /proc on linux only, not %s", runtime.GOOS)
	}
	if _, err := os.Stat("/proc/self/mountinfo"); err != nil {
		t.Skipf("/proc/self/mountinfo: %v", err)
	}
	got, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, m := range got {
		if m.Dir == "/" {
			return
		}
	}
	t.Skipf("no mount on / in this environment: %+v", got)
}

func TestStatfsLive(t *testing.T) {
	st, err := Statfs(t.TempDir())
	if errors.Is(err, ErrUnsupported) {
		t.Skipf("Statfs unsupported on %s", runtime.GOOS)
	}
	if err != nil {
		t.Fatalf("Statfs: %v", err)
	}
	if st.Bsize == 0 {
		t.Fatalf("Statfs returned Bsize 0: %+v", st)
	}
	if st.Blocks == 0 {
		t.Errorf("Statfs returned Blocks 0: %+v", st)
	}
}

func TestStatfsMissingPath(t *testing.T) {
	_, err := Statfs("/nonexistent/misa/mounts/test")
	if errors.Is(err, ErrUnsupported) {
		t.Skipf("Statfs unsupported on %s", runtime.GOOS)
	}
	if err == nil {
		t.Fatal("Statfs on a missing path returned no error")
	}
}
