package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type config struct {
	repoRoot string
	keep     bool
	jsonOut  bool
	verbose  bool
	report   string
	timeout  time.Duration
}

type result struct {
	OK             bool         `json:"ok"`
	RepoRoot       string       `json:"repo_root"`
	Store          string       `json:"store"`
	LedgerStore    string       `json:"ledger_store,omitempty"`
	Kept           bool         `json:"kept"`
	Report         string       `json:"report,omitempty"`
	DurationMillis int64        `json:"duration_millis"`
	Steps          []stepResult `json:"steps"`
	Error          string       `json:"error,omitempty"`
}

type stepResult struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

type harness struct {
	cfg         config
	ctx         context.Context
	tmpRoot     string
	binPath     string
	store       string
	ledgerStore string
	ledgerReady bool
	steps       []stepResult
}

func main() {
	cfg := parseFlags()
	start := time.Now()
	res, err := run(cfg)
	res.DurationMillis = time.Since(start).Milliseconds()
	if err != nil {
		res.Error = err.Error()
	}
	if cfg.report != "" {
		reportPath, reportErr := writeReport(cfg.report, res)
		res.Report = reportPath
		if reportErr != nil {
			if err == nil {
				err = reportErr
				res.Error = reportErr.Error()
				res.OK = false
			} else {
				res.Error = res.Error + "; report: " + reportErr.Error()
			}
		}
	}

	if cfg.jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if encodeErr := enc.Encode(res); encodeErr != nil {
			fmt.Fprintf(os.Stderr, "write json: %v\n", encodeErr)
			os.Exit(1)
		}
	} else {
		printPlain(res)
	}

	if err != nil {
		os.Exit(1)
	}
}

func parseFlags() config {
	var cfg config
	flag.StringVar(&cfg.repoRoot, "repo-root", ".", "work-cli checkout root")
	flag.BoolVar(&cfg.keep, "keep", false, "keep the temporary QA store after success")
	flag.BoolVar(&cfg.jsonOut, "json", false, "emit machine-readable JSON")
	flag.BoolVar(&cfg.verbose, "verbose", false, "print command stderr on success")
	flag.StringVar(&cfg.report, "report", "", "write a markdown QA report to this path")
	flag.DurationVar(&cfg.timeout, "timeout", 2*time.Minute, "total QA timeout")
	flag.Parse()
	return cfg
}

func run(cfg config) (result, error) {
	repoRoot, err := filepath.Abs(cfg.repoRoot)
	if err != nil {
		return result{}, err
	}
	cfg.repoRoot = repoRoot

	h := &harness{cfg: cfg}
	res := result{RepoRoot: repoRoot}
	defer func() {
		res.Steps = h.steps
	}()

	if err := h.prepare(); err != nil {
		res.Store = h.store
		res.Kept = h.tmpRoot != ""
		res.Steps = h.steps
		return res, err
	}
	res.Store = h.store
	res.LedgerStore = h.ledgerStore

	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	err = h.runAll(ctx)
	if err == nil && !cfg.keep {
		if removeErr := os.RemoveAll(h.tmpRoot); removeErr != nil {
			err = fmt.Errorf("cleanup temp store: %w", removeErr)
		}
	} else {
		res.Kept = true
	}
	res.OK = err == nil
	res.Steps = h.steps
	return res, err
}

func (h *harness) prepare() error {
	if _, err := os.Stat(filepath.Join(h.cfg.repoRoot, "cmd", "work", "main.go")); err != nil {
		return fmt.Errorf("repo root %q does not look like work-cli: %w", h.cfg.repoRoot, err)
	}
	tmpRoot, err := os.MkdirTemp("", "work-cli-qa-*")
	if err != nil {
		return err
	}
	h.tmpRoot = tmpRoot
	h.binPath = filepath.Join(tmpRoot, "bin", "work")
	h.store = filepath.Join(tmpRoot, ".work")
	h.ledgerStore = filepath.Join(tmpRoot, "qa-ledger", ".work")
	return nil
}

