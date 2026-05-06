package workcli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

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

func TestExecuteVersionFlag(t *testing.T) {
	code, stdout, stderr := runWork(t, "--version")
	if code != appctx.ExitSuccess {
		t.Fatalf("expected exit 0, got %d (stderr=%q)", code, stderr)
	}
	if !strings.Contains(stdout, appVersion) {
		t.Fatalf("expected %q in --version stdout, got %q", appVersion, stdout)
	}
}

func TestExecuteVersionCheckJSON(t *testing.T) {
	previousFetch := fetchLatestRelease
	previousVersion := appVersion
	fetchLatestRelease = func(context.Context) (releaseVersion, error) {
		return releaseVersion{Version: "v0.2.0", URL: "https://github.com/gh-xj/work-cli/releases/tag/v0.2.0"}, nil
	}
	appVersion = "v0.1.0"
	t.Cleanup(func() {
		fetchLatestRelease = previousFetch
		appVersion = previousVersion
	})

	code, stdout, stderr := runWork(t, "--json", "version", "--check")
	if code != appctx.ExitSuccess {
		t.Fatalf("expected exit 0, got %d (stderr=%q)", code, stderr)
	}
	var payload struct {
		Version       string `json:"version"`
		VersionSource string `json:"version_source"`
		Latest        string `json:"latest"`
		Status        string `json:"status"`
		Outdated      *bool  `json:"outdated"`
		InstallHint   string `json:"install_hint"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal stdout: %v (stdout=%q)", err, stdout)
	}
	if payload.Version != "v0.1.0" || payload.VersionSource != versionSourceLdflags {
		t.Fatalf("unexpected version payload: %#v", payload)
	}
	if payload.Latest != "v0.2.0" || payload.Status != "outdated" || payload.Outdated == nil || !*payload.Outdated {
		t.Fatalf("expected outdated latest payload, got %#v", payload)
	}
	if payload.InstallHint != installHint {
		t.Fatalf("expected install hint %q, got %q", installHint, payload.InstallHint)
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

func TestExecuteClaimMapsLeaseInput(t *testing.T) {
	fake, restore := installFakeStore(t)
	defer restore()

	code, _, stderr := runWork(t,
		"claim", "W-0001",
		"--actor", "agent:codex:test",
		"--ttl", "30m",
		"--session", "session-1",
		"--thread", "thread-1",
		"--turn", "turn-1",
	)
	if code != appctx.ExitSuccess {
		t.Fatalf("expected exit 0, got %d (stderr=%q)", code, stderr)
	}
	if !fake.claimCalled {
		t.Fatalf("expected ClaimWorkItem to be called")
	}
	if fake.claimInput.ID != "W-0001" || fake.claimInput.Actor.ID != "agent:codex:test" || fake.claimInput.TTL != 30*time.Minute {
		t.Fatalf("unexpected claim input: %#v", fake.claimInput)
	}
	if fake.claimInput.Session == nil || fake.claimInput.Session.ThreadID != "thread-1" || fake.claimInput.Session.TurnID != "turn-1" {
		t.Fatalf("expected session fields, got %#v", fake.claimInput.Session)
	}
}

func TestExecuteMigrateMapsDryRun(t *testing.T) {
	fake, restore := installFakeStore(t)
	defer restore()

	code, stdout, stderr := runWork(t, "--json", "migrate", "--dry-run")
	if code != appctx.ExitSuccess {
		t.Fatalf("expected exit 0, got %d (stderr=%q)", code, stderr)
	}
	if !fake.migrateCalled {
		t.Fatalf("expected Migrate to be called")
	}
	if !fake.migrateInput.DryRun {
		t.Fatalf("expected dry-run migrate input, got %#v", fake.migrateInput)
	}
	var payload struct {
		Migration work.MigrationResult `json:"migration"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal stdout: %v (stdout=%q)", err, stdout)
	}
	if payload.Migration.WorkItems.Changed != 2 {
		t.Fatalf("unexpected migration payload: %#v", payload.Migration)
	}
}

