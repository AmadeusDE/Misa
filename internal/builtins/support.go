package builtins

import "fmt"

// LimitKind says how a platform limit shows up at run time.
type LimitKind string

const (
	// Unsupported operations fail with exit status 1 and the Reason; they are
	// never a silent success.
	Unsupported LimitKind = "unsupported"
	// Partial operations run with narrower semantics described by the Reason.
	Partial LimitKind = "partial"
)

// Limit records one command (Option empty) or option that a platform cannot
// support in full. The commands stay registered on every platform so that a
// user gets the Reason instead of "command not found".
type Limit struct {
	Command string
	Option  string
	Kind    LimitKind
	Reason  string
}

func findLimit(table []Limit, command, option string) (Limit, bool) {
	for _, l := range table {
		if l.Command == command && l.Option == option {
			return l, true
		}
	}
	return Limit{}, false
}

// platformLimit looks up the limit of this platform; Option "" asks about the
// whole command.
func platformLimit(command, option string) (Limit, bool) {
	return findLimit(platformLimits, command, option)
}

// PlatformLimits lists what this platform cannot do in full.
func PlatformLimits() []Limit { return append([]Limit(nil), platformLimits...) }

// failUnsupported reports an unsupported command or option and returns the
// exit status.
func failUnsupported(ctx *Context, l Limit) int {
	what := l.Command
	if l.Option != "" {
		what += " " + l.Option
	}
	fmt.Fprintf(ctx.Stderr, "%s: %s is not supported on this platform: %s\n", l.Command, what, l.Reason)
	return 1
}

// refuseUnsupported fails a command that this platform cannot run at all.
func refuseUnsupported(ctx *Context, command string) (int, bool) {
	if l, ok := platformLimit(command, ""); ok && l.Kind == Unsupported {
		return failUnsupported(ctx, l), true
	}
	return 0, false
}
