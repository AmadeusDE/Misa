package aichat

import (
	"errors"
	"fmt"
	"strings"
)

// ErrHelp is returned by ParseArgs for -h and --help; its text is the usage.
var ErrHelp = errors.New("usage: aichat [-m provider/model] [conversation-id]")

// Args are the command line of `aichat`.
type Args struct{ Model, ID string }

// ParseArgs reads `aichat [-m provider/model] [conversation-id]`.
func ParseArgs(args []string) (Args, error) {
	var a Args
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			return a, ErrHelp
		case arg == "-m" || arg == "--model":
			if i+1 >= len(args) {
				return a, fmt.Errorf("%s needs a provider/model", arg)
			}
			i++
			a.Model = args[i]
		case strings.HasPrefix(arg, "--model="):
			a.Model = strings.TrimPrefix(arg, "--model=")
		case strings.HasPrefix(arg, "-") && arg != "-":
			return a, fmt.Errorf("unknown option %s (%w)", arg, ErrHelp)
		case a.ID != "":
			return a, fmt.Errorf("one conversation id at most (%w)", ErrHelp)
		default:
			a.ID = arg
		}
	}
	return a, nil
}
