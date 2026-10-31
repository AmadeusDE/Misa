//go:build !windows

package sysinfo

func windowsFetch(string) (Info, bool)           { return Info{}, false }
func windowsFetchExtended() (ExtendedInfo, bool) { return ExtendedInfo{}, false }
