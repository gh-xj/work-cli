package workcli

import (
	"context"
	"fmt"

	"github.com/gh-xj/work-cli/internal/work"
)

type MigrateCmd struct {
	DryRun bool `help:"report migrations without writing files"`
}

func (c *MigrateCmd) Run(globals *CLI) error {
	store, err := globals.workStore()
	if err != nil {
		return err
	}
	result, err := store.Migrate(context.Background(), work.MigrateInput{DryRun: c.DryRun})
	if err != nil {
		return err
	}
	out := globals.stdout()
	if globals.JSON {
		return emitJSON(out, map[string]any{
			"store":     globals.Store,
			"migration": result,
		})
	}
	switch {
	case result.Changed() == 0:
		_, err = fmt.Fprintln(out, "migration not needed")
	case result.DryRun:
		_, err = fmt.Fprintf(out, "migration dry run: inbox_items=%d work_items=%d\n", result.InboxItems.Changed, result.WorkItems.Changed)
	default:
		_, err = fmt.Fprintf(out, "migrated: inbox_items=%d work_items=%d\n", result.InboxItems.Changed, result.WorkItems.Changed)
	}
	return err
}
