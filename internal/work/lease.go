package work

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ClaimWorkItem writes or renews a time-bounded lease for a work item.
func (s *Store) ClaimWorkItem(input ClaimWorkItemInput) (WorkLease, error) {
	id := strings.TrimSpace(input.ID)
	if !workIDPattern.MatchString(id) {
		return WorkLease{}, fmt.Errorf("invalid work item id %q", input.ID)
	}
	actor, err := normalizeActor(input.Actor)
	if err != nil {
		return WorkLease{}, err
	}
	if input.TTL <= 0 {
		return WorkLease{}, errors.New("claim ttl must be positive")
	}

	var lease WorkLease
	err = s.withMutationLock(func() error {
		if err := s.ensureInitialized(); err != nil {
			return err
		}
		if _, err := s.readWorkItem(id); err != nil {
			return err
		}

		now := s.timestamp()
		if existing, ok, err := s.readWorkLease(id); err != nil {
			return err
		} else if ok && existing.ExpiresAt.After(now) && existing.Actor.ID != actor.ID {
			return fmt.Errorf("%w: %s held by %s until %s", ErrAlreadyClaimed, id, existing.Actor.ID, existing.ExpiresAt.Format(time.RFC3339))
		}

		lease = WorkLease{
			WorkItemID: id,
			Actor:      actor,
			Session:    normalizeSession(input.Session),
			AcquiredAt: now,
			ExpiresAt:  now.Add(input.TTL),
		}
		return writeYAMLFile(s.workLeasePath(id), lease)
	})
	return lease, err
}

// GetWorkLease returns the active lease for a work item, if one exists.
func (s *Store) GetWorkLease(id string) (WorkLease, bool, error) {
	id = strings.TrimSpace(id)
	if !workIDPattern.MatchString(id) {
		return WorkLease{}, false, fmt.Errorf("invalid work item id %q", id)
	}
	if err := s.ensureInitialized(); err != nil {
		return WorkLease{}, false, err
	}
	if _, err := s.readWorkItem(id); err != nil {
		return WorkLease{}, false, err
	}
	lease, ok, err := s.readWorkLease(id)
	if err != nil || !ok {
		return WorkLease{}, false, err
	}
	if !lease.ExpiresAt.After(s.timestamp()) {
		return WorkLease{}, false, nil
	}
	return lease, true, nil
}

func (s *Store) readWorkLease(id string) (WorkLease, bool, error) {
	var lease WorkLease
	path := s.workLeasePath(id)
	if err := readYAMLFile(path, &lease); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return WorkLease{}, false, nil
		}
		return WorkLease{}, false, fmt.Errorf("read work lease %s: %w", path, err)
	}
	if strings.TrimSpace(lease.WorkItemID) == "" {
		lease.WorkItemID = id
	}
	return lease, true, nil
}

func normalizeActor(actor Actor) (Actor, error) {
	actor.ID = strings.TrimSpace(actor.ID)
	actor.Kind = ActorKind(strings.TrimSpace(string(actor.Kind)))
	actor.Label = strings.TrimSpace(actor.Label)
	actor.Runtime = strings.TrimSpace(actor.Runtime)
	actor.Model = strings.TrimSpace(actor.Model)

	if actor.ID == "" {
		return Actor{}, errors.New("actor id is required")
	}
	if actor.Kind == "" {
		actor.Kind = inferActorKind(actor.ID)
	}
	switch actor.Kind {
	case ActorKindHuman, ActorKindAgent, ActorKindAutomation:
	default:
		return Actor{}, fmt.Errorf("invalid actor kind %q", actor.Kind)
	}
	return actor, nil
}

func inferActorKind(id string) ActorKind {
	switch {
	case strings.HasPrefix(id, "agent:"):
		return ActorKindAgent
	case strings.HasPrefix(id, "automation:"), strings.HasPrefix(id, "script:"):
		return ActorKindAutomation
	default:
		return ActorKindHuman
	}
}

func normalizeSession(session *Session) *Session {
	if session == nil {
		return nil
	}
	out := Session{
		ID:       strings.TrimSpace(session.ID),
		ThreadID: strings.TrimSpace(session.ThreadID),
		TurnID:   strings.TrimSpace(session.TurnID),
	}
	if out.ID == "" && out.ThreadID == "" && out.TurnID == "" {
		return nil
	}
	return &out
}

func (s *Store) leasesDir() string {
	return filepath.Join(s.root, "leases")
}

func (s *Store) workLeasePath(id string) string {
	return filepath.Join(s.leasesDir(), id+".yaml")
}
