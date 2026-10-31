package awk

import "fmt"

// Error is a syntax or runtime error. Line is zero for runtime errors.
type Error struct {
	Line int
	Msg  string
}

func (e *Error) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
	}
	return e.Msg
}

func unsupported(line int, feature, detail string) *Error {
	return &Error{Line: line, Msg: fmt.Sprintf("%s is not supported yet (%s)", feature, detail)}
}

func runtimeErrorf(format string, args ...any) {
	panic(&Error{Msg: fmt.Sprintf(format, args...)})
}
