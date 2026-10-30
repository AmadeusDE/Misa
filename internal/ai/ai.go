// Package ai gives Misa features one way to reach the AI providers ugh is configured with.
//
// Providers, keys and the default model live in ugh's config.toml and credential store, so ugh and
// Misa always agree. Misa's own choices (which model a feature uses, the web search engine) live in ai.yaml.
package ai

import (
	"context"
	"errors"
	"strings"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ugh/perm"
	"github.com/AmadeusDE/Misa/internal/ugh/provider"
	"github.com/AmadeusDE/Misa/internal/ugh/roots"
)

// UghRoots names the ugh instance whose config and credentials are used. The zero value is the user's own
// ugh ($XDG_CONFIG_HOME/ugh); tests point XDG_CONFIG_HOME, XDG_STATE_HOME and HOME at a temp directory.
var UghRoots roots.Roots

// Client is one resolved provider/model.
type Client struct {
	Ref, Model string
	P          llm.Provider
}

// Open resolves a "provider/model" ref with ugh's provider stack.
func Open(ref string) (*Client, error) {
	p, model, err := provider.ResolveIn(UghRoots, ref)
	if err != nil {
		return nil, err
	}
	return &Client{Ref: ref, Model: model, P: p}, nil
}

// OpenFor opens the model chosen for a feature (see ModelFor).
func OpenFor(feature string) (*Client, error) {
	ref, err := ModelFor(feature)
	if err != nil {
		return nil, err
	}
	return Open(ref)
}

// Stream sends one turn, filling in Request.Model.
func (c *Client) Stream(ctx context.Context, req llm.Request) (<-chan llm.Event, error) {
	req.Model = c.Model
	ch, err := c.P.Stream(ctx, req)
	return ch, scrub(err)
}

// Complete is a one-shot completion without tools: the text, token usage and any provider error.
func (c *Client) Complete(ctx context.Context, system string, msgs []llm.Message) (string, llm.Usage, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ch, err := c.Stream(ctx, llm.Request{System: system, Messages: msgs, Foreground: true})
	if err != nil {
		return "", llm.Usage{}, err
	}
	defer drain(ch)
	var text strings.Builder
	var used llm.Usage
	for ev := range ch {
		switch ev.Type {
		case llm.EventTextDelta:
			text.WriteString(ev.TextDelta)
		case llm.EventUsage:
			if ev.Usage != nil {
				used.Add(*ev.Usage)
			}
		case llm.EventError:
			return text.String(), used, scrub(ev.Err)
		}
	}
	return text.String(), used, scrub(ctx.Err())
}

func drain(ch <-chan llm.Event) {
	go func() {
		for range ch {
		}
	}()
}

// scrub redacts secret-looking text a provider may have echoed in an error body.
func scrub(err error) error {
	if err == nil {
		return nil
	}
	if msg := perm.Scrub(err.Error()); msg != err.Error() {
		return errors.New(msg)
	}
	return err
}
