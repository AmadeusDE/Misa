package builtins

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/AmadeusDE/Misa/internal/mounts"
)

// dfTestTable and dfTestStats are a hand-written mount table: a root
// filesystem, a tmpfs with nothing used, a full xfs, and a pseudo filesystem
// with no blocks at all.
func dfTestTable() []mounts.Mount {
	return []mounts.Mount{
		{Device: "/dev/sda1", Dir: "/", FSType: "ext4"},
		{Device: "tmpfs", Dir: "/run", FSType: "tmpfs"},
		{Device: "/dev/sdb1", Dir: "/mnt/data", FSType: "xfs"},
		{Device: "proc", Dir: "/proc", FSType: "proc"},
	}
}

func dfTestStats() map[string]mounts.Stat {
	return map[string]mounts.Stat{
		"/":         {Blocks: 1048576, Bfree: 524288, Bavail: 262144, Files: 655360, Ffree: 600000, Bsize: 4096},
		"/run":      {Blocks: 65536, Bfree: 65536, Bavail: 65536, Files: 16384, Ffree: 16384, Bsize: 4096},
		"/mnt/data": {Blocks: 2097152, Bfree: 0, Bavail: 0, Files: 1000, Ffree: 900, Bsize: 4096},
		"/proc":     {Bsize: 4096},
	}
}

// dfInstallFakes swaps the mount table and statfs for the duration of the
// test. A path missing from stats fails like an unreadable filesystem.
func dfInstallFakes(t *testing.T, table []mounts.Mount, stats map[string]mounts.Stat) {
	t.Helper()
	oldList, oldStatfs := dfListMounts, dfStatfs
	t.Cleanup(func() { dfListMounts, dfStatfs = oldList, oldStatfs })
	dfListMounts = func() ([]mounts.Mount, error) { return table, nil }
	dfStatfs = func(path string) (mounts.Stat, error) {
		if st, ok := stats[path]; ok {
			return st, nil
		}
		return mounts.Stat{}, &os.PathError{Op: "statfs", Path: path, Err: syscall.ENOENT}
	}
}

// dfDefaultHeader is the default header for a table whose values are no wider
// than the header text itself; wider values push the middle columns right.
func dfDefaultHeader() string {
	return "Filesystem     1K-blocks Used Available Use% Mounted on\n"
}

// dfWideHeader is the same header for a table holding seven-digit block counts.
func dfWideHeader() string {
	return "Filesystem     1K-blocks    Used Available Use% Mounted on\n"
}

