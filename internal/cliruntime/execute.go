// Package cliruntime contains the shared command-runner plumbing used by
// sibling binaries in this module.
package cliruntime

import (
	"fmt"
	"io"
	"os"

	"github.com/alecthomas/kong"

	"github.com/gh-xj/work-cli/internal/appctx"
)

type Meta struct {
	Name        string
	Description string
	Version     string
	Commit      string
	Date        string
}

type Options struct {
	Meta      Meta
	Root      any
	Args      []string
	Stdout    io.Writer
	Stderr    io.Writer
	Vars      map[string]string
	BeforeRun func()
}

func Execute(opts Options) int {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	vars := map[string]string{
		"version": opts.Meta.Version,
	}
	for key, value := range opts.Vars {
		vars[key] = value
	}

	parser, err := kong.New(opts.Root,
		kong.Name(opts.Meta.Name),
		kong.Description(opts.Meta.Description),
		kong.Writers(stdout, stderr),
		kong.Vars(vars),
	)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return appctx.ExitError
	}
	ctx, err := parser.Parse(opts.Args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", opts.Meta.Name, err)
		return appctx.ExitUsage
	}
	if opts.BeforeRun != nil {
		opts.BeforeRun()
	}
	if err := ctx.Run(opts.Root); err != nil {
		if msg := err.Error(); msg != "" {
			fmt.Fprintln(stderr, msg)
		}
		return appctx.ResolveExitCode(err)
	}
	return appctx.ExitSuccess
}
