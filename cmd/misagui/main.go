//go:build !android || !cgo

package main

// This stub keeps `go test ./...` and `go build ./...` working on targets
// that must not enable cgo. The Android library is jni.go, built with
// -buildmode=c-shared. See doc.go for the JNI class A4 binds.
func main() {}
