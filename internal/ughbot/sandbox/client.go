package sandbox

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/AmadeusDE/Misa/internal/ugh/tools"
)

// ErrLost is returned for calls the helper never answered because the stream ended.
var ErrLost = errors.New("sandbox connection lost")

// Client sends tool calls and git commands to a Serve at the other end of a stream.
type Client struct {
	mu      sync.Mutex
	enc     *json.Encoder
	next    uint64
	pending map[uint64]chan response
	prog    map[uint64]func(string)
	closed  error
	done    chan struct{}
}

// NewClient talks to a helper reading from w and writing to r.
func NewClient(r io.Reader, w io.Writer) *Client {
	c := &Client{enc: json.NewEncoder(w), pending: map[uint64]chan response{}, prog: map[uint64]func(string){}, done: make(chan struct{})}
	go c.read(r)
	return c
}

// Done is closed once the helper's stream has ended.
func (c *Client) Done() <-chan struct{} { return c.done }

func (c *Client) read(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), maxMessage)
	for sc.Scan() {
		var resp response
		if json.Unmarshal(sc.Bytes(), &resp) != nil {
			continue
		}
		c.mu.Lock()
		if resp.Progress != "" && !resp.Done {
			fn := c.prog[resp.ID]
			c.mu.Unlock()
			if fn != nil {
				fn(resp.Progress)
			}
			continue
		}
		ch := c.pending[resp.ID]
		delete(c.pending, resp.ID)
		delete(c.prog, resp.ID)
		c.mu.Unlock()
		if ch != nil {
			ch <- resp
		}
	}
	c.mu.Lock()
	c.closed = ErrLost
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	c.mu.Unlock()
	close(c.done)
}

func (c *Client) call(ctx context.Context, req request, progress func(string)) (response, error) {
	ch := make(chan response, 1)
	c.mu.Lock()
	if c.closed != nil {
		c.mu.Unlock()
		return response{}, c.closed
	}
	c.next++
	req.ID = c.next
	c.pending[req.ID] = ch
	if progress != nil {
		c.prog[req.ID] = progress
	}
	err := c.enc.Encode(req)
	c.mu.Unlock()
	if err != nil {
		return response{}, ErrLost
	}
	select {
	case resp, ok := <-ch:
		if !ok {
			return response{}, ErrLost
		}
		return resp, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, req.ID)
		delete(c.prog, req.ID)
		_ = c.enc.Encode(request{ID: req.ID, Op: opCancel})
		c.mu.Unlock()
		return response{}, ctx.Err()
	}
}

// RunTool runs a built-in tool in the sandbox; it is the code session's tools.Executor.
func (c *Client) RunTool(ctx context.Context, name, cwd string, raw json.RawMessage) (string, error) {
	blob := tools.BlobOf(ctx)
	req := request{Op: opTool, Name: name, Cwd: cwd, Args: raw, Scratch: tools.Scratch(ctx), Recall: blob.Recall}
	if blob.Dir != "" {
		req.Blob = blob.Alias
	}
	resp, err := c.call(ctx, req, tools.ProgressOf(ctx))
	if err != nil {
		return "", err
	}
	if resp.BlobText != "" && blob.Dir != "" {
		if os.MkdirAll(blob.Dir, 0o700) == nil {
			_ = os.WriteFile(filepath.Join(blob.Dir, blob.Alias+".txt"), []byte(resp.BlobText), 0o600)
		}
	}
	if resp.Err != "" {
		return resp.Out, errors.New(resp.Err)
	}
	return resp.Out, nil
}

// Git runs git in the sandbox; it is the code session's worktree.GitFunc.
func (c *Client) Git(ctx context.Context, dir string, env []string, args ...string) ([]byte, []byte, error) {
	resp, err := c.call(ctx, request{Op: opGit, Dir: dir, Env: env, Argv: args}, nil)
	if err != nil {
		return nil, nil, err
	}
	if resp.Err != "" {
		return resp.Stdout, resp.Stderr, errors.New(resp.Err)
	}
	return resp.Stdout, resp.Stderr, nil
}
