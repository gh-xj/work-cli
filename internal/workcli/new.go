package workcli

import (
	"context"
	"fmt"

	"github.com/gh-xj/work-cli/internal/work"
)

type NewCmd struct {
	Title       string   `arg:"" help:"work item title"`
	Type        string   `help:"work type id"`
	Description string   `help:"work item description"`
	Status      string   `help:"initial work status" enum:"ready,active,blocked,done,cancelled" default:"ready"`
	Priority    string   `help:"priority label"`
	Area        string   `help:"work area"`
	Labels      []string `name:"label" help:"label to attach; repeatable"`
}

func (c *NewCmd) Run(globals *CLI) error {
	store, err := globals.workStore()
	if err != nil {
		return err
	}
	item, err := store.CreateWorkItem(context.Background(), work.WorkItemInput{
		Title:       c.Title,
		Type:        c.Type,
		Description: c.Description,
		Status:      work.WorkStatus(c.Status),
		Priority:    c.Priority,
		Area:        c.Area,
		Labels:      c.Labels,
	})
	if err != nil {
		return err
	}
	out := globals.stdout()
	if globals.JSON {
		payload := map[string]any{
			"store": globals.Store,
			"item":  item,
		}
		if workspace := workspacePayload(globals.Store, item); workspace != nil {
			payload["workspace"] = workspace
		}
		return emitJSON(out, payload)
	}
	if id := fieldString(item, "ID", "Id"); id != "" {
		_, err = fmt.Fprintf(out, "created %s\n", id)
		return err
	}
	return printRecord(out, item)
}
