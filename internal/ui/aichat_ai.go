//go:build !nough

package ui

import (
	"github.com/AmadeusDE/Misa/internal/aichat"
	"github.com/AmadeusDE/Misa/internal/procmon"
)

func init() { newAIChat = openChat }

func openChat(args []string) (procmon.Viewer, error) {
	a, err := aichat.ParseArgs(args)
	if err != nil {
		return nil, err
	}
	chat, err := aichat.New(aichat.Options{Model: a.Model, ID: a.ID})
	if err != nil {
		return nil, err
	}
	return chat, nil
}
