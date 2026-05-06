// Package work implements the local-first domain store for the work CLI.
package work

import "time"

const (
	// DefaultStoreDir is the repository-relative store path used by the CLI.
	DefaultStoreDir = ".work"
)

const (
	// CurrentRecordSchemaVersion is the durable YAML schema version for inbox
	// and work item records.
	CurrentRecordSchemaVersion = 1
)

type InboxStatus string

const (
	InboxStatusOpen     InboxStatus = "open"
	InboxStatusAccepted InboxStatus = "accepted"
)

type WorkStatus string

const (
	WorkStatusReady     WorkStatus = "ready"
	WorkStatusActive    WorkStatus = "active"
	WorkStatusBlocked   WorkStatus = "blocked"
	WorkStatusDone      WorkStatus = "done"
	WorkStatusCancelled WorkStatus = "cancelled"
)

type ActorKind string

const (
	ActorKindHuman      ActorKind = "human"
	ActorKindAgent      ActorKind = "agent"
	ActorKindAutomation ActorKind = "automation"
)

// InboxItem is a captured piece of untriaged work.
type InboxItem struct {
	SchemaVersion int               `yaml:"schema_version" json:"schema_version"`
	ID            string            `yaml:"id" json:"id"`
	Title         string            `yaml:"title" json:"title"`
	Body          string            `yaml:"body,omitempty" json:"body,omitempty"`
	Source        string            `yaml:"source,omitempty" json:"source,omitempty"`
	Status        InboxStatus       `yaml:"status" json:"status"`
	Labels        []string          `yaml:"labels,omitempty" json:"labels,omitempty"`
	Metadata      map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	AcceptedAs    string            `yaml:"accepted_as,omitempty" json:"accepted_as,omitempty"`
	CreatedAt     time.Time         `yaml:"created_at" json:"created_at"`
	UpdatedAt     time.Time         `yaml:"updated_at" json:"updated_at"`
	AcceptedAt    *time.Time        `yaml:"accepted_at,omitempty" json:"accepted_at,omitempty"`
}

// InboxItemInput describes an inbox item to create.
type InboxItemInput struct {
	Title    string
	Body     string
	Source   string
	Labels   []string
	Metadata map[string]string
}

// WorkItem is the durable unit tracked by the work CLI.
type WorkItem struct {
	SchemaVersion int               `yaml:"schema_version" json:"schema_version"`
	ID            string            `yaml:"id" json:"id"`
	Title         string            `yaml:"title" json:"title"`
	Type          string            `yaml:"type,omitempty" json:"type,omitempty"`
	Description   string            `yaml:"description,omitempty" json:"description,omitempty"`
	Status        WorkStatus        `yaml:"status" json:"status"`
	Priority      string            `yaml:"priority,omitempty" json:"priority,omitempty"`
	Area          string            `yaml:"area,omitempty" json:"area,omitempty"`
	Labels        []string          `yaml:"labels,omitempty" json:"labels,omitempty"`
	SourceInboxID string            `yaml:"source_inbox_id,omitempty" json:"source_inbox_id,omitempty"`
	Metadata      map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	CreatedAt     time.Time         `yaml:"created_at" json:"created_at"`
	UpdatedAt     time.Time         `yaml:"updated_at" json:"updated_at"`
}

// Actor identifies the human, agent runtime, script, or automation touching
// coordination records such as leases and attempts.
type Actor struct {
	ID      string    `yaml:"id" json:"id"`
	Kind    ActorKind `yaml:"kind" json:"kind"`
	Label   string    `yaml:"label,omitempty" json:"label,omitempty"`
	Runtime string    `yaml:"runtime,omitempty" json:"runtime,omitempty"`
	Model   string    `yaml:"model,omitempty" json:"model,omitempty"`
}

// Session records optional runtime provenance for an external agent session.
type Session struct {
	ID       string `yaml:"id,omitempty" json:"id,omitempty"`
	ThreadID string `yaml:"thread_id,omitempty" json:"thread_id,omitempty"`
	TurnID   string `yaml:"turn_id,omitempty" json:"turn_id,omitempty"`
}

// WorkLease is a time-bounded claim on a work item.
type WorkLease struct {
	WorkItemID string    `yaml:"work_item_id" json:"work_item_id"`
	Actor      Actor     `yaml:"actor" json:"actor"`
	Session    *Session  `yaml:"session,omitempty" json:"session,omitempty"`
	AcquiredAt time.Time `yaml:"acquired_at" json:"acquired_at"`
	ExpiresAt  time.Time `yaml:"expires_at" json:"expires_at"`
}

// WorkPolicy is the agent-facing policy attached to a work type.
type WorkPolicy struct {
	WorkItemID string `yaml:"work_item_id" json:"work_item_id"`
	WorkType   string `yaml:"work_type" json:"work_type"`
	Path       string `yaml:"path" json:"path"`
	Body       string `yaml:"body" json:"body"`
}

// MigrateInput controls safe, idempotent store migrations.
type MigrateInput struct {
	DryRun bool
}

// MigrationResult summarizes the store records scanned and changed.
type MigrationResult struct {
	DryRun     bool                  `json:"dry_run"`
	InboxItems MigrationRecordResult `json:"inbox_items"`
	WorkItems  MigrationRecordResult `json:"work_items"`
}

// MigrationRecordResult summarizes one migrated record class.
type MigrationRecordResult struct {
	Scanned int `json:"scanned"`
	Changed int `json:"changed"`
}

// Changed returns the total records changed, or that would change in dry-run.
func (r MigrationResult) Changed() int {
	return r.InboxItems.Changed + r.WorkItems.Changed
}

// WorkItemInput describes a work item to create.
type WorkItemInput struct {
	Title         string
	Type          string
	Description   string
	Status        WorkStatus
	Priority      string
	Area          string
	Labels        []string
	SourceInboxID string
	Metadata      map[string]string
}

// ClaimWorkItemInput describes a lease claim request.
type ClaimWorkItemInput struct {
	ID      string
	Actor   Actor
	Session *Session
	TTL     time.Duration
}

// AcceptInboxOptions controls how an inbox item becomes a work item.
type AcceptInboxOptions struct {
	Title       string
	Type        string
	Description string
	Status      WorkStatus
	Priority    string
	Area        string
	Labels      []string
	Metadata    map[string]string
}

// WorkItemFilter is used by ListWorkItems and View definitions.
type WorkItemFilter struct {
	IDs      []string     `yaml:"ids,omitempty" json:"ids,omitempty"`
	Statuses []WorkStatus `yaml:"statuses,omitempty" json:"statuses,omitempty"`
	Areas    []string     `yaml:"areas,omitempty" json:"areas,omitempty"`
	Labels   []string     `yaml:"labels,omitempty" json:"labels,omitempty"`
	Text     string       `yaml:"text,omitempty" json:"text,omitempty"`
}

// View is a named saved filter over work items.
type View struct {
	ID          string         `yaml:"id" json:"id"`
	Name        string         `yaml:"name" json:"name"`
	Description string         `yaml:"description,omitempty" json:"description,omitempty"`
	Filter      WorkItemFilter `yaml:"filter,omitempty" json:"filter,omitempty"`
}

// ViewResult is the materialized item list for a saved view.
type ViewResult struct {
	View  View       `json:"view"`
	Items []WorkItem `json:"items"`
}
