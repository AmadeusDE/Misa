//go:build windows

package auth

// Credential Manager stores at most 2560 bytes per entry and the credential
// document is one entry, so `auto` uses the protected file (size_test.go).
func init() { filePreferred = true }
