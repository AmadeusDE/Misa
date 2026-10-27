package builtins

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/sshclient"
)

func TestSSHCOptions(t *testing.T) {
	o, err := sshcOptions([]string{"-p", "2222", "-i", "a", "-ib", "-o", "User=x", "-l", "me", "-tF", "cfg", "box", "ls", "-la"})
	if err != nil {
		t.Fatal(err)
	}
	want := sshclient.Options{
		Destination: "box",
		Command:     []string{"ls", "-la"},
		User:        "me",
		Port:        "2222",
		Identities:  []string{"a", "b"},
		Settings:    []string{"User=x"},
		ConfigFile:  "cfg",
		ForceTTY:    true,
	}
	if !reflect.DeepEqual(o, want) {
		t.Errorf("got  %+v\nwant %+v", o, want)
	}
}

func TestSSHCNeedsDestination(t *testing.T) {
	var errOut bytes.Buffer
	code := runSSHC(&Context{Stderr: &errOut, Stdout: &errOut})
	if code != 255 || !strings.Contains(errOut.String(), "usage: sshc") {
		t.Errorf("code %d, stderr %q", code, errOut.String())
	}
}
