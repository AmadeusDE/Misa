package client

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
)

func (c *Client) Routines(ctx context.Context, bot string) ([]api.RoutineInfo, error) {
	var out []api.RoutineInfo
	err := c.do(ctx, http.MethodGet, "/v1/routines?bot="+url.QueryEscape(bot), nil, &out)
	return out, err
}

func (c *Client) AddRoutine(ctx context.Context, req api.RoutineRequest) (api.RoutineInfo, error) {
	var out api.RoutineInfo
	err := c.do(ctx, http.MethodPost, "/v1/routines", req, &out)
	return out, err
}

func (c *Client) PauseRoutine(ctx context.Context, id int64, paused bool) (api.RoutineInfo, error) {
	verb := "resume"
	if paused {
		verb = "pause"
	}
	var out api.RoutineInfo
	err := c.do(ctx, http.MethodPost, "/v1/routines/"+strconv.FormatInt(id, 10)+"/"+verb, nil, &out)
	return out, err
}

func (c *Client) DeleteRoutine(ctx context.Context, id int64) error {
	return c.do(ctx, http.MethodDelete, "/v1/routines/"+strconv.FormatInt(id, 10), nil, nil)
}
