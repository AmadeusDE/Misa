package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFindNumber(t *testing.T) {
	for _, tt := range []struct {
		spec string
		n    int64
		want bool
	}{{"3", 3, true}, {"+3", 4, true}, {"+3", 3, false}, {"-3", 2, true}, {"-0", 0, false}} {
		cmp, err := findNumber(tt.spec)
		if err != nil || cmp(tt.n) != tt.want {
			t.Errorf("%q %d: %v", tt.spec, tt.n, err)
		}
	}
	for _, s := range []string{"", "+", "-1x", "1.5"} {
		if _, err := findNumber(s); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
}

func TestFindAge(t *testing.T) {
	now := time.Unix(1000000, 0)
	for _, tt := range []struct {
		ago  time.Duration
		want int64
	}{{0, 0}, {23 * time.Hour, 0}, {24 * time.Hour, 1}, {-time.Second, -1}} {
		if got := findAge(now, now.Add(-tt.ago), 24*time.Hour); got != tt.want {
			t.Errorf("age %v = %d want %d", tt.ago, got, tt.want)
		}
	}
}

func TestFindPerm(t *testing.T) {
	for _, s := range []string{"755", "-u=rw,g=r,o=r", "/111", "u+x"} {
		if _, _, err := findParsePerm(s); err != nil {
			t.Errorf("%q: %v", s, err)
		}
	}
	for _, s := range []string{"", "888", "u+z"} {
		if _, _, err := findParsePerm(s); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
}

func TestFindPredicates(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "File.txt")
	if err := os.WriteFile(full, make([]byte, 513), 0640); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1700000000, 0)
	old := now.Add(-25 * time.Hour)
	if err := os.Chtimes(full, old, old); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(full)
	if err != nil {
		t.Fatal(err)
	}
	ctx := &Context{Dir: dir}
	e := &findEntry{ctx: ctx, path: "./dir/File.txt", full: full, root: ".", info: info, now: now, depth: 2}
	tests := []struct {
		name, arg string
		want      bool
	}{
		{"-name", "File.*", true}, {"-name", "file.*", false}, {"-iname", "file.*", true}, {"-path", "*File.txt", true}, {"-path", "./*/File.txt", true}, {"-regex", `./dir/File\.txt`, true}, {"-regex", `File\.txt`, false}, {"-iregex", `./DIR/FILE\.TXT`, true},
		{"-type", "d,f", true}, {"-type", "l", false}, {"-size", "2", true}, {"-size", "1k", true}, {"-size", "+512c", true}, {"-size", "-514c", true}, {"-size", "257w", true}, {"-size", "0", false},
		{"-empty", "", false}, {"-mtime", "1", true}, {"-mtime", "+0", true}, {"-mmin", "1500", true}, {"-perm", "640", true}, {"-perm", "-600", true}, {"-perm", "/001", false}, {"-perm", "u=rw,g=r", true}, {"-links", "1", true}, {"-samefile", "File.txt", true},
	}
	for _, tt := range tests {
		t.Run(tt.name+"/"+tt.arg, func(t *testing.T) {
			p := findParser{ctx: ctx, args: []string{tt.arg}}
			n, err := p.predicate(tt.name)
			if err != nil {
				t.Fatal(err)
			}
			e.err = nil
			if got := n(e); got != tt.want || e.err != nil {
				t.Fatalf("got %v err %v want %v", got, e.err, tt.want)
			}
		})
	}
	for _, tt := range []struct{ name, arg string }{{"-type", "q"}, {"-size", "1z"}, {"-regex", "["}, {"-perm", "u+z"}, {"-uid", "-x"}, {"-unknown", ""}} {
		p := findParser{ctx: ctx, args: []string{tt.arg}}
		if _, err := p.predicate(tt.name); err == nil {
			t.Errorf("accepted %s %q", tt.name, tt.arg)
		}
	}
}

func TestFindReferenceAndLinks(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	link := filepath.Join(dir, "link")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("file", link); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	ctx := &Context{Dir: dir}
	e := &findEntry{ctx: ctx, path: "link", full: link, info: info, now: time.Now()}
	for _, tt := range []struct {
		name, arg string
		want      bool
	}{{"-type", "l", true}, {"-xtype", "f", true}, {"-samefile", "link", true}, {"-samefile", "file", false}} {
		p := findParser{ctx: ctx, args: []string{tt.arg}}
		n, err := p.predicate(tt.name)
		if err != nil {
			t.Fatal(err)
		}
		if got := n(e); got != tt.want {
			t.Errorf("%s %s: %v", tt.name, tt.arg, got)
		}
	}
}

func TestFindIdentityAndTimes(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	ref := filepath.Join(dir, "ref")
	for _, p := range []string{file, ref} {
		if err := os.WriteFile(p, nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	if err := os.Chtimes(file, now.Add(-time.Hour), now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(ref, now.Add(-2*time.Hour), now.Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	st := findStat(info)
	if st == nil {
		t.Skip("no stat metadata")
	}
	ctx := &Context{Dir: dir}
	e := &findEntry{ctx: ctx, path: "file", full: file, info: info, now: now}
	tests := []struct {
		name, arg string
		want      bool
	}{{"-uid", fmt.Sprint(st.Uid), true}, {"-gid", fmt.Sprint(st.Gid), true}, {"-inum", fmt.Sprint(st.Ino), true}, {"-user", fmt.Sprint(st.Uid), true}, {"-group", fmt.Sprint(st.Gid), true}, {"-amin", "60", true}, {"-cmin", "0", true}, {"-newer", "ref", true}, {"-anewer", "ref", true}, {"-cnewer", "ref", true}}
	for _, tt := range tests {
		p := findParser{ctx: ctx, args: []string{tt.arg}}
		n, err := p.predicate(tt.name)
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if got := n(e); got != tt.want || e.err != nil {
			t.Errorf("%s %s: got %v err %v", tt.name, tt.arg, got, e.err)
		}
	}
}
