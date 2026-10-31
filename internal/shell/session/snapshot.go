package session

import (
	"slices"
	"sort"

	"github.com/AmadeusDE/Misa/internal/sh/expand"
)

// Dir returns the current logical directory.
func (s *Session) Dir() string { s.runMu.Lock(); defer s.runMu.Unlock(); return s.runner.Dir }

// Vars returns a deep copy of all known shell variables.
func (s *Session) Vars() map[string]expand.Variable {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	return s.vars()
}
func (s *Session) vars() map[string]expand.Variable {
	out := map[string]expand.Variable{}
	s.runner.Env.Each(func(n string, v expand.Variable) bool { out[n] = v; return true })
	for n, v := range s.runner.Vars {
		out[n] = v
	}
	for n, v := range out {
		v.List = slices.Clone(v.List)
		if v.Map != nil {
			m := make(map[string]string, len(v.Map))
			for k, x := range v.Map {
				m[k] = x
			}
			v.Map = m
		}
		out[n] = v
	}
	return out
}

// Env returns a snapshot of exported variables.
func (s *Session) Env() []string {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	vars := s.vars()
	out := make([]string, 0, len(vars))
	for n, v := range vars {
		if v.IsSet() && v.Exported && v.Kind == expand.String {
			out = append(out, n+"="+v.Str)
		}
	}
	sort.Strings(out)
	return out
}
