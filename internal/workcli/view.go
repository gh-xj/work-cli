package workcli

import "context"

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
		return emitJSON(out, map[string]any{
			"store": globals.Store,
			"view":  result.View,
			"items": result.Items,
		})
	}
	return printRecords(out, result.Items, "no work items")
}