func (h *harness) runAll(ctx context.Context) error {
	h.ctx = ctx
	if err := h.step("build source work CLI", func() error {
		if err := os.MkdirAll(filepath.Dir(h.binPath), 0o755); err != nil {
			return err
		}
		_, _, err := h.runCmd(ctx, h.cfg.repoRoot, "go", "build", "-o", h.binPath, "./cmd/work")
		return err
	}); err != nil {
		return err
	}

	if err := h.step("initialize QA lifecycle ledger", func() error {
		return h.initQALedger(ctx)
	}); err != nil {
		return err
	}
	for _, step := range h.steps {
		if err := h.recordStepWorkItem(ctx, step); err != nil {
			return fmt.Errorf("record QA work item for %q: %w", step.Name, err)
		}
	}

	if err := h.step("version emits stable JSON", func() error {
		payload, err := h.workJSON(ctx, "version")
		if err != nil {
			return err
		}
		if err := requireString(payload, "schema_version", "v1"); err != nil {
			return err
		}
		return requireString(payload, "name", "work")
	}); err != nil {
		return err
	}

	if err := h.step("init creates base store and research preset", func() error {
		payload, err := h.workJSON(ctx, "init")
		if err != nil {
			return err
		}
		if err := requireString(payload, "store", h.store); err != nil {
			return err
		}
		if err := requireBool(payload, "initialized", true); err != nil {
			return err
		}
		for _, path := range []string{
			".gitignore",
			"config.yaml",
			"inbox",
			"items",
			"types/research/type.yaml",
			"types/research/scaffold/README.md",
			"types/research/scaffold/RULES.md",
			"types/research/scaffold/notes.md",
			"types/research/scaffold/findings.md",
			"types/research/scaffold/raw/.keep",
			"types/research/scaffold/pages/.keep",
		} {
			if err := requireExists(filepath.Join(h.store, path)); err != nil {
				return err
			}
		}
		if err := requireFileContains(filepath.Join(h.store, "types/research/scaffold/README.md"), "Scaffold version: 1"); err != nil {
			return err
		}
		if err := requireAbsent(filepath.Join(h.store, "spaces")); err != nil {
			return err
		}
		return h.requireNoLegacyStorePaths()
	}); err != nil {
		return err
	}

	if err := h.step("direct work item is flat and showable", func() error {
		payload, err := h.workJSON(ctx, "new", "Direct QA work", "--description", "created by work-cli-qa", "--priority", "P2", "--area", "cli", "--label", "qa")
		if err != nil {
			return err
		}
		item, err := requireMap(payload, "item")
		if err != nil {
			return err
		}
		for _, check := range []struct {
			key  string
			want string
		}{
			{"id", "W-0001"},
			{"title", "Direct QA work"},
			{"status", "ready"},
			{"priority", "P2"},
			{"area", "cli"},
		} {
			if err := requireString(item, check.key, check.want); err != nil {
				return err
			}
		}
		if err := requireStringInArray(item, "labels", "qa"); err != nil {
			return err
		}
		if err := requireExists(filepath.Join(h.store, "items", "W-0001.yaml")); err != nil {
			return err
		}
		if err := requireAbsent(filepath.Join(h.store, "items", "W-0001")); err != nil {
			return err
		}
		return h.requireShownWork(ctx, "W-0001", "Direct QA work")
	}); err != nil {
		return err
	}

	if err := h.step("inbox item accepts into work item", func() error {
		payload, err := h.workJSON(ctx, "inbox", "add", "Captured QA idea", "--body", "raw note", "--source", "work-cli-qa")
		if err != nil {
			return err
		}
		item, err := requireMap(payload, "item")
		if err != nil {
			return err
		}
		if err := requireString(item, "id", "IN-0001"); err != nil {
			return err
		}
		if err := requireString(item, "status", "open"); err != nil {
			return err
		}

		payload, err = h.workJSON(ctx, "inbox")
		if err != nil {
			return err
		}
		if err := requireItemIDs(payload, "items", []string{"IN-0001"}); err != nil {
			return err
		}

		payload, err = h.workJSON(ctx, "triage", "accept", "IN-0001", "--priority", "P1", "--area", "docs", "--label", "triaged")
		if err != nil {
			return err
		}
		workItem, err := requireMap(payload, "item")
		if err != nil {
			return err
		}
		if err := requireString(workItem, "id", "W-0002"); err != nil {
			return err
		}
		if err := requireString(workItem, "source_inbox_id", "IN-0001"); err != nil {
			return err
		}
		if err := h.requireShownWork(ctx, "W-0002", "Captured QA idea"); err != nil {
			return err
		}

		payload, err = h.workJSON(ctx, "show", "IN-0001")
		if err != nil {
			return err
		}
		inboxItem, err := requireMap(payload, "item")
		if err != nil {
			return err
		}
		if err := requireString(inboxItem, "status", "accepted"); err != nil {
			return err
		}
		if err := requireString(inboxItem, "accepted_as", "W-0002"); err != nil {
			return err
		}

		payload, err = h.workJSON(ctx, "inbox")
		if err != nil {
			return err
		}
		return requireItemIDs(payload, "items", nil)
	}); err != nil {
		return err
	}

	if err := h.step("views filter by status", func() error {
		cases := []struct {
			title  string
			status string
			id     string
			view   string
		}{
			{"Active QA work", "active", "W-0003", "active"},
			{"Blocked QA work", "blocked", "W-0004", "blocked"},
			{"Done QA work", "done", "W-0005", "done"},
		}
		for _, tc := range cases {
			payload, err := h.workJSON(ctx, "new", tc.title, "--status", tc.status)
			if err != nil {
				return err
			}
			item, err := requireMap(payload, "item")
			if err != nil {
				return err
			}
			if err := requireString(item, "id", tc.id); err != nil {
				return err
			}
			if err := requireString(item, "status", tc.status); err != nil {
				return err
			}
			payload, err = h.workJSON(ctx, "view", tc.view)
			if err != nil {
				return err
			}
			if err := requireViewStatuses(payload, tc.status); err != nil {
				return fmt.Errorf("%s view: %w", tc.view, err)
			}
			if err := requireItemIDs(payload, "items", []string{tc.id}); err != nil {
				return fmt.Errorf("%s view: %w", tc.view, err)
			}
		}
		payload, err := h.workJSON(ctx, "view", "ready")
		if err != nil {
			return err
		}
		if err := requireViewStatuses(payload, "ready"); err != nil {
			return err
		}
		return requireItemIDs(payload, "items", []string{"W-0001", "W-0002"})
	}); err != nil {
		return err
	}

	if err := h.step("missing work type fails without allocating id", func() error {
		_, _, err := h.workRaw(ctx, "new", "Missing typed work", "--type", "missing-type")
		if err == nil {
			return errors.New("expected missing work type command to fail")
		}
		if err := requireAbsent(filepath.Join(h.store, "items", "W-0006.yaml")); err != nil {
			return err
		}
		return requireAbsent(filepath.Join(h.store, "spaces", "W-0006"))
	}); err != nil {
		return err
	}

	if err := h.step("typed work item creates workspace under spaces", func() error {
		payload, err := h.workJSON(ctx, "new", "Typed research work", "--type", "research")
		if err != nil {
			return err
		}
		item, err := requireMap(payload, "item")
		if err != nil {
			return err
		}
		if err := requireString(item, "id", "W-0006"); err != nil {
			return err
		}
		if err := requireString(item, "type", "research"); err != nil {
			return err
		}
		workspace, err := requireMap(payload, "workspace")
		if err != nil {
			return err
		}
		if err := requireString(workspace, "type", "research"); err != nil {
			return err
		}
		if err := requireBool(workspace, "scaffolded", true); err != nil {
			return err
		}
		wantPath := filepath.Join(h.store, "spaces", "W-0006")
		if err := requireString(workspace, "path", wantPath); err != nil {
			return err
		}
		for _, rel := range []string{
			"README.md",
			"RULES.md",
			"notes.md",
			"findings.md",
			"raw/.keep",
			"pages/.keep",
		} {
			if err := requireExists(filepath.Join(wantPath, rel)); err != nil {
				return err
			}
		}
		if err := requireAbsent(filepath.Join(h.store, "items", "W-0006")); err != nil {
			return err
		}
		return h.requireNoLegacyStorePaths()
	}); err != nil {
		return err
	}

	if err := h.step("typed inbox acceptance creates workspace", func() error {
		payload, err := h.workJSON(ctx, "inbox", "add", "Typed inbox research", "--body", "raw typed request")
		if err != nil {
			return err
		}
		inboxItem, err := requireMap(payload, "item")
		if err != nil {
			return err
		}
		if err := requireString(inboxItem, "id", "IN-0002"); err != nil {
			return err
		}

		payload, err = h.workJSON(ctx, "triage", "accept", "IN-0002", "--type", "research", "--priority", "P2")
		if err != nil {
			return err
		}
		workItem, err := requireMap(payload, "item")
		if err != nil {
			return err
		}
		if err := requireString(workItem, "id", "W-0007"); err != nil {
			return err
		}
		if err := requireString(workItem, "type", "research"); err != nil {
			return err
		}
		if err := requireString(workItem, "source_inbox_id", "IN-0002"); err != nil {
			return err
		}
		workspace, err := requireMap(payload, "workspace")
		if err != nil {
			return err
		}
		if err := requireString(workspace, "type", "research"); err != nil {
			return err
		}
		return requireExists(filepath.Join(h.store, "spaces", "W-0007", "README.md"))
	}); err != nil {
		return err
	}

	if err := h.step("existing typed work survives type removal", func() error {
		if err := os.RemoveAll(filepath.Join(h.store, "types", "research")); err != nil {
			return err
		}
		if err := h.requireShownWork(ctx, "W-0006", "Typed research work"); err != nil {
			return err
		}
		return h.requireShownWork(ctx, "W-0007", "Typed inbox research")
	}); err != nil {
		return err
	}

	if err := h.step("QA lifecycle ledger records each smoke scenario", func() error {
		payload, err := h.workJSONAt(ctx, h.ledgerStore, "view", "done")
		if err != nil {
			return err
		}
		items, err := requireArray(payload, "items")
		if err != nil {
			return err
		}
		if len(items) != len(h.steps) {
			return fmt.Errorf("QA ledger has %d done items, want %d", len(items), len(h.steps))
		}
		for i, value := range items {
			item, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("items[%d] is %T, want object", i, value)
			}
			if err := requireString(item, "type", "work-cli-qa"); err != nil {
				return fmt.Errorf("items[%d]: %w", i, err)
			}
			if err := requireString(item, "status", "done"); err != nil {
				return fmt.Errorf("items[%d]: %w", i, err)
			}
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (h *harness) step(name string, fn func() error) error {
	recordAfter := h.ledgerReady
	err := fn()
	if err != nil {
		step := stepResult{Name: name, OK: false, Detail: err.Error()}
		h.steps = append(h.steps, step)
		if recordAfter {
			if recordErr := h.recordStepWorkItem(h.ctx, step); recordErr != nil {
				return fmt.Errorf("%s: %w; record QA work item: %v", name, err, recordErr)
			}
		}
		return fmt.Errorf("%s: %w", name, err)
	}
	step := stepResult{Name: name, OK: true}
	h.steps = append(h.steps, step)
	if recordAfter {
		if recordErr := h.recordStepWorkItem(h.ctx, step); recordErr != nil {
			return fmt.Errorf("record QA work item for %q: %w", name, recordErr)
		}
	}
	return nil
}

func (h *harness) runCmd(ctx context.Context, dir string, name string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return stdout.Bytes(), stderr.Bytes(), ctx.Err()
	}
	if err != nil {
		return stdout.Bytes(), stderr.Bytes(), formatCommandError(name, args, stdout.String(), stderr.String(), err)
	}
	if h.cfg.verbose && stderr.Len() > 0 {
		fmt.Fprint(os.Stderr, stderr.String())
	}
	return stdout.Bytes(), stderr.Bytes(), nil
}

func (h *harness) workJSON(ctx context.Context, args ...string) (map[string]any, error) {
	return h.workJSONAt(ctx, h.store, args...)
}

func (h *harness) workJSONAt(ctx context.Context, store string, args ...string) (map[string]any, error) {
	stdout, _, err := h.workRawAt(ctx, store, args...)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout, &payload); err != nil {
		workArgs := append([]string{"--store", store, "--json"}, args...)
		return nil, fmt.Errorf("parse JSON from %q: %w\nstdout:\n%s", strings.Join(append([]string{h.binPath}, workArgs...), " "), err, string(stdout))
	}
	if err := requireString(payload, "schema_version", "v1"); err != nil {
		return nil, err
	}
	return payload, nil
}

func (h *harness) workRaw(ctx context.Context, args ...string) ([]byte, []byte, error) {
	return h.workRawAt(ctx, h.store, args...)
}

func (h *harness) workRawAt(ctx context.Context, store string, args ...string) ([]byte, []byte, error) {
	workArgs := append([]string{"--store", store, "--json"}, args...)
	return h.runCmd(ctx, h.cfg.repoRoot, h.binPath, workArgs...)
}

func (h *harness) requireShownWork(ctx context.Context, id string, title string) error {
	payload, err := h.workJSON(ctx, "show", id)
	if err != nil {
		return err
	}
	item, err := requireMap(payload, "item")
	if err != nil {
		return err
	}
	if err := requireString(item, "id", id); err != nil {
		return err
	}
	return requireString(item, "title", title)
}

func (h *harness) initQALedger(ctx context.Context) error {
	if _, err := h.workJSONAt(ctx, h.ledgerStore, "init"); err != nil {
		return err
	}
	if err := h.installWorkCLIQAType(h.ledgerStore); err != nil {
		return err
	}
	h.ledgerReady = true
	return nil
}

func (h *harness) installWorkCLIQAType(store string) error {
	source := filepath.Join(h.cfg.repoRoot, ".claude", "skills", "work-cli-qa", "references", "work-types", "work-cli-qa")
	if err := requireExists(filepath.Join(source, "type.yaml")); err != nil {
		return fmt.Errorf("local work-cli-qa type is missing: %w", err)
	}
	dest := filepath.Join(store, "types", "work-cli-qa")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if err := copyDir(source, dest); err != nil {
		return fmt.Errorf("install local work-cli-qa type: %w", err)
	}
	return nil
}

func (h *harness) recordStepWorkItem(ctx context.Context, step stepResult) error {
	status := "done"
	stateLabel := "pass"
	description := "Scenario passed."
	if !step.OK {
		status = "blocked"
		stateLabel = "fail"
		description = "Scenario failed:\n\n" + step.Detail
	}
	payload, err := h.workJSONAt(
		ctx,
		h.ledgerStore,
		"new",
		"Smoke: "+step.Name,
		"--type",
		"work-cli-qa",
		"--status",
		status,
		"--priority",
		"P2",
		"--area",
		"qa",
		"--label",
		"work-cli-qa",
		"--label",
		"smoke",
		"--label",
		stateLabel,
		"--description",
		description,
	)
	if err != nil {
		return err
	}
	item, err := requireMap(payload, "item")
	if err != nil {
		return err
	}
	if err := requireString(item, "type", "work-cli-qa"); err != nil {
		return err
	}
	if err := requireString(item, "status", status); err != nil {
		return err
	}
	workspace, err := requireMap(payload, "workspace")
	if err != nil {
		return err
	}
	workspacePathValue, ok := workspace["path"].(string)
	if !ok || workspacePathValue == "" {
		return fmt.Errorf("workspace.path is missing or not a string")
	}
	for _, file := range []string{"RULES.md", "playbook.md", "report.md"} {
		if err := requireExists(filepath.Join(workspacePathValue, file)); err != nil {
			return err
		}
	}
	return nil
}

func (h *harness) requireNoLegacyStorePaths() error {
	for _, rel := range []string{
		"views.yaml",
		"events",
		"projects",
		"relations",
		"attachments",
	} {
		if err := requireAbsent(filepath.Join(h.store, rel)); err != nil {
			return err
		}
	}
	return nil
}

func requireExists(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("expected %s to exist: %w", path, err)
	}
	return nil
}

