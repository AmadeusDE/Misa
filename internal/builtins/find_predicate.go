// Predicates inspired by Toybox find.c, copyright 2014 Rob Landley,
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb (0BSD).
package builtins

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AmadeusDE/Misa/internal/posixre"
)

type findCompare func(int64) bool

func findNumber(s string) (findCompare, error) {
	orig := s
	sign := byte('=')
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		sign = s[0]
		s = s[1:]
	}
	if s == "" {
		return nil, fmt.Errorf("invalid number %q", orig)
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return nil, fmt.Errorf("invalid number %q", orig)
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid number %q", orig)
	}
	return func(v int64) bool {
		switch sign {
		case '+':
			return v > n
		case '-':
			return v < n
		default:
			return v == n
		}
	}, nil
}
func findAge(now, then time.Time, unit time.Duration) int64 {
	d := now.Sub(then)
	n := int64(d / unit)
	if d < 0 && d%unit != 0 {
		n--
	}
	return n
}

func (p *findParser) predicate(name string) (findNode, error) {
	switch name {
	case "-true":
		return func(*findEntry) bool { return true }, nil
	case "-false":
		return func(*findEntry) bool { return false }, nil
	case "-name", "-iname", "-path", "-ipath", "-wholename", "-iwholename":
		s, err := p.argument(name)
		if err != nil {
			return nil, err
		}
		insensitive := name == "-iname" || name == "-ipath" || name == "-iwholename"
		path := name != "-name" && name != "-iname"
		if insensitive {
			s = strings.ToLower(s)
		}
		if path {
			s = strings.ReplaceAll(s, "/", "\x00")
		}
		if _, err = filepath.Match(s, ""); err != nil {
			return nil, err
		}
		return func(e *findEntry) bool {
			v := e.path
			if !path {
				v = filepath.Base(v)
			}
			if insensitive {
				v = strings.ToLower(v)
			}
			if path {
				v = strings.ReplaceAll(v, "/", "\x00")
			}
			ok, _ := filepath.Match(s, v)
			return ok
		}, nil
	case "-regex", "-iregex":
		s, err := p.argument(name)
		if err != nil {
			return nil, err
		}
		re, err := posixre.Compile(s, posixre.Options{Syntax: posixre.Extended, IgnoreCase: name == "-iregex", Line: true})
		if err != nil {
			return nil, err
		}
		return func(e *findEntry) bool {
			ok, err := re.Match([]byte(e.path))
			if err != nil {
				return e.fail(err)
			}
			return ok
		}, nil
	case "-type", "-xtype":
		s, err := p.argument(name)
		if err != nil {
			return nil, err
		}
		parts := strings.Split(s, ",")
		for _, v := range parts {
			if len(v) != 1 || !strings.Contains("bcdpfls", v) {
				return nil, fmt.Errorf("invalid type %q", s)
			}
		}
		return func(e *findEntry) bool {
			info := e.info
			if name == "-xtype" {
				var err error
				if e.follow {
					info, err = os.Lstat(e.full)
				} else {
					info, err = os.Stat(e.full)
				}
				if err != nil {
					if os.IsNotExist(err) && e.info.Mode()&os.ModeSymlink != 0 {
						info = e.info
					} else {
						return e.fail(err)
					}
				}
			}
			return strings.Contains(s, string(findType(info)))
		}, nil
	case "-empty":
		return func(e *findEntry) bool {
			if !e.info.IsDir() {
				return e.info.Mode().IsRegular() && e.info.Size() == 0
			}
			f, err := os.Open(e.full)
			if err != nil {
				return e.fail(err)
			}
			defer f.Close()
			_, err = f.Readdirnames(1)
			if err == io.EOF {
				return true
			}
			if err != nil {
				return e.fail(err)
			}
			return false
		}, nil
	case "-size":
		s, err := p.argument(name)
		if err != nil {
			return nil, err
		}
		unit := int64(512)
		if len(s) > 0 {
			switch s[len(s)-1] {
			case 'b':
				unit = 512
			case 'c':
				unit = 1
			case 'w':
				unit = 2
			case 'k':
				unit = 1024
			case 'M':
				unit = 1024 * 1024
			case 'G':
				unit = 1024 * 1024 * 1024
			default:
				goto sizeParsed
			}
			s = s[:len(s)-1]
		}
	sizeParsed:
		cmp, err := findNumber(s)
		if err != nil {
			return nil, err
		}
		return func(e *findEntry) bool { n := e.info.Size(); return cmp(n/unit + boolInt(n%unit != 0)) }, nil
	case "-mtime", "-atime", "-ctime", "-mmin", "-amin", "-cmin":
		s, err := p.argument(name)
		if err != nil {
			return nil, err
		}
		cmp, err := findNumber(s)
		if err != nil {
			return nil, err
		}
		unit := 24 * time.Hour
		if strings.HasSuffix(name, "min") {
			unit = time.Minute
		}
		kind := name[1]
		return func(e *findEntry) bool { return cmp(findAge(e.now, findTime(e.info, kind), unit)) }, nil
	case "-newer", "-anewer", "-cnewer", "-samefile":
		s, err := p.argument(name)
		if err != nil {
			return nil, err
		}
		stat := os.Lstat
		if p.options.follow || p.options.followRoot {
			stat = os.Stat
		}
		info, err := stat(resolvePath(p.ctx, s))
		if err != nil {
			return nil, err
		}
		if name == "-samefile" {
			return func(e *findEntry) bool { return os.SameFile(e.info, info) }, nil
		}
		kind := byte('m')
		if name != "-newer" {
			kind = name[1]
		}
		return func(e *findEntry) bool { return findTime(e.info, kind).After(info.ModTime()) }, nil
	case "-perm":
		s, err := p.argument(name)
		if err != nil {
			return nil, err
		}
		mode, how, err := findParsePerm(s)
		if err != nil {
			return nil, err
		}
		return func(e *findEntry) bool {
			m := findMode(e.info)
			switch how {
			case '-':
				return m&mode == mode
			case '/':
				return mode == 0 || m&mode != 0
			default:
				return m == mode
			}
		}, nil
	}
	return p.findIdentityPredicate(name)
}
func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
