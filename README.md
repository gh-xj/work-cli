# work

Local-first work tracker. Plain YAML files in `.work/`. Single Go binary.
No service, no database, no account.

## Install

```sh
go install github.com/gh-xj/work-cli/cmd/work@latest
```

Check whether an installed binary is stale:

```sh
work version --check
```

To refresh a Go-installed binary, rerun `go install`. The CLI does not
self-update; future package-manager installs should be updated through that
package manager.

Pre-built tarballs (`darwin`/`linux`, `amd64`/`arm64`) are attached to each
GitHub release.

## Quick start

```sh
work init                              # create .work/ in $PWD
work inbox add "Idea I want to capture"
work inbox                              # list captured items
work triage accept IN-0001 --priority P1 --area infra
work claim W-0001 --actor xj@laptop       # time-bound coordination lease
work done W-0001 --summary "Shipped" --evidence "task ci passed"
work migrate --dry-run                 # inspect safe store migrations
work migrate                           # backfill older record schema fields
work new "Research question" --type research
work show W-0002 --policy              # print the type policy for typed work
work view ready                         # show ready work items
work show W-0001
```

All commands accept `--json` for machine-readable output.

## File layout

```
.work/
  config.yaml         # store config (gitignored)
  inbox/
    IN-0001.yaml      # inbox capture, schema_version: 1
  items/
    W-0001.yaml       # accepted work item, schema_version: 1
  leases/
    W-0001.yaml       # optional time-bounded claim
  types/
    research/
      type.yaml       # type manifest
      policy.md       # optional agent-facing type policy
      scaffold/       # copied into spaces/ for typed work items
```

The schema is plain YAML. Edit by hand if you want; the CLI re-reads on
every command.

## Status

`v0.x` — used in production by the author, contracts may shift before
`v1.0`. Issues and PRs welcome.

## Philosophy

What `work` is, what it isn't, and why:

- [`docs/core-belief.md`](docs/core-belief.md) — manifesto.
- [`docs/invariants.md`](docs/invariants.md) — verifiable hard constraints.
- [`docs/anti-patterns.md`](docs/anti-patterns.md) — explicit refusals.

## Where this came from

`work` was extracted from
[`agent-repo-kit`](https://github.com/gh-xj/agent-repo-kit), a kit for
making repos legible to coding agents. ARK still ships a
[`work-cli` skill](https://github.com/gh-xj/agent-repo-kit/tree/main/skills/work-cli)
that teaches agents how to drive this binary.

## License

MIT.
