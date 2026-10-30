//go:build !nough

package ui

import (
	"context"
	"time"

	"github.com/AmadeusDE/Misa/internal/aihelp"
)

const aiTimeout = 2 * time.Minute

func init() {
	aiFixReady = aihelp.Available
	aiFixer = func(cmd, output string, exit int) (string, string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), aiTimeout)
		defer cancel()
		h, err := aihelp.New("")
		if err != nil {
			return "", "", err
		}
		return h.Fix(ctx, cmd, output, exit)
	}
	aiAsker = func(command, question, page string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), aiTimeout)
		defer cancel()
		h, err := aihelp.New("")
		if err != nil {
			return "", err
		}
		return h.Ask(ctx, aihelp.Query{Command: command, Question: question, Context: page}, nil)
	}
}
