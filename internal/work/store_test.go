package work

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestInitCreatesLocalFirstLayout(t *testing.T) {
	store := newTestStore(t)

	if err := store.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if err := store.Init(); err != nil {
		t.Fatalf("second Init() error = %v", err)
	}

	for _, rel := range []string{
		".gitignore",
		"config.yaml",
		"inbox",
		"items",
		"types",
		"types/research/type.yaml",
		"types/research/scaffold/README.md",
		"types/research/scaffold/RULES.md",
		"types/research/scaffold/notes.md",
		"types/research/scaffold/findings.md",
		"types/research/scaffold/raw/.keep",
		"types/research/scaffold/pages/.keep",
	} {
		if _, err := os.Stat(filepath.Join(store.Root(), rel)); err != nil {
			t.Fatalf("expected %s to exist: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(store.Root(), "spaces")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected spaces to be created lazily, got %v", err)
	}
	gitignore, err := os.ReadFile(filepath.Join(store.Root(), ".gitignore"))
	if err != nil {
		t.Fatalf("read .work/.gitignore: %v", err)
	}
	if !bytes.Contains(gitignore, []byte(".lock")) || !bytes.Contains(gitignore, []byte(".*.tmp")) {
		t.Fatalf("expected .work/.gitignore to ignore lock and temp paths, got:\n%s", gitignore)
	}
	readme, err := os.ReadFile(filepath.Join(store.Root(), "types/research/scaffold/README.md"))
	if err != nil {
		t.Fatalf("read research README: %v", err)
	}
	if !bytes.Contains(readme, []byte("Scaffold version: 1")) {
		t.Fatalf("expected research README to include scaffold version, got:\n%s", readme)
	}
}

func TestInitDoesNotOverwriteExistingResearchType(t *testing.T) {
	store := newTestStore(t)
	customPath := filepath.Join(store.Root(), "types", "research", "type.yaml")
	if err := os.MkdirAll(filepath.Dir(customPath), 0o755); err != nil {
		t.Fatalf("mkdir custom research type: %v", err)
	}
	custom := []byte("schema_version: 1\nid: research\ndescription: Custom research\n")
	if err := os.WriteFile(customPath, custom, 0o644); err != nil {
		t.Fatalf("write custom research type: %v", err)
	}

	if err := store.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	got, err := os.ReadFile(customPath)
	if err != nil {
		t.Fatalf("read custom research type: %v", err)
	}
	if !bytes.Equal(got, custom) {
		t.Fatalf("expected Init not to overwrite custom research type, got:\n%s", got)
	}
}

func TestAddInboxItemAndListInbox(t *testing.T) {
	store := newInitializedTestStore(t)

	first, err := store.AddInboxItem(InboxItemInput{
		Title:    "Capture deploy issue",
		Body:     "Need to inspect the release logs.",
		Source:   "slack",
		Labels:   []string{"ops"},
		Metadata: map[string]string{"channel": "infra"},
	})
	if err != nil {
		t.Fatalf("AddInboxItem() error = %v", err)
	}
	second, err := store.AddInboxItem(InboxItemInput{Title: "Write migration notes"})
	if err != nil {
		t.Fatalf("AddInboxItem() second error = %v", err)
	}

	if first.ID != "IN-0001" || second.ID != "IN-0002" {
		t.Fatalf("unexpected inbox ids: %s %s", first.ID, second.ID)
	}
	items, err := store.ListInbox()
	if err != nil {
		t.Fatalf("ListInbox() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 inbox items, got %d", len(items))
	}
	if items[0].ID != "IN-0001" || items[1].ID != "IN-0002" {
		t.Fatalf("expected sorted inbox ids, got %#v", []string{items[0].ID, items[1].ID})
	}

	content, err := os.ReadFile(filepath.Join(store.Root(), "inbox", "IN-0001.yaml"))
	if err != nil {
		t.Fatalf("read inbox yaml: %v", err)
	}
	if !bytes.Contains(content, []byte("id: IN-0001")) || !bytes.Contains(content, []byte("source: slack")) {
		t.Fatalf("expected inbox YAML to contain stored fields, got:\n%s", content)
	}
}

func TestAcceptInboxItemCreatesWorkItemAndMarksInboxAccepted(t *testing.T) {
	store := newInitializedTestStore(t)
	inbox, err := store.AddInboxItem(InboxItemInput{
		Title:  "Triage flaky test",
		Body:   "The scaffold init test flakes on CI.",
		Labels: []string{"ci"},
	})
	if err != nil {
		t.Fatalf("AddInboxItem() error = %v", err)
	}

	item, err := store.AcceptInboxItem(inbox.ID, AcceptInboxOptions{
		Priority: "high",
		Area:     "ci",
		Labels:   []string{"test"},
		Metadata: map[string]string{"owner": "platform"},
	})
	if err != nil {
		t.Fatalf("AcceptInboxItem() error = %v", err)
	}
	if item.ID != "W-0001" {
		t.Fatalf("expected W-0001, got %s", item.ID)
	}
	if item.SourceInboxID != "IN-0001" || item.Status != WorkStatusReady || item.Area != "ci" {
		t.Fatalf("unexpected accepted work item: %#v", item)
	}
	if !containsString(item.Labels, "ci") || !containsString(item.Labels, "test") {
		t.Fatalf("expected merged labels, got %#v", item.Labels)
	}

	openInbox, err := store.ListInbox()
	if err != nil {
		t.Fatalf("ListInbox() error = %v", err)
	}
	if len(openInbox) != 0 {
		t.Fatalf("expected accepted inbox item to be hidden, got %#v", openInbox)
	}

	var accepted InboxItem
	if err := readYAMLFile(filepath.Join(store.Root(), "inbox", "IN-0001.yaml"), &accepted); err != nil {
		t.Fatalf("read accepted inbox item: %v", err)
	}
	if accepted.Status != InboxStatusAccepted || accepted.AcceptedAs != "W-0001" || accepted.AcceptedAt == nil {
		t.Fatalf("expected accepted inbox marker, got %#v", accepted)
	}

	got, err := store.GetWorkItem(item.ID)
	if err != nil {
		t.Fatalf("GetWorkItem() error = %v", err)
	}
	if got.ID != item.ID {
		t.Fatalf("expected returned work item %s, got %s", item.ID, got.ID)
	}
	if _, err := os.Stat(filepath.Join(store.Root(), "items", "W-0001.yaml")); err != nil {
		t.Fatalf("expected flat work item file: %v", err)
	}

	if _, err := store.AcceptInboxItem(inbox.ID, AcceptInboxOptions{}); !errors.Is(err, ErrAlreadyAccepted) {
		t.Fatalf("expected ErrAlreadyAccepted, got %v", err)
	}
}

func TestCreateListAndGetWorkItems(t *testing.T) {
	store := newInitializedTestStore(t)

	first, err := store.CreateWorkItem(WorkItemInput{
		Title:       "Implement parser",
		Status:      WorkStatusActive,
		Area:        "cli",
		Labels:      []string{"cli", "parser"},
		Description: "Parse the local work store.",
	})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	second, err := store.CreateWorkItem(WorkItemInput{
		Title:  "Ship docs",
		Status: WorkStatusDone,
		Area:   "docs",
		Labels: []string{"docs"},
	})
	if err != nil {
		t.Fatalf("CreateWorkItem() second error = %v", err)
	}
	if first.ID != "W-0001" || second.ID != "W-0002" {
		t.Fatalf("unexpected work ids: %s %s", first.ID, second.ID)
	}

	filtered, err := store.ListWorkItems(WorkItemFilter{
		Statuses: []WorkStatus{WorkStatusActive},
		Areas:    []string{"cli"},
		Labels:   []string{"cli"},
		Text:     "parser",
	})
	if err != nil {
		t.Fatalf("ListWorkItems() error = %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != "W-0001" {
		t.Fatalf("expected only W-0001, got %#v", filtered)
	}

	all, err := store.ListWorkItems(WorkItemFilter{})
	if err != nil {
		t.Fatalf("ListWorkItems(all) error = %v", err)
	}
	if len(all) != 2 || all[0].ID != "W-0001" || all[1].ID != "W-0002" {
		t.Fatalf("expected sorted work items, got %#v", all)
	}

	got, err := store.GetWorkItem("W-0002")
	if err != nil {
		t.Fatalf("GetWorkItem(W-0002) error = %v", err)
	}
	if got.ID != "W-0002" || got.Title != "Ship docs" {
		t.Fatalf("unexpected W-0002: %#v", got)
	}
}

func TestCreateTypedWorkItemUsesWorkTypeScaffold(t *testing.T) {
	store := newInitializedTestStore(t)

	item, err := store.CreateWorkItem(WorkItemInput{
		Title: "Understand filesystem research",
		Type:  "research",
	})
	if err != nil {
		t.Fatalf("CreateWorkItem() typed error = %v", err)
	}
	if item.ID != "W-0001" || item.Type != "research" {
		t.Fatalf("unexpected typed item: %#v", item)
	}

	for _, rel := range []string{
		"spaces/W-0001/README.md",
		"spaces/W-0001/RULES.md",
		"spaces/W-0001/notes.md",
		"spaces/W-0001/findings.md",
		"spaces/W-0001/raw/.keep",
		"spaces/W-0001/pages/.keep",
	} {
		if _, err := os.Stat(filepath.Join(store.Root(), rel)); err != nil {
			t.Fatalf("expected scaffold path %s: %v", rel, err)
		}
	}
	itemYAML, err := os.ReadFile(filepath.Join(store.Root(), "items", "W-0001.yaml"))
	if err != nil {
		t.Fatalf("read item yaml: %v", err)
	}
	if !bytes.Contains(itemYAML, []byte("type: research")) {
		t.Fatalf("expected item yaml to record work type, got:\n%s", itemYAML)
	}

	if err := os.RemoveAll(filepath.Join(store.Root(), "types", "research")); err != nil {
		t.Fatalf("remove work type: %v", err)
	}
	got, err := store.GetWorkItem("W-0001")
	if err != nil {
		t.Fatalf("GetWorkItem() should not require work type to remain installed: %v", err)
	}
	if got.Type != "research" {
		t.Fatalf("expected persisted type after work type removal, got %#v", got)
	}
}

func TestCreateTypedWorkItemFailsBeforeItemWhenWorkTypeMissing(t *testing.T) {
	store := newInitializedTestStore(t)

	if _, err := store.CreateWorkItem(WorkItemInput{
		Title: "Missing work type",
		Type:  "missing-type",
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for unknown work type, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.Root(), "items", "W-0001.yaml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no item to be created, got %v", err)
	}

	item, err := store.CreateWorkItem(WorkItemInput{Title: "Plain work"})
	if err != nil {
		t.Fatalf("CreateWorkItem() plain error = %v", err)
	}
	if item.ID != "W-0001" {
		t.Fatalf("expected failed work type lookup not to consume id, got %s", item.ID)
	}
}

func TestCreateTypedWorkItemDoesNotExposeItemWhenScaffoldFails(t *testing.T) {
	store := newInitializedTestStore(t)
	typeDir := filepath.Join(store.Root(), "types", "broken-research")
	if err := os.MkdirAll(typeDir, 0o755); err != nil {
		t.Fatalf("mkdir work type: %v", err)
	}
	manifest := []byte("schema_version: 1\nid: broken-research\nscaffold: missing-scaffold\n")
	if err := os.WriteFile(filepath.Join(typeDir, "type.yaml"), manifest, 0o644); err != nil {
		t.Fatalf("write work type manifest: %v", err)
	}

	if _, err := store.CreateWorkItem(WorkItemInput{
		Title: "Broken scaffold",
		Type:  "broken-research",
	}); err == nil {
		t.Fatalf("expected scaffold failure")
	}
	if _, err := os.Stat(filepath.Join(store.Root(), "items", "W-0001.yaml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected failed scaffold not to expose item, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.Root(), "spaces", "W-0001")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected failed scaffold not to publish workspace, got %v", err)
	}
}

func TestAcceptInboxItemWithTypeCreatesScaffold(t *testing.T) {
	store := newInitializedTestStore(t)
	inbox, err := store.AddInboxItem(InboxItemInput{Title: "Research request"})
	if err != nil {
		t.Fatalf("AddInboxItem() error = %v", err)
	}

	item, err := store.AcceptInboxItem(inbox.ID, AcceptInboxOptions{Type: "research"})
	if err != nil {
		t.Fatalf("AcceptInboxItem() typed error = %v", err)
	}
	if item.Type != "research" {
		t.Fatalf("expected typed accepted item, got %#v", item)
	}
	if _, err := os.Stat(filepath.Join(store.Root(), "spaces", item.ID, "RULES.md")); err != nil {
		t.Fatalf("expected accepted typed item workspace: %v", err)
	}
}

func TestListViewUsesSavedFilters(t *testing.T) {
	store := newInitializedTestStore(t)
	if _, err := store.CreateWorkItem(WorkItemInput{Title: "Ready item"}); err != nil {
		t.Fatalf("CreateWorkItem(ready) error = %v", err)
	}
	if _, err := store.CreateWorkItem(WorkItemInput{Title: "Done item", Status: WorkStatusDone}); err != nil {
		t.Fatalf("CreateWorkItem(done) error = %v", err)
	}
	if _, err := store.CreateWorkItem(WorkItemInput{Title: "Blocked item", Status: WorkStatusBlocked}); err != nil {
		t.Fatalf("CreateWorkItem(blocked) error = %v", err)
	}

	ready, err := store.ListView("ready")
	if err != nil {
		t.Fatalf("ListView(ready) error = %v", err)
	}
	if ready.View.ID != "ready" || len(ready.Items) != 1 || ready.Items[0].Title != "Ready item" {
		t.Fatalf("unexpected ready view result: %#v", ready)
	}

	done, err := store.ListView("Done")
	if err != nil {
		t.Fatalf("ListView(Done) error = %v", err)
	}
	if done.View.ID != "done" || len(done.Items) != 1 || done.Items[0].Title != "Done item" {
		t.Fatalf("unexpected done view result: %#v", done)
	}

	blocked, err := store.ListView("blocked")
	if err != nil {
		t.Fatalf("ListView(blocked) error = %v", err)
	}
	if blocked.View.ID != "blocked" || len(blocked.Items) != 1 || blocked.Items[0].Title != "Blocked item" {
		t.Fatalf("unexpected blocked view result: %#v", blocked)
	}

	if _, err := store.ListView("needs-evidence"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected needs-evidence to be absent until evidence rules exist, got %v", err)
	}
}

func TestConcurrentCreateWorkItemsAllocateUniqueIDs(t *testing.T) {
	store := newInitializedTestStore(t)

	const count = 12
	var wg sync.WaitGroup
	errs := make(chan error, count)
	ids := make(chan string, count)
	for range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			item, err := store.CreateWorkItem(WorkItemInput{Title: "Concurrent work"})
			if err != nil {
				errs <- err
				return
			}
			ids <- item.ID
		}()
	}
	wg.Wait()
	close(errs)
	close(ids)

	for err := range errs {
		t.Fatalf("CreateWorkItem() concurrent error = %v", err)
	}
	seen := map[string]bool{}
	for id := range ids {
		if seen[id] {
			t.Fatalf("duplicate id allocated: %s", id)
		}
		seen[id] = true
		item, err := store.GetWorkItem(id)
		if err != nil {
			t.Fatalf("GetWorkItem(%s) error = %v", id, err)
		}
		if item.ID != id {
			t.Fatalf("expected item %s, got %#v", id, item)
		}
	}
	if len(seen) != count {
		t.Fatalf("expected %d ids, got %d", count, len(seen))
	}
	if _, err := os.Stat(filepath.Join(store.Root(), ".lock")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected mutation lock to be released, got %v", err)
	}
}

