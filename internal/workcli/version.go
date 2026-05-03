package workcli

import (
	"fmt"

	appio "github.com/gh-xj/work-cli/internal/io"
)

type VersionCmd struct{}

func (c *VersionCmd) Run(globals *CLI) error {
	out := globals.stdout()
	data := map[string]string{
		"schema_version": "v1",
		"name":           binaryName,
		"version":        appVersion,
		"commit":         appCommit,
		"date":           appDate,
	}
	if globals.JSON {
		return appio.WriteJSON(out, data)
	}
	_, err := fmt.Fprintf(out, "%s %s\n", data["name"], data["version"])
	return err
}
