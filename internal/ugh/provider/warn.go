package provider

import (
	"fmt"
	"os"
	"sync/atomic"
)

// Process-level warnings (console providers unavailable, scope problems) go
// through one sink: stderr by default, the host's own display when ugh is
// embedded, so nothing draws over a screen ugh does not own.

var warnSink atomic.Pointer[func(string)]

// SetWarn sets where provider warnings go; nil restores stderr.
func SetWarn(f func(string)) {
	if f == nil {
		warnSink.Store(nil)
		return
	}
	warnSink.Store(&f)
}

func warn(text string) {
	if f := warnSink.Load(); f != nil {
		(*f)(text)
		return
	}
	fmt.Fprintln(os.Stderr, text)
}
