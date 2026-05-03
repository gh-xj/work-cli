package workcli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gh-xj/work-cli/internal/appctx"
	"github.com/gh-xj/work-cli/internal/work"
)

func runWork(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := execWriters(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestExecuteUnknownCommandReturnsUsageCode(t *testing.T) {
	code, _, _ := runWork(t, "not-a-real-command")
	if code != appctx.ExitUsage {
		t.Fatalf("expected ExitUsage on unknown command, got %d", code)
	}
}

func TestExecuteVersionCommand(t *testing.T) {
	code, stdout, _ := runWork(t, "version")
	if code != appctx.ExitSuccess {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !bytes.Contains([]byte(stdout), []byte(binaryName)) {
		t.Fatalf("expected %q in version stdout, got %q", binaryName, stdout)
	}
}

func TestExecuteInitUsesGlobalStoreAndHonorsJSON(t *testing.T) {
	fake, restore := installFakeStore(t)
	defer restore()

	code, stdout, stderr := runWork(t, "--store", "/tmp/work-store", "--json", "init")
	if code != appctx.ExitSuccess {
		t.Fatalf("expected exit 0, got %d (stderr=%q)", code, stderr)
	}
	if fake.path != "/tmp/work-store" {
		t.Fatalf("expected store path %q, got %q", "/tmp/work-store", fake.path)
	}
	if !fake.initCalled {
		t.Fatalf("expected Init to be called")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal stdout: %v (stdout=%q)", err, stdout)
	}
	if payload["schema_version"] != "v1" || payload["store"] != "/tmp/work-store" || payload["initialized"] != true {
		t.Fatalf("unexpected init JSON payload: %#v", payload)
	}
}

func TestExecuteInboxListsItems(t *testing.T) {
	fake, restore := installFakeStore(t)
	defer restore()

	code, stdout, stderr := runWork(t, "inbox")
	if code != appctx.ExitSuccess {
		t.Fatalf("expected exit 0, got %d (stderr=%q)", code, stderr)
	}
	if !fake.listInboxCalled {
		t.Fatalf("expected ListInbox to be called")
	}
	if !strings.Contains(stdout, "no inbox items") {
		t.Fatalf("expected empty inbox output, got %q", stdout)
	}
}

func TestExecuteInboxAddMapsTitle(t *testing.T) {
	fake, restore := installFakeStore(t)
	defer restore()

	code, _, stderr := runWork(t, "inbox", "add", "capture this")
	if code != appctx.ExitSuccess {
		t.Fatalf("expected exit 0, got %d (stderr=%q)", code, stderr)
	}
	if !fake.addInboxCalled {
		t.Fatalf("expected AddInboxItem to be called")
	}
	if fake.addInboxInput.Title != "capture this" {
		t.Fatalf("expected title to be mapped, got %#v", fake.addInboxInput)
	}
}

func TestExecuteTriageAcceptMapsID(t *testing.T) {
	fake, restore := installFakeStore(t)
	defer restore()

	code, _, stderr := runWork(t, "triage", "accept", "inbox-1", "--type", "research")
	if code != appctx.ExitSuccess {
		t.Fatalf("expected exit 0, got %d (stderr=%q)", code, stderr)
	}
	if !fake.acceptCalled {
		t.Fatalf("expected AcceptInboxItem to be called")
	}
	if fake.acceptInput.ID != "inbox-1" {
		t.Fatalf("expected inbox id to be mapped, got %#v", fake.acceptInput)
	}
	if fake.acceptInput.Options.Type != "research" {
		t.Fatalf("expected type to be mapped, got %#v", fake.acceptInput)
	}
}

func TestExecuteNewMapsTitle(t *testing.T) {
	fake, restore := installFakeStore(t)
	defer restore()

	code, _, stderr := runWork(t, "new", "ship v0", "--type", "research")
	if code != appctx.ExitSuccess {
		t.Fatalf("expected exit 0, got %d (stderr=%q)", code, stderr)
	}
	if !fake.createCalled {
		t.Fatalf("expected CreateWorkItem to be called")
	}
	if fake.createInput.Title != "ship v0" {
		t.Fatalf("expected title to be mapped, got %#v", fake.createInput)
	}
	if fake.createInput.Type != "research" {
		t.Fatalf("expected type to be mapped, got %#v", fake.createInput)
	}
}

func TestExecuteViewDefaultsReadyAndAcceptsName(t *testing.T) {
	fake, restore := installFakeStore(t)
	defer restore()

	code, _, stderr := runWork(t, "view")
	if code != appctx.ExitSuccess {
		t.Fatalf("expected exit 0, got %d (stderr=%q)", code, stderr)
	}
	if fake.viewName != "ready" {
		t.Fatalf("expected default view ready, got %q", fake.viewName)
	}

	fake.viewName = ""
	code, _, stderr = runWork(t, "view", "backlog")
	if code != appctx.ExitSuccess {
		t.Fatalf("expected exit 0, got %d (stderr=%q)", code, stderr)
	}
	if fake.viewName != "backlog" {
		t.Fatalf("expected named view backlog, got %q", fake.viewName)
	}
}

func TestExecuteShowMapsID(t *testing.T) {
	fake, restore := installFakeStore(t)
	defer restore()

	code, _, stderr := runWork(t, "--json", "show", "work-1")
	if code != appctx.ExitSuccess {
		t.Fatalf("expected exit 0, got %d (stderr=%q)", code, stderr)
	}
	if !fake.getCalled {
		t.Fatalf("expected GetWorkItem to be called")
	}
	if fake.getID != "work-1" {
		t.Fatalf("expected work id to be mapped, got %q", fake.getID)
	}
}

func installFakeStore(t *testing.T) (*fakeWorkStore, func()) {
	t.Helper()
	fake := &fakeWorkStore{}
	previous := openWorkStore
	openWorkStore = func(path string) (workStore, error) {
		fake.path = path
		return fake, nil
	}
	return fake, func() {
		openWorkStore = previous
	}
}

type fakeWorkStore struct {
	path string

	initCalled      bool
	listInboxCalled bool
	addInboxCalled  bool
	acceptCalled    bool
	createCalled    bool
	getCalled       bool

	addInboxInput work.InboxItemInput
	acceptInput   acceptInboxItemInput
	createInput   work.WorkItemInput
	viewName      string
	getID         string
	getInboxID    string
}

func (f *fakeWorkStore) Init(context.Context) error {
	f.initCalled = true
	return nil
}

func (f *fakeWorkStore) AddInboxItem(_ context.Context, input work.InboxItemInput) (work.InboxItem, error) {
	f.addInboxCalled = true
	f.addInboxInput = input
	return work.InboxItem{}, nil
}

func (f *fakeWorkStore) ListInbox(context.Context) ([]work.InboxItem, error) {
	f.listInboxCalled = true
	return nil, nil
}

func (f *fakeWorkStore) GetInboxItem(_ context.Context, id string) (work.InboxItem, error) {
	f.getInboxID = id
	return work.InboxItem{}, nil
}

func (f *fakeWorkStore) AcceptInboxItem(_ context.Context, input acceptInboxItemInput) (work.WorkItem, error) {
	f.acceptCalled = true
	f.acceptInput = input
	return work.WorkItem{}, nil
}

func (f *fakeWorkStore) CreateWorkItem(_ context.Context, input work.WorkItemInput) (work.WorkItem, error) {
	f.createCalled = true
	f.createInput = input
	return work.WorkItem{}, nil
}

func (f *fakeWorkStore) ListView(_ context.Context, name string) (work.ViewResult, error) {
	f.viewName = name
	return work.ViewResult{}, nil
}

func (f *fakeWorkStore) GetWorkItem(_ context.Context, id string) (work.WorkItem, error) {
	f.getCalled = true
	f.getID = id
	return work.WorkItem{}, nil
}