func requireFileContains(path string, want string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if !strings.Contains(string(content), want) {
		return fmt.Errorf("expected %s to contain %q", path, want)
	}
	return nil
}

func requireAbsent(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("expected %s to be absent", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	return nil
}

func copyDir(src string, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", src)
	}
	if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported type scaffold entry %s", srcPath)
		}
		if err := copyFile(srcPath, dstPath, info.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src string, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func requireMap(payload map[string]any, key string) (map[string]any, error) {
	value, ok := payload[key]
	if !ok {
		return nil, fmt.Errorf("missing %q", key)
	}
	m, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%q is %T, want object", key, value)
	}
	return m, nil
}

func requireString(payload map[string]any, key string, want string) error {
	value, ok := payload[key]
	if !ok {
		return fmt.Errorf("missing %q", key)
	}
	got, ok := value.(string)
	if !ok {
		return fmt.Errorf("%q is %T, want string %q", key, value, want)
	}
	if got != want {
		return fmt.Errorf("%q = %q, want %q", key, got, want)
	}
	return nil
}

func requireBool(payload map[string]any, key string, want bool) error {
	value, ok := payload[key]
	if !ok {
		return fmt.Errorf("missing %q", key)
	}
	got, ok := value.(bool)
	if !ok {
		return fmt.Errorf("%q is %T, want bool %v", key, value, want)
	}
	if got != want {
		return fmt.Errorf("%q = %v, want %v", key, got, want)
	}
	return nil
}

