package sandbox

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/AmadeusDE/Misa/internal/ugh/tools"
	"github.com/AmadeusDE/Misa/internal/ugh/worktree"
)

const maxMessage = 64 << 20

// Serve answers requests from r on w until r ends or ctx is done: the helper inside a VM.
func Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s := &server{enc: json.NewEncoder(w), running: map[uint64]context.CancelFunc{}, tools: map[string]tools.Tool{}}
	for _, t := range tools.All() {
		s.tools[t.Spec().Name] = t
	}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), maxMessage)
	var wg sync.WaitGroup
	defer wg.Wait()
	for sc.Scan() {
		var req request
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			continue
		}
		if req.Op == opCancel {
			s.cancel(req.ID)
			continue
		}
		callCtx, stop := context.WithCancel(ctx)
		s.mu.Lock()
		s.running[req.ID] = stop
		s.mu.Unlock()
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer s.cancel(req.ID)
			s.send(s.handle(callCtx, req))
		}()
	}
	cancel()
	return sc.Err()
}

type server struct {
	mu      sync.Mutex
	enc     *json.Encoder
	running map[uint64]context.CancelFunc
	tools   map[string]tools.Tool
}

func (s *server) send(resp response) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.enc.Encode(resp)
}

func (s *server) cancel(id uint64) {
	s.mu.Lock()
	stop := s.running[id]
	delete(s.running, id)
	s.mu.Unlock()
	if stop != nil {
		stop()
	}
}

func (s *server) handle(ctx context.Context, req request) response {
	resp := response{ID: req.ID, Done: true}
	switch req.Op {
	case opTool:
		resp.Out, resp.BlobText, resp.Err = s.runTool(ctx, req)
	case opGit:
		var err error
		resp.Stdout, resp.Stderr, err = worktree.LocalGit(ctx, req.Dir, req.Env, req.Argv...)
		if err != nil {
			resp.Err = err.Error()
		}
	default:
		resp.Err = "unknown op " + req.Op
	}
	return resp
}

func (s *server) runTool(ctx context.Context, req request) (out, blob, errText string) {
	t, ok := s.tools[req.Name]
	if !ok {
		return "", "", "no tool " + req.Name + " in the sandbox"
	}
	if req.Scratch != "" {
		ctx = tools.WithScratch(ctx, req.Scratch)
	}
	ctx = tools.WithProgress(ctx, func(tail string) { s.send(response{ID: req.ID, Progress: tail}) })
	var blobDir string
	if req.Blob != "" {
		var err error
		if blobDir, err = os.MkdirTemp("", "ughbot-blob-"); err == nil {
			defer os.RemoveAll(blobDir)
			ctx = tools.WithBlob(ctx, tools.Blob{Dir: blobDir, Alias: req.Blob, Recall: req.Recall})
		}
	}
	out, err := t.Run(ctx, req.Cwd, req.Args)
	if err != nil {
		errText = err.Error()
	}
	if blobDir != "" {
		if b, rerr := os.ReadFile(filepath.Join(blobDir, req.Blob+".txt")); rerr == nil {
			blob = string(b)
		}
	}
	return out, blob, errText
}
