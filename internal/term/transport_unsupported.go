//go:build !unix && !windows

package term

func startProcess(startSpec) (conn, proc, error) { return nil, nil, ErrUnsupported }