func TestDfFullListing(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "default",
			want: dfWideHeader() +
				"/dev/sda1        4194304 2097152   1048576  67% /\n" +
				"tmpfs             262144       0    262144   0% /run\n" +
				"/dev/sdb1        8388608 8388608         0 100% /mnt/data\n",
		},
		{
			name: "-h",
			args: []string{"-h"},
			want: "Filesystem      Size Used Avail Use% Mounted on\n" +
				"/dev/sda1       4.0G 2.0G  1.0G  67% /\n" +
				"tmpfs           256M    0  256M   0% /run\n" +
				"/dev/sdb1       8.0G 8.0G     0 100% /mnt/data\n",
		},
		{
			name: "-H",
			args: []string{"-H"},
			want: "Filesystem      Size Used Avail Use% Mounted on\n" +
				"/dev/sda1       4.2G 2.1G  1.0G  67% /\n" +
				"tmpfs           268M    0  268M   0% /run\n" +
				"/dev/sdb1       8.5G 8.5G     0 100% /mnt/data\n",
		},
		{
			name: "-i",
			args: []string{"-i"},
			want: "Filesystem     Inodes IUsed  IFree IUse% Mounted on\n" +
				"/dev/sda1      655360 55360 600000    9% /\n" +
				"tmpfs           16384     0  16384    0% /run\n" +
				"/dev/sdb1        1000   100    900   10% /mnt/data\n",
		},
		{
			name: "-i -h keeps inode counts plain",
			args: []string{"-i", "-h"},
			want: "Filesystem      Inodes IUsed  IFree IUse% Mounted on\n" +
				"/dev/sda1       655360 55360 600000    9% /\n" +
				"tmpfs            16384     0  16384    0% /run\n" +
				"/dev/sdb1         1000   100    900   10% /mnt/data\n",
		},
		{
			name: "-P",
			args: []string{"-P"},
			want: "Filesystem     512-blocks     Used Available Capacity Mounted on\n" +
				"/dev/sda1         8388608  4194304   2097152      67% /\n" +
				"tmpfs              524288        0    524288       0% /run\n" +
				"/dev/sdb1        16777216 16777216         0     100% /mnt/data\n",
		},
		{
			name: "-P -k",
			args: []string{"-P", "-k"},
			want: "Filesystem     1024-blocks    Used Available Capacity Mounted on\n" +
				"/dev/sda1          4194304 2097152   1048576      67% /\n" +
				"tmpfs               262144       0    262144       0% /run\n" +
				"/dev/sdb1          8388608 8388608         0     100% /mnt/data\n",
		},
		{
			name: "-k",
			args: []string{"-k"},
			want: dfWideHeader() +
				"/dev/sda1        4194304 2097152   1048576  67% /\n" +
				"tmpfs             262144       0    262144   0% /run\n" +
				"/dev/sdb1        8388608 8388608         0 100% /mnt/data\n",
		},
		{
			name: "-T",
			args: []string{"-T"},
			want: "Filesystem      Type 1K-blocks    Used Available Use% Mounted on\n" +
				"/dev/sda1       ext4   4194304 2097152   1048576  67% /\n" +
				"tmpfs          tmpfs    262144       0    262144   0% /run\n" +
				"/dev/sdb1        xfs   8388608 8388608         0 100% /mnt/data\n",
		},
		{
			name: "-a shows pseudo filesystems",
			args: []string{"-a"},
			want: dfWideHeader() +
				"/dev/sda1        4194304 2097152   1048576  67% /\n" +
				"tmpfs             262144       0    262144   0% /run\n" +
				"/dev/sdb1        8388608 8388608         0 100% /mnt/data\n" +
				"proc                   0       0         0   0% /proc\n",
		},
		{
			name: "-B 4096",
			args: []string{"-B", "4096"},
			want: "Filesystem     4K-blocks    Used Available Use% Mounted on\n" +
				"/dev/sda1        1048576  524288    262144  67% /\n" +
				"tmpfs              65536       0     65536   0% /run\n" +
				"/dev/sdb1        2097152 2097152         0 100% /mnt/data\n",
		},
		{
			name: "-B 1M",
			args: []string{"-B", "1M"},
			want: "Filesystem     1M-blocks Used Available Use% Mounted on\n" +
				"/dev/sda1           4096 2048      1024  67% /\n" +
				"tmpfs                256    0       256   0% /run\n" +
				"/dev/sdb1           8192 8192         0 100% /mnt/data\n",
		},
		{
			name: "-B 1000 truncates and labels in SI units",
			args: []string{"-B", "1000"},
			want: "Filesystem     1kB-blocks    Used Available Use% Mounted on\n" +
				"/dev/sda1         4294967 2147483   1073741  67% /\n" +
				"tmpfs              268435       0    268435   0% /run\n" +
				"/dev/sdb1         8589934 8589934         0 100% /mnt/data\n",
		},
		{
			name: "-P -B uses the POSIX numeric label",
			args: []string{"-P", "-B", "1M"},
			want: "Filesystem     1048576-blocks Used Available Capacity Mounted on\n" +
				"/dev/sda1                4096 2048      1024      67% /\n" +
				"tmpfs                     256    0       256       0% /run\n" +
				"/dev/sdb1                8192 8192         0     100% /mnt/data\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dfInstallFakes(t, dfTestTable(), dfTestStats())
			r := runBuiltin(t, "df", tc.args...)
			assertExitCode(t, r, 0)
			if r.stdout != tc.want {
				t.Errorf("stdout =\n%s\nwant\n%s", r.stdout, tc.want)
			}
			if r.stderr != "" {
				t.Errorf("stderr = %q, want empty", r.stderr)
			}
		})
	}
}

