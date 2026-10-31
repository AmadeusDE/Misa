package builtins

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Location resolves TZ for this invocation without changing process state.
// It accepts zoneinfo names and absolute zoneinfo files, not POSIX TZ rules.
// Unset TZ uses time.Local; empty or unresolvable TZ uses UTC.
func (c *Context) Location() *time.Location {
	tz, ok := c.LookupEnv("TZ")
	if !ok {
		return time.Local
	}
	tz = strings.TrimPrefix(tz, ":")
	if tz == "" {
		return time.UTC
	}
	if filepath.IsAbs(tz) {
		data, err := os.ReadFile(tz)
		if err == nil {
			if loc, err := time.LoadLocationFromTZData(tz, data); err == nil {
				return loc
			}
		}
	} else if loc, err := time.LoadLocation(tz); err == nil {
		return loc
	}
	return time.UTC
}