func requireStringInArray(payload map[string]any, key string, want string) error {
	values, err := requireArray(payload, key)
	if err != nil {
		return err
	}
	for _, value := range values {
		if s, ok := value.(string); ok && s == want {
			return nil
		}
	}
	return fmt.Errorf("%q does not contain %q", key, want)
}

func requireArray(payload map[string]any, key string) ([]any, error) {
	value, ok := payload[key]
	if !ok {
		return nil, fmt.Errorf("missing %q", key)
	}
	values, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%q is %T, want array", key, value)
	}
	return values, nil
}

func requireItemIDs(payload map[string]any, key string, want []string) error {
	values, err := requireArray(payload, key)
	if err != nil {
		return err
	}
	got := make([]string, 0, len(values))
	for i, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("%s[%d] is %T, want object", key, i, value)
		}
		id, ok := item["id"].(string)
		if !ok {
			return fmt.Errorf("%s[%d].id is missing or not a string", key, i)
		}
		got = append(got, id)
	}
	if len(got) != len(want) {
		return fmt.Errorf("%s ids = %v, want %v", key, got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			return fmt.Errorf("%s ids = %v, want %v", key, got, want)
		}
	}
	return nil
}

func requireViewStatuses(payload map[string]any, want string) error {
	items, err := requireArray(payload, "items")
	if err != nil {
		return err
	}
	for i, value := range items {
		item, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("items[%d] is %T, want object", i, value)
		}
		status, ok := item["status"].(string)
		if !ok {
			return fmt.Errorf("items[%d].status is missing or not a string", i)
		}
		if status != want {
			return fmt.Errorf("items[%d].status = %q, want %q", i, status, want)
		}
	}
	return nil
}

