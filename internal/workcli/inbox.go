package workcli

import (
	"context"
	"fmt"

	"github.com/gh-xj/work-cli/internal/work"
)

type InboxCmd struct {
	List InboxListCmd `cmd:"" default:"1" hidden:"" help:"list inbox items"`
	Add  InboxAddCmd  `cmd:"" help:"add an inbox item"`
}

type InboxListCmd struct{}

func (c *InboxListCmd) Run(globals *CLI) error {
	store, err := globals.workStore()
	if err != nil {
		return err
	}
	items, err := store.ListInbox(context.Background())
	if err != nil {
		return err
	}
	out := globals.stdout()
	if globals.JSON {
		return emitJSON(out, map[string]any{
			"store": globals.Store,
			"items": items,
		})
	}
	return printRecords(out, items, "no inbox items")
}

type InboxAddCmd struct {
	Title  string   `arg:"" help:"inbox item title"`
	Body   string   `help:"inbox item body"`
	Source string   `help:"source reference"`
	Labels []string `name:"label" help:"label to attach; repeatable"`
}

func (c *InboxAddCmd) Run(globals *CLI) error {
	store, err := globals.workStore()
	if err != nil {
		return err
	}
	item, err := store.AddInboxItem(context.Background(), work.InboxItemInput{
		Title:  c.Title,
		Body:   c.Body,
		Source: c.Source,
		Labels: c.Labels,
	})
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
	if id := fieldString(item, "ID", "Id"); id != "" {
		_, err = fmt.Fprintf(out, "added %s\n", id)
		return err
	}
	return printRecord(out, item)
}
