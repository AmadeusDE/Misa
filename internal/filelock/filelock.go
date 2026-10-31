// Package filelock provides exclusive interprocess locks on caller-owned files.
//
// Open prevents following only the final path component; parent components are
// outside this guarantee. Each contender must use a separate Open. There is no
// implicit in-process mutex: callers must serialize operations on a given file,
// including closing it, and must close it after unlocking.
//
// Unix uses advisory whole-file flock locks. Windows locks the reserved byte
// interval [1<<62, (1<<62)+1); clients must not store data in that interval.
// Lock polls with exponential backoff from 5 ms to a 100 ms cap.
package filelock

import (
	"context"
	"errors"
	"os"
	"time"
)

// ErrLocked indicates another process holds the lock.
var ErrLocked = errors.New("file lock is held")

// Lock waits for the lock or context cancellation. The caller must Unlock and
// close f when finished. Cancellation does not release an acquired lock.
func Lock(ctx context.Context, f *os.File) error {
	delay := 5 * time.Millisecond
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := TryLock(f)
		if !errors.Is(err, ErrLocked) {
			return err
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		case <-timer.C:
		}
		delay *= 2
		if delay > 100*time.Millisecond {
			delay = 100 * time.Millisecond
		}
	}
}
