// Package log configures the default slog logger for repo CLIs.
//
// Output is leveled and colored by default (via lmittmann/tint) when
// stderr is a TTY. Color is auto-disabled when stderr is not a TTY or
// when NO_COLOR is set in the environment. The --no-color flag and the
// --verbose flag on each root command both feed into Setup().
package log

import (
	"io"
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
	"golang.org/x/term"
)

// Options controls logger initialization.
type Options struct {
	Verbose bool // raise level to Debug
	NoColor bool // explicit --no-color flag; overrides TTY check
	Writer  io.Writer
}

// Setup installs a tint-backed slog logger as the default. Call once
// near program start. Safe to call multiple times (re-initializes).
func Setup(opts Options) {
	w := opts.Writer
	if w == nil {
		w = os.Stderr
	}

	noColor := opts.NoColor || os.Getenv("NO_COLOR") != "" || !isTerminal(w)

	level := slog.LevelInfo
	if opts.Verbose {
		level = slog.LevelDebug
	}

	handler := tint.NewHandler(w, &tint.Options{
		Level:   level,
		NoColor: noColor,
		// Strip the timestamp — CLI output shouldn't be noisy, and
		// redirected logs can rely on the caller's timestamp.
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})
	slog.SetDefault(slog.New(handler))
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}