func TestDfLongNamesWidenColumns(t *testing.T) {
	table := []mounts.Mount{
		{Device: "/dev/mapper/very-long-volume-name--for-widths", Dir: "/", FSType: "ext4"},
		{Device: "overlay", Dir: "/var/lib/a/really/long/mount/point/here", FSType: "overlay"},
	}
	stats := map[string]mounts.Stat{
		"/": {Blocks: 2097152, Bfree: 1048576, Bavail: 524288, Files: 100, Ffree: 90, Bsize: 4096},
		"/var/lib/a/really/long/mount/point/here": {Blocks: 524288, Bfree: 262144, Bavail: 262144, Files: 200, Ffree: 100, Bsize: 4096},
	}
	dfInstallFakes(t, table, stats)

	r := runBuiltin(t, "df")
	assertExitCode(t, r, 0)
	want := "Filesystem                                    1K-blocks    Used Available Use% Mounted on\n" +
		"/dev/mapper/very-long-volume-name--for-widths   8388608 4194304   2097152  67% /\n" +
		"overlay                                         2097152 1048576   1048576  50% /var/lib/a/really/long/mount/point/here\n"
	if r.stdout != want {
		t.Errorf("stdout =\n%s\nwant\n%s", r.stdout, want)
	}

	r = runBuiltin(t, "df", "-h")
	assertExitCode(t, r, 0)
	want = "Filesystem                                    Size Used Avail Use% Mounted on\n" +
		"/dev/mapper/very-long-volume-name--for-widths 8.0G 4.0G  2.0G  67% /\n" +
		"overlay                                       2.0G 1.0G  1.0G  50% /var/lib/a/really/long/mount/point/here\n"
	if r.stdout != want {
		t.Errorf("-h stdout =\n%s\nwant\n%s", r.stdout, want)
	}
}

func TestDfTypeFilters(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "-t keeps one type",
			args: []string{"-t", "ext4"},
			want: dfWideHeader() +
				"/dev/sda1        4194304 2097152   1048576  67% /\n",
		},
		{
			name: "repeated -t keeps several",
			args: []string{"-t", "ext4", "-t", "tmpfs"},
			want: dfWideHeader() +
				"/dev/sda1        4194304 2097152   1048576  67% /\n" +
				"tmpfs             262144       0    262144   0% /run\n",
		},
		{
			name: "-x drops one type",
			args: []string{"-x", "tmpfs"},
			want: dfWideHeader() +
				"/dev/sda1        4194304 2097152   1048576  67% /\n" +
				"/dev/sdb1        8388608 8388608         0 100% /mnt/data\n",
		},
		{
			name: "-a -t shows a zero-block type",
			args: []string{"-a", "-t", "proc"},
			want: dfDefaultHeader() +
				"proc                   0    0         0   0% /proc\n",
		},
		{
			name: "no match still prints the header",
			args: []string{"-t", "nfs4"},
			want: dfDefaultHeader(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dfInstallFakes(t, dfTestTable(), dfTestStats())
			r := runBuiltin(t, "df", tc.args...)
			assertExitCode(t, r, 0)
			if r.stdout != tc.want {
				t.Errorf("stdout =\n%s\nwant\n%s", r.stdout, tc.want)
			}
		})
	}

	t.Run("selected and excluded", func(t *testing.T) {
		dfInstallFakes(t, dfTestTable(), dfTestStats())
		r := runBuiltin(t, "df", "-t", "ext4", "-x", "ext4", "-t", "xfs", "-x", "xfs")
		assertExitCode(t, r, 1)
		want := "df: file system type 'ext4' both selected and excluded\n" +
			"df: file system type 'xfs' both selected and excluded\n"
		if r.stderr != want {
			t.Errorf("stderr = %q, want %q", r.stderr, want)
		}
		if r.stdout != "" {
			t.Errorf("stdout = %q, want empty", r.stdout)
		}
	})
}

