//go:build !unix

package worker

import "context"

// StartProcess is unavailable without Unix descriptor passing.
func StartProcess(context.Context, ProcessOptions) (*Client, error) {
	return nil, ErrProcessUnsupported
}
