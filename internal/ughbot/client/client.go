package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
)

type Client struct {
	http *http.Client
}

// Dial checks that a daemon is listening on socketPath and returns a client for it.
func Dial(socketPath string) (*Client, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("ughbot daemon not reachable at %s: %w", socketPath, err)
	}
	conn.Close()
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socketPath)
		},
	}
	return &Client{http: &http.Client{Transport: transport}}, nil
}

func (c *Client) Close() { c.http.CloseIdleConnections() }

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://ughbot"+path, rd)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func checkStatus(resp *http.Response) error {
	if resp.StatusCode < 300 {
		return nil
	}
	var eb api.ErrorBody
	json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&eb)
	if eb.Error == "" {
		eb.Error = http.StatusText(resp.StatusCode)
	}
	return &api.StatusError{Code: resp.StatusCode, Message: eb.Error}
}

func (c *Client) ListBots(ctx context.Context) ([]api.BotInfo, error) {
	var bots []api.BotInfo
	err := c.do(ctx, http.MethodGet, "/v1/bots", nil, &bots)
	return bots, err
}

func (c *Client) Status(ctx context.Context) (api.Status, error) {
	var st api.Status
	err := c.do(ctx, http.MethodGet, "/v1/status", nil, &st)
	return st, err
}

func (c *Client) Cost(ctx context.Context, q api.CostQuery) ([]api.CostRow, error) {
	v := url.Values{}
	for k, s := range map[string]string{"bot": q.Bot, "from": q.From, "to": q.To} {
		if s != "" {
			v.Set(k, s)
		}
	}
	path := "/v1/cost"
	if len(v) > 0 {
		path += "?" + v.Encode()
	}
	var rep api.CostReport
	err := c.do(ctx, http.MethodGet, path, nil, &rep)
	return rep.Rows, err
}

func (c *Client) Send(ctx context.Context, bot, text string) (string, error) {
	var resp api.SendResponse
	err := c.do(ctx, http.MethodPost, "/v1/bots/"+url.PathEscape(bot)+"/send", api.SendRequest{Text: text}, &resp)
	return resp.Ticket, err
}

func (c *Client) Stop(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/v1/stop", nil, nil)
}

// Subscribe streams events with Seq > afterSeq. The channel closes when ctx
// ends or the connection drops; resume by passing the last Seq seen.
func (c *Client) Subscribe(ctx context.Context, bot string, afterSeq uint64) (<-chan api.Event, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://ughbot/v1/bots/"+url.PathEscape(bot)+"/events", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	if afterSeq > 0 {
		req.Header.Set("Last-Event-ID", strconv.FormatUint(afterSeq, 10))
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp); err != nil {
		resp.Body.Close()
		return nil, err
	}
	out := make(chan api.Event)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		readSSE(ctx, resp.Body, out)
	}()
	return out, nil
}

func readSSE(ctx context.Context, r io.Reader, out chan<- api.Event) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 8<<20)
	var data []string
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			if len(data) == 0 {
				continue
			}
			var ev api.Event
			err := json.Unmarshal([]byte(strings.Join(data, "\n")), &ev)
			data = data[:0]
			if err != nil {
				continue
			}
			select {
			case out <- ev:
			case <-ctx.Done():
				return
			}
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
}