func TestDfOverMountedRowsAreHidden(t *testing.T) {
	table := []mounts.Mount{
		{Device: "/dev/hidden", Dir: "/data", FSType: "ext4"},
		{Device: "/dev/visible", Dir: "/data", FSType: "xfs"},
	}
	stats := map[string]mounts.Stat{
		"/data": {Blocks: 2000, Bfree: 1000, Bavail: 500, Files: 100, Ffree: 50, Bsize: 1024},
	}
	dfInstallFakes(t, table, stats)

	r := runBuiltin(t, "df")
	assertExitCode(t, r, 0)
	want := dfDefaultHeader() +
		"/dev/visible        2000 1000       500  67% /data\n"
	if r.stdout != want {
		t.Errorf("stdout =\n%s\nwant\n%s", r.stdout, want)
	}
}

func TestDfOperands(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"sub", "sub2", "mnt/data/x"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "plain"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "denied"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	table := []mounts.Mount{
		{Device: "root-dev", Dir: "/", FSType: "ext4"},
		{Device: "mnt-dev", Dir: filepath.Join(dir, "mnt"), FSType: "xfs"},
		{Device: "data-dev", Dir: filepath.Join(dir, "mnt", "data"), FSType: "btrfs"},
		{Device: "tmp-dev", Dir: dir, FSType: "ext4"},
	}
	stats := map[string]mounts.Stat{
		filepath.Join(dir, "sub"):        {Blocks: 2000, Bfree: 1000, Bavail: 500, Files: 100, Ffree: 50, Bsize: 1024},
		filepath.Join(dir, "sub2"):       {Blocks: 4000, Bfree: 1000, Bavail: 3000, Files: 200, Ffree: 150, Bsize: 1024},
		filepath.Join(dir, "plain"):      {Blocks: 1024, Bfree: 512, Bavail: 512, Files: 10, Ffree: 5, Bsize: 512},
		filepath.Join(dir, "mnt/data/x"): {Blocks: 8000, Bfree: 4000, Bavail: 4000, Files: 400, Ffree: 300, Bsize: 1024},
		filepath.Join(dir, "denied"):     {},
	}
	dfInstallFakes(t, table, stats)
	deniedStatfs := dfStatfs
	dfStatfs = func(path string) (mounts.Stat, error) {
		if strings.HasSuffix(path, "/denied") {
			return mounts.Stat{}, &os.PathError{Op: "statfs", Path: path, Err: syscall.EACCES}
		}
		return deniedStatfs(path)
	}

	cases := []struct {
		name string
		args []string
		want string
		code int
		err  string
	}{
		{
			name: "relative operand resolved against the working directory",
			args: []string{"sub"},
			want: dfDefaultHeader() +
				"tmp-dev             2000 1000       500  67% " + dir + "\n",
		},
		{
			name: "longest mount prefix wins",
			args: []string{"mnt/data/x"},
			want: dfDefaultHeader() +
				"data-dev            8000 4000      4000  50% " + filepath.Join(dir, "mnt", "data") + "\n",
		},
		{
			name: "several operands give one row each",
			args: []string{"sub", "plain"},
			want: dfDefaultHeader() +
				"tmp-dev             2000 1000       500  67% " + dir + "\n" +
				"tmp-dev              512  256       256  50% " + dir + "\n",
		},
		{
			name: "a missing operand reports and keeps going",
			args: []string{"missing", "sub"},
			want: dfDefaultHeader() +
				"tmp-dev             2000 1000       500  67% " + dir + "\n",
			code: 1,
			err:  "df: 'missing': No such file or directory\n",
		},
		{
			name: "an unreadable filesystem fails",
			args: []string{"denied"},
			want: dfDefaultHeader(),
			code: 1,
			err:  "df: 'denied': Permission denied\n",
		},
		{
			name: "a filtered-out operand prints no row",
			args: []string{"-t", "xfs", "mnt/data/x"},
			want: dfDefaultHeader(),
		},
		{
			name: "a matching type prints the row",
			args: []string{"-P", "-t", "btrfs", "mnt/data/x"},
			want: "Filesystem     512-blocks Used Available Capacity Mounted on\n" +
				"data-dev            16000 8000      8000      50% " + filepath.Join(dir, "mnt", "data") + "\n",
		},
		{
			name: "an excluded type prints no row",
			args: []string{"-x", "btrfs", "mnt/data/x"},
			want: dfDefaultHeader(),
		},
		{
			name: "duplicate operands repeat the row",
			args: []string{"sub", "sub"},
			want: dfDefaultHeader() +
				"tmp-dev             2000 1000       500  67% " + dir + "\n" +
				"tmp-dev             2000 1000       500  67% " + dir + "\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := runBuiltinIn(t, dir, "df", tc.args...)
			assertExitCode(t, r, tc.code)
			if r.stdout != tc.want {
				t.Errorf("stdout =\n%s\nwant\n%s", r.stdout, tc.want)
			}
			if r.stderr != tc.err {
				t.Errorf("stderr = %q, want %q", r.stderr, tc.err)
			}
		})
	}
}

func TestDfOperandWithoutMountMatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "inner"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "inner")
	table := []mounts.Mount{{Device: "/dev/other", Dir: "/mnt/not-here", FSType: "ext4"}}
	stats := map[string]mounts.Stat{
		path: {Blocks: 2000, Bfree: 1000, Bavail: 500, Files: 100, Ffree: 50, Bsize: 1024},
	}
	dfInstallFakes(t, table, stats)

	r := runBuiltinIn(t, dir, "df", "inner")
	assertExitCode(t, r, 0)
	mountPoint := dfMountPoint(path)
	if !strings.HasPrefix(path, mountPoint) {
		t.Fatalf("dfMountPoint(%q) = %q, not an ancestor", path, mountPoint)
	}
	want := dfDefaultHeader() +
		"-                   2000 1000       500  67% " + mountPoint + "\n"
	if r.stdout != want {
		t.Errorf("stdout =\n%s\nwant\n%s", r.stdout, want)
	}

	r = runBuiltinIn(t, dir, "df", "-T", "inner")
	assertExitCode(t, r, 0)
	data := strings.Fields(strings.Split(strings.TrimSuffix(r.stdout, "\n"), "\n")[1])
	if len(data) < 2 || data[0] != "-" || data[1] != "-" {
		t.Errorf("-T row = %q, want '-' in the Filesystem and Type columns", strings.Join(data, " "))
	}
}

func TestDfMissingMountTable(t *testing.T) {
	oldList := dfListMounts
	t.Cleanup(func() { dfListMounts = oldList })
	dfListMounts = func() ([]mounts.Mount, error) {
		return nil, fmt.Errorf("/proc/self/mountinfo: %w", os.ErrNotExist)
	}

	r := runBuiltin(t, "df")
	assertExitCode(t, r, 1)
	if want := "df: cannot read table of mounted file systems: /proc/self/mountinfo: file does not exist\n"; r.stderr != want {
		t.Errorf("stderr = %q, want %q", r.stderr, want)
	}
	if r.stdout != "" {
		t.Errorf("stdout = %q, want empty", r.stdout)
	}

	// An operand still works: the device and type columns fall back to "-".
	dir := t.TempDir()
	stats := map[string]mounts.Stat{
		dir: {Blocks: 2000, Bfree: 1000, Bavail: 500, Files: 100, Ffree: 50, Bsize: 1024},
	}
	dfInstallFakes(t, nil, stats)
	dfListMounts = func() ([]mounts.Mount, error) { return nil, mounts.ErrUnsupported }
	r = runBuiltinIn(t, dir, "df", ".")
	assertExitCode(t, r, 0)
	if !strings.Contains(r.stderr, "cannot read table of mounted file systems") {
		t.Errorf("stderr = %q, want the mount table warning", r.stderr)
	}
	if want := "-                   2000 1000       500  67% " + dfMountPoint(dir) + "\n"; r.stdout[len("Filesystem     1K-blocks Used Available Use% Mounted on\n"):] != want {
		t.Errorf("row = %q, want %q", r.stdout, want)
	}
}

