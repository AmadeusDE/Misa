package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
)

func (c *Client) BotDef(ctx context.Context, name string) (api.BotDef, error) {
	var def api.BotDef
	err := c.do(ctx, http.MethodGet, "/v1/bots/"+url.PathEscape(name), nil, &def)
	return def, err
}

func (c *Client) CreateBot(ctx context.Context, def api.BotDef) error {
	return c.do(ctx, http.MethodPost, "/v1/bots", def, nil)
}

func (c *Client) UpdateBot(ctx context.Context, def api.BotDef) error {
	return c.do(ctx, http.MethodPut, "/v1/bots/"+url.PathEscape(def.Name), def, nil)
}

func (c *Client) DeleteBot(ctx context.Context, name string) error {
	return c.do(ctx, http.MethodDelete, "/v1/bots/"+url.PathEscape(name), nil, nil)
}