func TestExecuteMigratePrintsNoop(t *testing.T) {
	fake, restore := installFakeStore(t)
	defer restore()
	fake.migrateResult = work.MigrationResult{}
	fake.migrateResultSet = true

	code, stdout, stderr := runWork(t, "migrate")
	if code != appctx.ExitSuccess {
		t.Fatalf("expected exit 0, got %d (stderr=%q)", code, stderr)
	}
	if stdout != "migration not needed\n" {
		t.Fatalf("unexpected migrate stdout: %q", stdout)
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

func TestExecuteShowPolicyPrintsTypePolicy(t *testing.T) {
	fake, restore := installFakeStore(t)
	defer restore()

	code, stdout, stderr := runWork(t, "show", "W-0001", "--policy")
	if code != appctx.ExitSuccess {
		t.Fatalf("expected exit 0, got %d (stderr=%q)", code, stderr)
	}
	if fake.getPolicyID != "W-0001" {
		t.Fatalf("expected policy lookup for W-0001, got %q", fake.getPolicyID)
	}
	if stdout != "# Policy\n" {
		t.Fatalf("unexpected policy stdout: %q", stdout)
	}
}

func TestExecuteShowPolicyJSONIncludesPolicy(t *testing.T) {
	fake, restore := installFakeStore(t)
	defer restore()

	code, stdout, stderr := runWork(t, "--json", "show", "W-0001", "--policy")
	if code != appctx.ExitSuccess {
		t.Fatalf("expected exit 0, got %d (stderr=%q)", code, stderr)
	}
	var payload struct {
		Policy work.WorkPolicy `json:"policy"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal stdout: %v (stdout=%q)", err, stdout)
	}
	if payload.Policy.WorkItemID != "W-0001" || payload.Policy.Body != "# Policy\n" {
		t.Fatalf("unexpected policy JSON: %#v", payload.Policy)
	}
	if fake.getPolicyID != "W-0001" {
		t.Fatalf("expected policy lookup for W-0001, got %q", fake.getPolicyID)
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
	claimCalled     bool
	migrateCalled   bool
	getCalled       bool

	addInboxInput    work.InboxItemInput
	acceptInput      acceptInboxItemInput
	createInput      work.WorkItemInput
	claimInput       work.ClaimWorkItemInput
	migrateInput     work.MigrateInput
	migrateResult    work.MigrationResult
	migrateResultSet bool
	viewName         string
	getID            string
	getInboxID       string
	getPolicyID      string
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

func (f *fakeWorkStore) ClaimWorkItem(_ context.Context, input work.ClaimWorkItemInput) (work.WorkLease, error) {
	f.claimCalled = true
	f.claimInput = input
	return work.WorkLease{WorkItemID: input.ID, Actor: input.Actor, Session: input.Session, ExpiresAt: time.Date(2026, 4, 29, 13, 0, 0, 0, time.UTC)}, nil
}

func (f *fakeWorkStore) Migrate(_ context.Context, input work.MigrateInput) (work.MigrationResult, error) {
	f.migrateCalled = true
	f.migrateInput = input
	if f.migrateResultSet {
		return f.migrateResult, nil
	}
	return work.MigrationResult{
		DryRun: input.DryRun,
		InboxItems: work.MigrationRecordResult{
			Scanned: 1,
			Changed: 1,
		},
		WorkItems: work.MigrationRecordResult{
			Scanned: 3,
			Changed: 2,
		},
	}, nil
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

func (f *fakeWorkStore) GetWorkLease(context.Context, string) (work.WorkLease, bool, error) {
	return work.WorkLease{}, false, nil
}

func (f *fakeWorkStore) GetWorkPolicy(_ context.Context, id string) (work.WorkPolicy, bool, error) {
	f.getPolicyID = id
	return work.WorkPolicy{
		WorkItemID: id,
		WorkType:   "research",
		Path:       "/tmp/.work/types/research/policy.md",
		Body:       "# Policy\n",
	}, true, nil
}
