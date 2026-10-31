// Additional find predicates. See find_predicate.go for attribution.
package builtins

import (
	"fmt"
	"os/user"
	"strconv"
	"strings"
)

func findParsePerm(s string) (uint32, byte, error) {
	original := s
	how := byte('=')
	if len(s) > 0 && (s[0] == '-' || s[0] == '/') {
		how = s[0]
		s = s[1:]
	}
	bad := func() (uint32, byte, error) { return 0, how, fmt.Errorf("invalid mode %q", original) }
	if s == "" {
		return bad()
	}
	if s[0] >= '0' && s[0] <= '9' {
		n, err := strconv.ParseUint(s, 8, 12)
		if err != nil {
			return bad()
		}
		return uint32(n), how, nil
	}
	var mode uint32
	for _, clause := range strings.Split(s, ",") {
		i := 0
		var who uint32
		for i < len(clause) {
			switch clause[i] {
			case 'u':
				who |= 04700
			case 'g':
				who |= 02070
			case 'o':
				who |= 01007
			case 'a':
				who |= 07777
			default:
				goto op
			}
			i++
		}
	op:
		if who == 0 {
			who = 07777
		}
		if i == len(clause) {
			return bad()
		}
		for i < len(clause) {
			c := clause[i]
			if c != '+' && c != '-' && c != '=' {
				return bad()
			}
			i++
			var bits uint32
			start := i
			for i < len(clause) && !strings.ContainsRune("+-=", rune(clause[i])) {
				switch clause[i] {
				case 'r':
					bits |= 0444
				case 'w':
					bits |= 0222
				case 'x':
					bits |= 0111
				case 'X':
					if mode&0111 != 0 {
						bits |= 0111
					}
				case 's':
					bits |= 06000
				case 't':
					bits |= 01000
				case 'u', 'g', 'o':
					var shift uint
					if clause[i] == 'u' {
						shift = 6
					} else if clause[i] == 'g' {
						shift = 3
					}
					v := (mode >> shift) & 7
					bits |= v | (v << 3) | (v << 6)
				default:
					return bad()
				}
				i++
			}
			if i == start && c != '=' {
				return bad()
			}
			bits &= who
			switch c {
			case '+':
				mode |= bits
			case '-':
				mode &^= bits
			case '=':
				mode = (mode &^ who) | bits
			}
		}
	}
	return mode, how, nil
}

func (p *findParser) findIdentityPredicate(name string) (findNode, error) {
	switch name {
	case "-uid", "-gid", "-links", "-inum":
		s, err := p.argument(name)
		if err != nil {
			return nil, err
		}
		cmp, err := findNumber(s)
		if err != nil {
			return nil, err
		}
		return func(e *findEntry) bool {
			st := findStat(e.info)
			if st == nil {
				return e.fail(fmt.Errorf("metadata unavailable: %s", e.path))
			}
			var n int64
			switch name {
			case "-uid":
				n = int64(st.Uid)
			case "-gid":
				n = int64(st.Gid)
			case "-links":
				n = int64(st.Nlink)
			case "-inum":
				n = int64(st.Ino)
			}
			return cmp(n)
		}, nil
	case "-user", "-group":
		s, err := p.argument(name)
		if err != nil {
			return nil, err
		}
		var id string
		if name == "-user" {
			u, e := user.Lookup(s)
			if e == nil {
				id = u.Uid
			}
		} else {
			g, e := user.LookupGroup(s)
			if e == nil {
				id = g.Gid
			}
		}
		if id == "" {
			if _, e := strconv.ParseUint(s, 10, 32); e != nil {
				return nil, fmt.Errorf("unknown %s %q", name[1:], s)
			}
			id = s
		}
		n, err := strconv.ParseUint(id, 10, 32)
		if err != nil {
			return nil, err
		}
		return func(e *findEntry) bool {
			st := findStat(e.info)
			if st == nil {
				return e.fail(fmt.Errorf("metadata unavailable: %s", e.path))
			}
			if name == "-user" {
				return uint64(st.Uid) == n
			}
			return uint64(st.Gid) == n
		}, nil
	case "-nouser", "-nogroup":
		return func(e *findEntry) bool {
			st := findStat(e.info)
			if st == nil {
				return e.fail(fmt.Errorf("metadata unavailable: %s", e.path))
			}
			if name == "-nouser" {
				_, err := user.LookupId(strconv.FormatUint(uint64(st.Uid), 10))
				return err != nil
			}
			_, err := user.LookupGroupId(strconv.FormatUint(uint64(st.Gid), 10))
			return err != nil
		}, nil
	}
	return nil, fmt.Errorf("unknown predicate %s", name)
}
