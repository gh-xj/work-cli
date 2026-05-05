package workcli

import (
	"context"

	"github.com/gh-xj/work-cli/internal/work"
)

type ViewCmd struct {
	Name string `arg:"" optional:"" default:"ready" help:"view name"`
}

func (c *ViewCmd) Run(globals *CLI) error {
	store, err := globals.workStore()
	if err != nil {
		return err
	}
	result, err := store.ListView(context.Background(), c.Name)
	if err != nil {
		return err
	}
	out := globals.stdout()
	if globals.JSON {
		payload := map[string]any{
			"store": globals.Store,
			"view":  result.View,
			"items": result.Items,
		}
		if leases, err := activeLeases(context.Background(), store, result.Items); err != nil {
			return err
		} else if len(leases) > 0 {
			payload["leases"] = leases
		}
		return emitJSON(out, payload)
	}
	return printRecords(out, result.Items, "no work items")
}

func activeLeases(ctx context.Context, store workStore, items []work.WorkItem) (map[string]work.WorkLease, error) {
	leases := map[string]work.WorkLease{}
	for _, item := range items {
		lease, ok, err := store.GetWorkLease(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		if ok {
			leases[item.ID] = lease
		}
	}
	return leases, nil
}
