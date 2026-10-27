//go:build !android || !cgo

package sysinfo

func systemProperty(string) string { return "" }
