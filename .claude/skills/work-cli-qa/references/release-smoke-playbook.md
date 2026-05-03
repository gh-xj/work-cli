# Work CLI Release Smoke Playbook

Use this before releasing the `work` binary or landing work-store
contract changes.

## Release Gate

Run from the `work-cli` repository root:

```bash
task qa -- --report /tmp/work-cli-qa.md --json
go test ./...
task verify
```

The release is not ready if any command fails.

## What The Smoke Covers

The harness builds the current source `work` binary and drives it through an
isolated temporary store. It covers:

- binary build and `version`
- store initialization with built-in `research` type installation
- direct work item creation
- inbox capture and triage acceptance
- exact lookup with `show`
- status views
- unknown work type failure behavior
- typed work creation through `work new --type`
- typed work creation through `work triage accept --type`
- one typed QA ledger work item per smoke scenario
- built-in type presets and lazy workspaces
- legacy top-level store layout rejection

## Debug Loop

On failure, the harness keeps the temp store and prints its path.

Inspect:

```bash
find <temp-store> -maxdepth 4 -print | sort
cat <temp-store>/config.yaml
find <temp-store>/items -maxdepth 1 -type f -print -exec sed -n '1,120p' {} \;
find <qa-ledger-store>/items -maxdepth 1 -type f -print | sort
find <qa-ledger-store>/spaces -maxdepth 2 -type f -print | sort
```

Then rerun with:

```bash
task qa -- --keep --verbose --report /tmp/work-cli-qa.md
```

The report is a compact handoff for the next debugging pass. It includes the
repo root, temp store, QA ledger store, duration, step outcomes, and the first
failing detail.

If a smoke scenario fails:

1. Keep the temp store.
2. Inspect the failed QA ledger item.
3. Patch the smallest failing code path.
4. Rerun `task qa -- --keep --report /tmp/work-cli-qa.md`.
5. Run `go test ./...` and `task verify`.
6. Do not claim the bug is fixed until the failing smoke item is replaced by a
   passing run.

## Update Rule

When the `work` command surface or `.work/` storage contract changes, update
all three surfaces in the same patch:

- `SKILL.md`
- `references/checks.md`
- `cli/main.go`

When a real failure or chaotic workflow exposes a gap, treat the QA skill as a
living release gate: add the scenario to the chaos playbooks, encode it in the
harness when deterministic, run it, evaluate the QA ledger, and refine until it
passes before release.