func TestDfRejectsUnsupportedOptions(t *testing.T) {
	dfInstallFakes(t, dfTestTable(), dfTestStats())
	for _, arg := range []string{"--output=x", "-l", "--local", "--total", "--sync", "-m", "-B"} {
		t.Run(arg, func(t *testing.T) {
			r := runBuiltin(t, "df", arg)
			assertExitCode(t, r, 1)
			if r.stdout != "" {
				t.Errorf("stdout = %q, want empty", r.stdout)
			}
			if !strings.Contains(r.stderr, "option") {
				t.Errorf("stderr = %q, want an option error", r.stderr)
			}
		})
	}
}

func TestDfInvalidBlockSize(t *testing.T) {
	dfInstallFakes(t, dfTestTable(), dfTestStats())
	cases := map[string]string{
		"x":   "df: invalid --block-size argument 'x'\n",
		"0":   "df: invalid --block-size argument '0'\n",
		"1fb": "df: invalid suffix in --block-size argument '1fb'\n",
		"1H":  "df: invalid suffix in --block-size argument '1H'\n",
	}
	for arg, want := range cases {
		t.Run(arg, func(t *testing.T) {
			r := runBuiltin(t, "df", "--block-size="+arg)
			assertExitCode(t, r, 1)
			if r.stderr != want {
				t.Errorf("stderr = %q, want %q", r.stderr, want)
			}
			if r.stdout != "" {
				t.Errorf("stdout = %q, want empty", r.stdout)
			}
		})
	}
}

// runDfNamed runs df with CmdName set, which the shared test helper leaves
// empty and the usage text needs.
func runDfNamed(t *testing.T, args ...string) runResult {
	t.Helper()
	cmd, ok := Lookup("df")
	if !ok {
		t.Fatal("builtin df not registered")
	}
	var stdout, stderr bytes.Buffer
	code := cmd.Run(&Context{Dir: t.TempDir(), CmdName: "df", Args: args, Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader("")})
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func TestDfHelpAndVersion(t *testing.T) {
	r := runDfNamed(t, "--help")
	assertExitCode(t, r, 0)
	for _, want := range []string{"Usage: df", "--all", "--human-readable", "--si", "--inodes",
		"--portability", "--type", "--exclude-type", "--print-type", "--block-size"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("--help missing %q:\n%s", want, r.stdout)
		}
	}

	if r := runDfNamed(t, "--version"); r.code != 0 || !strings.Contains(r.stdout, "df") {
		t.Errorf("--version: code %d, stdout %q", r.code, r.stdout)
	}
}

func TestDfLiveOperand(t *testing.T) {
	if _, err := mounts.List(); err != nil {
		if errors.Is(err, mounts.ErrUnsupported) {
			t.Skip("no mount table on this platform")
		}
		t.Skipf("cannot read the mount table: %v", err)
	}
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "df", ".")
	assertExitCode(t, r, 0)
	lines := strings.Split(strings.TrimSuffix(r.stdout, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("want a header and at least one row, got %q", r.stdout)
	}
	want := []string{"Filesystem", "1K-blocks", "Used", "Available", "Use%", "Mounted", "on"}
	if got := strings.Fields(lines[0]); len(got) != len(want) {
		t.Errorf("header = %q, want the default header", lines[0])
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("header = %q, want the default header", lines[0])
				break
			}
		}
	}
	if !strings.HasPrefix(lines[0], "Filesystem  ") {
		t.Errorf("header = %q, want the Filesystem column at least 14 wide", lines[0])
	}
}

func TestDfLiveListing(t *testing.T) {
	if _, err := dfListMounts(); err != nil {
		if errors.Is(err, mounts.ErrUnsupported) {
			t.Skip("no mount table on this platform")
		}
		t.Skipf("cannot read the mount table: %v", err)
	}
	r := runBuiltin(t, "df")
	assertExitCode(t, r, 0)
	if !strings.HasPrefix(r.stdout, "Filesystem ") {
		t.Errorf("stdout = %q, want the default header first", r.stdout)
	}
}

