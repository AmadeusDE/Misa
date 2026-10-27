package android

import (
	"sync"

	"github.com/AmadeusDE/Misa/internal/gui"
)

// memClip is the in-process selection. The system clipboard arrives with
// phase A5; until then OSC 52 writes land here and a synchronous Read
// returns them.
type memClip struct {
	mu     sync.Mutex
	text   string
	pastes chan string
}

func newMemClip() *memClip {
	return &memClip{pastes: make(chan string, 4)}
}

func (c *memClip) Read() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.text, nil
}

func (c *memClip) Write(text string) error {
	c.mu.Lock()
	c.text = text
	c.mu.Unlock()
	return nil
}

func (c *memClip) Pastes() <-chan string { return c.pastes }

var _ gui.Clipboard = (*memClip)(nil)
