package sshclient

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/x/term"
)

// prompter asks the user for answers sshc cannot know: host key trust,
// passwords and key passphrases.
type prompter interface {
	Line(prompt string) (string, error)
	Secret(prompt string) (string, error)
}

// ttyPrompter asks on the terminal sshc runs in. It reads a byte at a time so
// nothing typed after the answer is buffered away from the remote session.
type ttyPrompter struct {
	tty *os.File
	out io.Writer
}

func (p ttyPrompter) Line(prompt string) (string, error) {
	fmt.Fprint(p.out, prompt)
	var line strings.Builder
	b := make([]byte, 1)
	for {
		if _, err := p.tty.Read(b); err != nil {
			return "", err
		}
		if b[0] == '\n' || b[0] == '\r' {
			return line.String(), nil
		}
		line.WriteByte(b[0])
	}
}

func (p ttyPrompter) Secret(prompt string) (string, error) {
	fmt.Fprint(p.out, prompt)
	secret, err := term.ReadPassword(p.tty.Fd())
	fmt.Fprintln(p.out)
	return string(secret), err
}

// noPrompter answers nothing, for sessions without a terminal.
type noPrompter struct{}

var errNoTerminal = errors.New("no terminal to ask on")

func (noPrompter) Line(string) (string, error)   { return "", errNoTerminal }
func (noPrompter) Secret(string) (string, error) { return "", errNoTerminal }
