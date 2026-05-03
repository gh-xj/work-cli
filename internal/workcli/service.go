package workcli

import (
	"context"

	"github.com/gh-xj/work-cli/internal/work"
)

type workStore interface {
	Init(context.Context) error
	AddInboxItem(context.Context, work.InboxItemInput) (work.InboxItem, error)
	ListInbox(context.Context) ([]work.InboxItem, error)
	GetInboxItem(context.Context, string) (work.InboxItem, error)
	AcceptInboxItem(context.Context, acceptInboxItemInput) (work.WorkItem, error)
	CreateWorkItem(context.Context, work.WorkItemInput) (work.WorkItem, error)
	ListView(context.Context, string) (work.ViewResult, error)
	GetWorkItem(context.Context, string) (work.WorkItem, error)
}

type acceptInboxItemInput struct {
	ID      string
	Options work.AcceptInboxOptions
}

type domainStore struct {
	store *work.Store
}

var openWorkStore = func(path string) (workStore, error) {
	return domainStore{store: work.New(path)}, nil
}

func (c *CLI) workStore() (workStore, error) {
	return openWorkStore(c.Store)
}

func (s domainStore) Init(context.Context) error {
	return s.store.Init()
}

func (s domainStore) AddInboxItem(_ context.Context, input work.InboxItemInput) (work.InboxItem, error) {
	return s.store.AddInboxItem(input)
}

func (s domainStore) ListInbox(context.Context) ([]work.InboxItem, error) {
	return s.store.ListInbox()
}

func (s domainStore) GetInboxItem(_ context.Context, id string) (work.InboxItem, error) {
	return s.store.GetInboxItem(id)
}

func (s domainStore) AcceptInboxItem(_ context.Context, input acceptInboxItemInput) (work.WorkItem, error) {
	return s.store.AcceptInboxItem(input.ID, input.Options)
}

func (s domainStore) CreateWorkItem(_ context.Context, input work.WorkItemInput) (work.WorkItem, error) {
	return s.store.CreateWorkItem(input)
}

func (s domainStore) ListView(_ context.Context, name string) (work.ViewResult, error) {
	return s.store.ListView(name)
}

func (s domainStore) GetWorkItem(_ context.Context, id string) (work.WorkItem, error) {
	return s.store.GetWorkItem(id)
}
