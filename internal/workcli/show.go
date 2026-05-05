package workcli

import (
	"context"
	"fmt"
	"strings"
)

type ShowCmd struct {
	ID     string `arg:"" help:"work item id"`
	Policy bool   `help:"print the work type policy for a typed work item"`
}

func (c *ShowCmd) Run(globals *CLI) error {
	store, err := globals.workStore()
	if err != nil {
		return err
	}
	if strings.HasPrefix(c.ID, "IN-") {
		if c.Policy {
			return fmt.Errorf("inbox entry %s has no work type policy", c.ID)
		}
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
	if c.Policy {
		policy, ok, err := store.GetWorkPolicy(context.Background(), c.ID)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("work item %s has no work type policy", c.ID)
		}
		out := globals.stdout()
		if globals.JSON {
			return emitJSON(out, map[string]any{
				"store":  globals.Store,
				"policy": policy,
			})
		}
		_, err = fmt.Fprint(out, policy.Body)
		return err
	}
	item, err := store.GetWorkItem(context.Background(), c.ID)
	if err != nil {
		return err
	}
	out := globals.stdout()
	if globals.JSON {
		payload := map[string]any{
			"store": globals.Store,
			"item":  item,
		}
		if lease, ok, err := store.GetWorkLease(context.Background(), item.ID); err != nil {
			return err
		} else if ok {
			payload["lease"] = lease
		}
		return emitJSON(out, payload)
	}
	return printRecord(out, item)
}
