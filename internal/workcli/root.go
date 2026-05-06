// Package workcli wires the `work` command-line surface.
package workcli

import (
	"io"
	"os"

	"github.com/alecthomas/kong"

	"github.com/gh-xj/work-cli/internal/cliruntime"
	"github.com/gh-xj/work-cli/internal/log"
)

const binaryName = "work"

// Build-time metadata, overridden via -ldflags by the build toolchain.
var (
	appVersion = "dev"
	appCommit  = "none"
	appDate    = "unknown"
)

type CLI struct {
	Verbose     bool             `short:"v" help:"enable debug logs"`
	NoColor     bool             `name:"no-color" help:"disable colorized output"`
	JSON        bool             `help:"emit machine-readable JSON output"`
	Store       string           `help:"path to the work store" default:".work"`
	VersionFlag kong.VersionFlag `name:"version" help:"print version and exit"`

	Init    InitCmd    `cmd:"" help:"initialize a work store"`
	Inbox   InboxCmd   `cmd:"" help:"capture and list inbox items"`
	Triage  TriageCmd  `cmd:"" help:"triage inbox items"`
	New     NewCmd     `cmd:"" help:"create a work item"`
	Claim   ClaimCmd   `cmd:"" help:"claim a work item lease"`
	Migrate MigrateCmd `cmd:"" help:"migrate older work store records"`
	View    ViewCmd    `cmd:"" help:"list a named work view"`
	Show    ShowCmd    `cmd:"" help:"show a work item"`
	Version VersionCmd `cmd:"" help:"print build metadata"`

	out io.Writer
	err io.Writer
}

func (c *CLI) stdout() io.Writer {
	if c.out != nil {
		return c.out
	}
	return os.Stdout
}

func (c *CLI) stderr() io.Writer {
	if c.err != nil {
		return c.err
	}
	return os.Stderr
}

func Execute(args []string) int {
	return execWriters(args, os.Stdout, os.Stderr)
}

func execWriters(args []string, stdout, stderr io.Writer) int {
	cli := CLI{out: stdout, err: stderr}
	return cliruntime.Execute(cliruntime.Options{
		Meta: cliruntime.Meta{
			Name:        binaryName,
			Description: "agent-native story/work CLI",
			Version:     effectiveAppVersion(),
			Commit:      appCommit,
			Date:        appDate,
		},
		Root:   &cli,
		Args:   args,
		Stdout: stdout,
		Stderr: stderr,
		BeforeRun: func() {
			log.Setup(log.Options{Verbose: cli.Verbose, NoColor: cli.NoColor, Writer: stderr})
		},
	})
}
