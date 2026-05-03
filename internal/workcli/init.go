package workcli

import (
	"context"
	"fmt"
)

type InitCmd struct{}

func (c *InitCmd) Run(globals *CLI) error {
	store, err := globals.workStore()
	if err != nil {
		return err
	}
	if err := store.Init(context.Background()); err != nil {
		return err
	}
	out := globals.stdout()
	if globals.JSON {
		return emitJSON(out, map[string]any{
			"store":       globals.Store,
			"initialized": true,
		})
	}
	_, err = fmt.Fprintf(out, "initialized %s\n", globals.Store)
	return err
}
