package webui

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
)

// Client is the non-browser side (CLI, Misa pane): it logs in by signing the
// server challenge and adds the CSRF header to every request.
type Client struct {
	BaseURL  string
	DeviceID string // enrolled device id, or AuthorizedKeyID(pub) for an authorized_keys login
	Key      ed25519.PrivateKey
	HTTP     *http.Client

	csrf string
}

func NewClient(baseURL string, key ed25519.PrivateKey) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	pub, _ := key.Public().(ed25519.PublicKey)
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), Key: key, DeviceID: AuthorizedKeyID(pub),
		HTTP: &http.Client{Jar: jar}}, nil
}

func (c *Client) postJSON(ctx context.Context, path string, body, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.doJSON(req, out)
}

func (c *Client) doJSON(req *http.Request, out any) error {
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusOK {
		var e struct{ Error string }
		_ = json.Unmarshal(data, &e)
		if e.Error == "" {
			e.Error = res.Status
		}
		return fmt.Errorf("webui: %s %s: %s", req.Method, req.URL.Path, e.Error)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}

// Enroll registers Key's public half and adopts the returned device id.
func (c *Client) Enroll(ctx context.Context, totpCode, name string) error {
	pub, _ := c.Key.Public().(ed25519.PublicKey)
	var out struct {
		DeviceID string `json:"device_id"`
	}
	err := c.postJSON(ctx, "/auth/enroll", enrollRequest{Code: totpCode, Name: name,
		PubKey: base64.StdEncoding.EncodeToString(pub)}, &out)
	if err == nil {
		c.DeviceID = out.DeviceID
	}
	return err
}

func (c *Client) Login(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/auth/challenge", nil)
	if err != nil {
		return err
	}
	var ch struct {
		Nonce string `json:"nonce"`
	}
	if err := c.doJSON(req, &ch); err != nil {
		return err
	}
	var out struct {
		CSRF string `json:"csrf_token"`
	}
	err = c.postJSON(ctx, "/auth/login", loginRequest{DeviceID: c.DeviceID, Nonce: ch.Nonce,
		Signature: base64.StdEncoding.EncodeToString(SignChallenge(c.Key, ch.Nonce))}, &out)
	if err != nil {
		return err
	}
	if out.CSRF == "" {
		return errors.New("webui: login returned no csrf token")
	}
	c.csrf = out.CSRF
	return nil
}

// Do sends req with the session cookie and CSRF header, logging in first when
// there is no session yet.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if c.csrf == "" {
		if err := c.Login(req.Context()); err != nil {
			return nil, err
		}
	}
	req.Header.Set(csrfHeader, c.csrf)
	return c.HTTP.Do(req)
}
