package workcli

import (
	"context"
	"strings"
)

type ShowCmd struct {
	ID string `arg:"" help:"work item id"`
}

func (c *ShowCmd) Run(globals *CLI) error {
	store, err := globals.workStore()
	if err != nil {
		return err
	}
	if strings.HasPrefix(c.ID, "IN-") {
		item, err := store.GetInboxItem(context.Background(), c.ID)
		if err != nil {
			return err
		}
		out := globals.stdout()
		if globals.JSON {
			return emitJSON(out, map[string]any{
				"store": globals.Store,
				"item":  item,
			})
		}
		return printRecord(out, item)
	}
	item, err := store.GetWorkItem(context.Background(), c.ID)
	if err != nil {
		return err
	}
	out := globals.stdout()
	if globals.JSON {
		return emitJSON(out, map[string]any{
			"store": globals.Store,
			"item":  item,
		})
	}
	return printRecord(out, item)
}
