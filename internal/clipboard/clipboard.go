// Package clipboard is the system clipboard behind one interface.
// Linux, BSD and macOS keep atotto/clipboard (xclip, xsel, wl-copy, pbcopy).
// Termux uses termux-clipboard-get and termux-clipboard-set when they are on
// PATH. When no host tool accepts the text, Write returns ErrUnavailable and
// the caller emits OSC 52. The native APK replaces the backend later.
package clipboard

import (
	"errors"
	"os/exec"
	"strings"

	atclip "github.com/atotto/clipboard"
)

// ErrUnavailable means no host clipboard tool accepted the request.
var ErrUnavailable = errors.New("clipboard: unavailable")

// Clipboard reads and writes the host clipboard.
type Clipboard interface {
	Read() (string, error)
	Write(string) error
}

type hostClipboard struct{}

func (hostClipboard) Read() (string, error) { return readHost() }
func (hostClipboard) Write(s string) error  { return writeHost(s) }

var current Clipboard = hostClipboard{}

// Use installs c. nil restores the host clipboard. Tests and the future APK
// JNI backend use this.
func Use(c Clipboard) {
	if c == nil {
		current = hostClipboard{}
		return
	}
	current = c
}

func Read() (string, error) { return current.Read() }
func Write(text string) error {
	return current.Write(text)
}

var writeSystem = func(text string) error { return atclip.WriteAll(text) }
var readSystem = func() (string, error) { return atclip.ReadAll() }

func readHost() (string, error) {
	if path, err := exec.LookPath("termux-clipboard-get"); err == nil {
		if text, err := termuxGet(path); err == nil {
			return text, nil
		}
	}
	if text, err := readSystem(); err == nil {
		return text, nil
	}
	return "", ErrUnavailable
}

func writeHost(text string) error {
	if path, err := exec.LookPath("termux-clipboard-set"); err == nil {
		if err := termuxSet(path, text); err == nil {
			return nil
		}
	}
	if err := writeSystem(text); err == nil {
		return nil
	}
	return ErrUnavailable
}

func termuxGet(path string) (string, error) {
	out, err := exec.Command(path).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}

func termuxSet(path, text string) error {
	cmd := exec.Command(path)
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