func formatCommandError(name string, args []string, stdout string, stderr string, err error) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s failed: %v", name, strings.Join(args, " "), err)
	if strings.TrimSpace(stdout) != "" {
		fmt.Fprintf(&b, "\nstdout:\n%s", stdout)
	}
	if strings.TrimSpace(stderr) != "" {
		fmt.Fprintf(&b, "\nstderr:\n%s", stderr)
	}
	return errors.New(b.String())
}

func writeReport(path string, res result) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path, err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return abs, err
	}
	var b strings.Builder
	fmt.Fprintln(&b, "# Work CLI QA Report")
	fmt.Fprintln(&b)
	if res.OK {
		fmt.Fprintln(&b, "Status: PASS")
	} else {
		fmt.Fprintln(&b, "Status: FAIL")
	}
	fmt.Fprintf(&b, "Repo: `%s`\n", res.RepoRoot)
	fmt.Fprintf(&b, "Store: `%s`\n", res.Store)
	if res.LedgerStore != "" {
		fmt.Fprintf(&b, "QA Ledger: `%s`\n", res.LedgerStore)
	}
	fmt.Fprintf(&b, "Kept: `%t`\n", res.Kept)
	fmt.Fprintf(&b, "Duration: `%dms`\n", res.DurationMillis)
	if res.Error != "" {
		fmt.Fprintf(&b, "Error: `%s`\n", strings.ReplaceAll(res.Error, "`", "'"))
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Steps")
	fmt.Fprintln(&b)
	for _, step := range res.Steps {
		state := "PASS"
		if !step.OK {
			state = "FAIL"
		}
		fmt.Fprintf(&b, "- %s `%s`", state, step.Name)
		if step.Detail != "" {
			fmt.Fprintf(&b, " - %s", strings.ReplaceAll(step.Detail, "\n", " "))
		}
		fmt.Fprintln(&b)
	}
	return abs, os.WriteFile(abs, []byte(b.String()), 0o644)
}

func printPlain(res result) {
	if res.OK {
		fmt.Println("work-cli QA passed")
	} else {
		fmt.Println("work-cli QA failed")
	}
	for _, step := range res.Steps {
		state := "ok"
		if !step.OK {
			state = "fail"
		}
		if step.Detail == "" {
			fmt.Printf("[%s] %s\n", state, step.Name)
			continue
		}
		fmt.Printf("[%s] %s: %s\n", state, step.Name, step.Detail)
	}
	if res.Store != "" && res.Kept {
		fmt.Printf("temp store: %s\n", res.Store)
	}
	if res.LedgerStore != "" && res.Kept {
		fmt.Printf("QA ledger: %s\n", res.LedgerStore)
	}
	if res.Report != "" {
		fmt.Printf("report: %s\n", res.Report)
	}
	if res.Error != "" {
		fmt.Printf("error: %s\n", res.Error)
	}
}
