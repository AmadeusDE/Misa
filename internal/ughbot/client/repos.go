package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
)

func (c *Client) RepoGrants(ctx context.Context, bot string) ([]api.RepoGrant, error) {
	var out []api.RepoGrant
	err := c.do(ctx, http.MethodGet, "/v1/repos?bot="+url.QueryEscape(bot), nil, &out)
	return out, err
}

func (c *Client) RevokeRepo(ctx context.Context, bot, dir string) error {
	return c.do(ctx, http.MethodDelete, "/v1/repos?"+url.Values{"bot": {bot}, "dir": {dir}}.Encode(), nil, nil)
}
