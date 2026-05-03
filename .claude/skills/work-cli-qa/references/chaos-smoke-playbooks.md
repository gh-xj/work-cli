# Chaos Smoke Playbooks

Use these as concise scenario seeds before release or after any `.work` storage
change. Each scenario should become a `work-cli-qa` work item in the harness QA
ledger.

## Self-Evolution Protocol

This playbook set is expected to grow as the source `work` CLI meets new
failure modes. When a bug, regression, confusing UX path, or chaotic user
workflow appears:

1. Capture the scenario as a short playbook with goal, run path, and pass
   criteria.
2. Add or extend a deterministic harness step in `cli/main.go` when possible.
3. Run `task work:qa -- --keep --report /tmp/work-cli-qa.md --json`.
4. Evaluate the generated report and QA ledger work items.
5. Refine the implementation, harness, or playbook until the scenario passes.
6. Promote any changed contract into `checks.md` and the release smoke
   playbook.

Every deterministic scenario should leave a completed `work-cli-qa` ledger item
in the harness QA store. Do not rely on prose-only confidence when the behavior
can be tested.

## Core Lifecycle

Goal: prove the happy path still works end to end.

Run:

```bash
task work:qa -- --report /tmp/work-cli-qa.md
```

Pass criteria: all smoke items are done, the report says `PASS`, and no temp
store is kept unless requested.

Use `--keep` when you need to inspect the generated `work-cli-qa` items and
their workspaces after the run.

## Broken Type

Goal: prove user error fails cleanly without corrupting IDs.

Scenario: create work with an unknown type.

Pass criteria: command fails, no work item file is written, and the next valid
work item receives the expected ID.

## Type Removal

Goal: prove typed work is durable after its type definition is removed.

Scenario: create typed work, delete `.work/types/<type>/`, then show the work
item.

Pass criteria: `work show` still works and existing workspace files remain.

## Triage Pressure

Goal: prove raw intake remains separate from committed work.

Scenario: add inbox item, list inbox, accept with type, show inbox and work.

Pass criteria: inbox moves to `accepted`, work item records `source_inbox_id`,
and the workspace is scaffolded once.

## Failure Loop

When a scenario fails:

1. Preserve the temp store with `--keep`.
2. Read the failed QA ledger work item.
3. Patch the smallest implementation path.
4. Rerun only the harness first.
5. Run `go test ./...`.
6. Run `task verify`.
7. Add the failing command and fix note to the report before release.