func TestAcceptInboxItemRepairsOpenInboxWhenWorkAlreadyExists(t *testing.T) {
	store := newInitializedTestStore(t)
	inbox, err := store.AddInboxItem(InboxItemInput{Title: "Captured twice"})
	if err != nil {
		t.Fatalf("AddInboxItem() error = %v", err)
	}
	existing, err := store.CreateWorkItem(WorkItemInput{
		Title:         "Recovered work",
		SourceInboxID: inbox.ID,
	})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}

	accepted, err := store.AcceptInboxItem(inbox.ID, AcceptInboxOptions{})
	if err != nil {
		t.Fatalf("AcceptInboxItem() repair error = %v", err)
	}
	if accepted.ID != existing.ID {
		t.Fatalf("expected repair to return existing %s, got %s", existing.ID, accepted.ID)
	}
	open, err := store.ListInbox()
	if err != nil {
		t.Fatalf("ListInbox() error = %v", err)
	}
	if len(open) != 0 {
		t.Fatalf("expected repaired inbox to be hidden, got %#v", open)
	}
	items, err := store.ListWorkItems(WorkItemFilter{})
	if err != nil {
		t.Fatalf("ListWorkItems() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != existing.ID {
		t.Fatalf("expected no duplicate work items, got %#v", items)
	}
}

func newInitializedTestStore(t *testing.T) *Store {
	t.Helper()
	store := newTestStore(t)
	if err := store.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	return store
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store := New(filepath.Join(t.TempDir(), ".work"))
	base := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	tick := 0
	var mu sync.Mutex
	store.now = func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		tick++
		return base.Add(time.Duration(tick) * time.Second)
	}
	return store
}
