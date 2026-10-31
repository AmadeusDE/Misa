//go:build !linux && !darwin && !freebsd && !openbsd

package mounts

// List reports ErrUnsupported: this platform has no mount table reader here yet.
func List() ([]Mount, error) { return nil, ErrUnsupported }

// Statfs reports ErrUnsupported: this platform has no statfs implementation here yet.
func Statfs(string) (Stat, error) { return Stat{}, ErrUnsupported }
