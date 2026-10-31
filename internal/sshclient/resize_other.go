//go:build !unix

package sshclient

// resizeEvents has no source here: Windows reports console resizes to no
// signal Go can wait on, so followSize relies on its polling alone.
func resizeEvents() (events <-chan struct{}, unsubscribe func()) {
	return nil, func() {}
}
