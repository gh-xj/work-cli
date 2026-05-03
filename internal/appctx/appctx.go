package appctx

import (
	"context"
	"errors"
)

const (
	ExitSuccess = 0
	ExitError   = 1
	ExitUsage   = 2
)

type AppMeta struct {
	Name    string
	Version string
	Commit  string
	Date    string
}

type AppContext struct {
	Context context.Context
	Meta    AppMeta
	Values  map[string]any
}

func NewAppContext(ctx context.Context) *AppContext {
	if ctx == nil {
		ctx = context.Background()
	}
	return &AppContext{Context: ctx, Values: map[string]any{}}
}

// ExitCodeError wraps an explicit process exit code. Commands can return
// this (or anything that unwraps to it via errors.As) to override the
// default "err != nil → ExitError" mapping performed by ResolveExitCode.
type ExitCodeError struct {
	Code    int
	Message string
}

// NewExitError constructs an ExitCodeError with the given code and optional
// message. A message of "" produces a silent exit-code-only error, useful
// for commands that already emitted their own output to stdout/stderr.
func NewExitError(code int, message string) *ExitCodeError {
	return &ExitCodeError{Code: code, Message: message}
}

func (e *ExitCodeError) Error() string {
	return e.Message
}

func ResolveExitCode(err error) int {
	if err == nil {
		return ExitSuccess
	}
	var exitErr *ExitCodeError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}
	return ExitError
}
