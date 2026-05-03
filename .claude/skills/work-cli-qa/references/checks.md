# Work CLI QA Checks

The QA harness is a black-box contract for the source `work` binary in this
checkout.

## Command Surface

The current required surface is:

```bash
work --json version
work --store <tmp> --json init
work --store <tmp> --json new "Title" ...
work --store <tmp> --json inbox add "Title" ...
work --store <tmp> --json inbox
work --store <tmp> --json triage accept IN-0001 ...
work --store <tmp> --json view ready
work --store <tmp> --json show W-0001
work --store <tmp> --json show IN-0001
```

There is no standalone `work verify` command in this contract. Store
verification is done by this QA harness plus the repo's existing Go tests.

## Storage Contract

`work init` must create the base store and built-in `research` preset:

```text
.work/
  .gitignore
  config.yaml
  inbox/
  items/
  types/research/type.yaml
  types/research/scaffold/README.md
```

Work items stay flat under `.work/items/W-NNNN.yaml`. Type-owned artifacts
belong under `.work/spaces/W-NNNN/`; workspaces are created lazily when typed
work is used.

Work type definitions use:

```text
.work/types/<type>/type.yaml
```

The harness fails if legacy top-level storage appears:

```text
.work/views.yaml
.work/events/
.work/projects/
.work/relations/
.work/attachments/
```

## Lifecycle Contract

The harness checks this sequence:

1. Build `cli/cmd/work` from the current checkout.
2. Print `version` as JSON with `schema_version: v1` and `name: work`.
3. Initialize an isolated `.work` store and confirm the built-in `research`
   type is installed with scaffold version text in its README.
4. Create a direct ready work item and show it.
5. Add an inbox item, list open inbox items, accept it, and show both the
   accepted inbox record and resulting work item.
6. Create active, blocked, and done work items and confirm named views only
   return matching statuses.
7. Confirm an unknown work type fails before allocating a work ID.
8. Create a typed `research` work item from the built-in preset and confirm
   the workspace scaffold is published under `.work/spaces/`.
9. Accept an inbox item as `--type research` and confirm a second typed
   workspace is created.
10. Remove the work type definition and confirm existing typed work items still
    show correctly.
11. Record every smoke step as a typed `work-cli-qa` work item in a separate
    QA ledger store using the tracked `references/work-types/work-cli-qa/`
    template.
12. Confirm the QA ledger has one completed `work-cli-qa` item for every smoke
    scenario completed so far.

On failure, inspect the preserved temp store and QA ledger printed by the
harness.

## Evolution Contract

This check list is a living contract. When a new deterministic failure mode or
chaotic workflow matters to release confidence, add it to the chaos playbooks,
encode it in the harness, and require a matching completed `work-cli-qa` ledger
item before release readiness is claimed.
