package sandbox

import (
	"context"
	"io"
)

// Local is a Sandbox whose machines are a Serve in this process: no isolation, for tests
// and for checking the protocol.
type Local struct{}

type localMachine struct {
	*Client
	stop   context.CancelFunc
	closeW io.Closer
	served chan struct{}
}

func (Local) Start(ctx context.Context, _ []string) (Machine, error) {
	reqR, reqW := io.Pipe()
	respR, respW := io.Pipe()
	serveCtx, stop := context.WithCancel(context.WithoutCancel(ctx))
	m := &localMachine{stop: stop, closeW: reqW, served: make(chan struct{})}
	go func() {
		defer close(m.served)
		_ = Serve(serveCtx, reqR, respW)
		respW.Close()
	}()
	m.Client = NewClient(respR, reqW)
	return m, nil
}

func (m *localMachine) Describe() string { return "local (no isolation)" }

func (m *localMachine) Close() error {
	m.stop()
	err := m.closeW.Close()
	<-m.served
	return err
}
