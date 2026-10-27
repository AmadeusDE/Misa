package client

import (
	"context"
	"net/http"
	"strconv"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
)

func (c *Client) WebDevices(ctx context.Context) ([]api.WebDevice, error) {
	var out []api.WebDevice
	err := c.do(ctx, http.MethodGet, "/v1/web/devices", nil, &out)
	return out, err
}

func (c *Client) WebSessions(ctx context.Context) ([]api.WebSession, error) {
	var out []api.WebSession
	err := c.do(ctx, http.MethodGet, "/v1/web/sessions", nil, &out)
	return out, err
}

func (c *Client) WebEvents(ctx context.Context, limit int) ([]api.WebEvent, error) {
	var out []api.WebEvent
	err := c.do(ctx, http.MethodGet, "/v1/web/events?limit="+strconv.Itoa(limit), nil, &out)
	return out, err
}

func (c *Client) WebNotices(ctx context.Context) ([]api.WebNotice, error) {
	var out []api.WebNotice
	err := c.do(ctx, http.MethodGet, "/v1/web/notices", nil, &out)
	return out, err
}

func (c *Client) WebRevoke(ctx context.Context, idOrName string) error {
	return c.do(ctx, http.MethodPost, "/v1/web/devices/revoke", api.WebTarget{ID: idOrName}, nil)
}

func (c *Client) WebRename(ctx context.Context, idOrName, newName string) error {
	return c.do(ctx, http.MethodPost, "/v1/web/devices/rename", api.WebTarget{ID: idOrName, NewName: newName}, nil)
}

func (c *Client) WebKillSession(ctx context.Context, prefix string) error {
	return c.do(ctx, http.MethodPost, "/v1/web/sessions/kill", api.WebTarget{ID: prefix}, nil)
}

func (c *Client) WebDismissNotice(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodPost, "/v1/web/notices/dismiss", api.WebTarget{ID: id}, nil)
}
