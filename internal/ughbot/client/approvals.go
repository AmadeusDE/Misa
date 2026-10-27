package client

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
)

// Approvals lists requests in state, or all of them for "".
func (c *Client) Approvals(ctx context.Context, state string) ([]api.ApprovalInfo, error) {
	var out []api.ApprovalInfo
	err := c.do(ctx, http.MethodGet, "/v1/approvals?state="+url.QueryEscape(state), nil, &out)
	return out, err
}

func (c *Client) Approve(ctx context.Context, id int64, by string) (api.ApprovalInfo, error) {
	return c.decide(ctx, id, "approve", by)
}

func (c *Client) Deny(ctx context.Context, id int64, by string) (api.ApprovalInfo, error) {
	return c.decide(ctx, id, "deny", by)
}

func (c *Client) decide(ctx context.Context, id int64, verb, by string) (api.ApprovalInfo, error) {
	var out api.ApprovalInfo
	err := c.do(ctx, http.MethodPost, "/v1/approvals/"+strconv.FormatInt(id, 10)+"/"+verb, api.DecideRequest{By: by}, &out)
	return out, err
}
