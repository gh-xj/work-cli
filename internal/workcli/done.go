package workcli

import (
	"context"
	"fmt"

	"github.com/gh-xj/work-cli/internal/work"
)

type DoneCmd struct {
	ID       string   `arg:"" help:"work item id"`
	Summary  string   `help:"completion summary to write to the work item space"`
	Evidence []string `help:"completion evidence line; repeatable"`
}

func (c *DoneCmd) Run(globals *CLI) error {
	store, err := globals.workStore()
	if err != nil {
		return err
	}
	result, err := store.DoneWorkItem(context.Background(), work.DoneWorkItemInput{
		ID:       c.ID,
		Summary:  c.Summary,
		Evidence: c.Evidence,
	})
	if err != nil {
		return err
	}
	out := globals.stdout()
	if globals.JSON {
		return emitJSON(out, map[string]any{
			"store":  globals.Store,
			"result": result,
		})
	}
	_, err = fmt.Fprintf(out, "completed %s\n", result.Item.ID)
	return err
}
