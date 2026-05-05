package workcli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gh-xj/work-cli/internal/work"
)

type ClaimCmd struct {
	ID      string        `arg:"" help:"work item id"`
	Actor   string        `help:"actor id; defaults to WORK_ACTOR or USER@hostname"`
	Owner   string        `help:"alias for --actor"`
	TTL     time.Duration `help:"lease duration" default:"1h"`
	Session string        `help:"runtime session id"`
	Thread  string        `help:"runtime thread id"`
	Turn    string        `help:"runtime turn id"`
}

func (c *ClaimCmd) Run(globals *CLI) error {
	actorID, err := resolveActorID(c.Actor, c.Owner)
	if err != nil {
		return err
	}
	store, err := globals.workStore()
	if err != nil {
		return err
	}
	lease, err := store.ClaimWorkItem(context.Background(), work.ClaimWorkItemInput{
		ID: c.ID,
		Actor: work.Actor{
			ID:   actorID,
			Kind: inferCLIActorKind(actorID),
		},
		Session: claimSession(c.Session, c.Thread, c.Turn),
		TTL:     c.TTL,
	})
	if err != nil {
		return err
	}
	out := globals.stdout()
	if globals.JSON {
		return emitJSON(out, map[string]any{
			"store": globals.Store,
			"lease": lease,
		})
	}
	_, err = fmt.Fprintf(out, "claimed %s until %s\n", lease.WorkItemID, lease.ExpiresAt.Format(time.RFC3339))
	return err
}

func resolveActorID(actor, owner string) (string, error) {
	actor = strings.TrimSpace(actor)
	owner = strings.TrimSpace(owner)
	if actor != "" && owner != "" && actor != owner {
		return "", errors.New("--actor and --owner disagree")
	}
	if actor != "" {
		return actor, nil
	}
	if owner != "" {
		return owner, nil
	}
	if env := strings.TrimSpace(os.Getenv("WORK_ACTOR")); env != "" {
		return env, nil
	}
	user := strings.TrimSpace(os.Getenv("USER"))
	host, _ := os.Hostname()
	host = strings.TrimSpace(host)
	switch {
	case user != "" && host != "":
		return user + "@" + host, nil
	case user != "":
		return user, nil
	default:
		return "", errors.New("actor id is required; pass --actor or set WORK_ACTOR")
	}
}

func inferCLIActorKind(id string) work.ActorKind {
	switch {
	case strings.HasPrefix(id, "agent:"):
		return work.ActorKindAgent
	case strings.HasPrefix(id, "automation:"), strings.HasPrefix(id, "script:"):
		return work.ActorKindAutomation
	default:
		return work.ActorKindHuman
	}
}

func claimSession(id, thread, turn string) *work.Session {
	session := &work.Session{
		ID:       id,
		ThreadID: thread,
		TurnID:   turn,
	}
	if strings.TrimSpace(session.ID) == "" &&
		strings.TrimSpace(session.ThreadID) == "" &&
		strings.TrimSpace(session.TurnID) == "" {
		return nil
	}
	return session
}
