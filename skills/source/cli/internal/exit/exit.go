// Package exit defines monet-source's fixed exit codes and the error that carries one.
package exit

import "fmt"

// Error ends the process with Code. main prints Msg as "monet-source: <Msg>".
type Error struct {
	Code int
	Msg  string
}

func (e *Error) Error() string { return e.Msg }

// Exit codes, fixed by the CLI contract and shared with gang.
const (
	OK       = 0
	User     = 1
	Refused  = 2
	Conflict = 3
	Held     = 4
	Usage    = 64
	Pending  = 75
)

// New returns an *Error with code and a formatted message.
func New(code int, format string, args ...any) *Error {
	return &Error{Code: code, Msg: fmt.Sprintf(format, args...)}
}