func TestHumanReadable(t *testing.T) {
	cases := []struct {
		n    uint64
		si   bool
		want string
	}{
		{0, false, "0"},
		{1, false, "1"},
		{999, false, "999"},
		{1023, false, "0.9K"},
		{1024, false, "1.0K"},
		{1536, false, "1.5K"},
		{8192, false, "8.0K"},
		{10240, false, "10K"},
		{1000000, false, "977K"},
		{1048576, false, "1.0M"},
		{268435456, false, "256M"},
		{4294967296, false, "4.0G"},
		{8589934592, false, "8.0G"},
		{1099511627776, false, "1.0T"},
		{999, true, "999"},
		{1000, true, "1.0k"},
		{1024, true, "1.0k"},
		{1500000, true, "1.5M"},
		{4294967296, true, "4.2G"},
	}
	for _, tc := range cases {
		if got := humanReadable(tc.n, tc.si); got != tc.want {
			t.Errorf("humanReadable(%d, si=%v) = %q, want %q", tc.n, tc.si, got, tc.want)
		}
	}
}

// TestDfBlockSizeLabels covers the -B column names uutils checks in
// test_block_size_1024, test_block_size_with_suffix and
// test_block_size_in_posix_portability_mode.
func TestDfBlockSizeLabels(t *testing.T) {
	cases := []struct {
		bytes       int64
		portability bool
		want        string
	}{
		{1, false, "1B-blocks"},
		{512, false, "512B-blocks"},
		{1000, false, "1kB-blocks"},
		{1024, false, "1K-blocks"},
		{2048, false, "2K-blocks"},
		{4096, false, "4K-blocks"},
		{128000, false, "128kB-blocks"},
		{1000 * 1024, false, "1.1MB-blocks"},
		{1 << 20, false, "1M-blocks"},
		{34 << 30, false, "34G-blocks"},
		{1 << 30, false, "1G-blocks"},
		{1000000000000, false, "1TB-blocks"},
		{1 << 20, true, "1048576-blocks"},
		{1000, true, "1000-blocks"},
	}
	for _, tc := range cases {
		opts := &dfOptions{blockSize: tc.bytes, portability: tc.portability}
		if got := opts.sizeLabel(); got != tc.want {
			t.Errorf("-B %d (portability=%v) = %q, want %q", tc.bytes, tc.portability, got, tc.want)
		}
	}

	for _, tc := range []struct {
		opts *dfOptions
		want string
	}{
		{&dfOptions{unit: 1024}, "1K-blocks"},
		{&dfOptions{unit: 512, portability: true}, "512-blocks"},
		{&dfOptions{unit: 1024, portability: true, kilo: true}, "1024-blocks"},
	} {
		if got := tc.opts.sizeLabel(); got != tc.want {
			t.Errorf("%+v = %q, want %q", tc.opts, got, tc.want)
		}
	}
}

func TestDfMountPoint(t *testing.T) {
	dir := t.TempDir()
	inner := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(inner, "f")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	got := dfMountPoint(inner)
	if !filepath.IsAbs(got) {
		t.Fatalf("dfMountPoint(%q) = %q, want an absolute path", inner, got)
	}
	if got != "/" && !strings.HasPrefix(inner+"/", got+"/") {
		t.Fatalf("dfMountPoint(%q) = %q, want an ancestor", inner, got)
	}
	if got != dfMountPoint(dir) {
		t.Errorf("dfMountPoint(%q) = %q, differs from the temp dir's %q", inner, got, dfMountPoint(dir))
	}
	if got := dfMountPoint(file); got != dfMountPoint(inner) {
		t.Errorf("dfMountPoint(%q) = %q, want the containing directory's mount point", file, got)
	}
	if got := dfMountPoint(filepath.Join(dir, "missing")); got != filepath.Join(dir, "missing") {
		t.Errorf("dfMountPoint of a missing path = %q, want the path itself", got)
	}
}
