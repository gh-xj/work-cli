# work

Local-first work tracker. Plain YAML files in `.work/`. Single Go binary.
No service, no database, no account.

## Install

```sh
go install github.com/gh-xj/work-cli/cmd/work@latest
```

Pre-built tarballs (`darwin`/`linux`, `amd64`/`arm64`) are attached to each
GitHub release.

## Quick start

```sh
work init                              # create .work/ in $PWD
work inbox add "Idea I want to capture"
work inbox                              # list captured items
work triage accept IN-0001 --priority P1 --area infra
work view ready                         # show ready work items
work show W-0001
```

All commands accept `--json` for machine-readable output.

## File layout

```
.work/
  config.yaml         # store config (gitignored)
  items/
    IN-0001.yaml      # inbox capture
    W-0001.yaml       # accepted work item
```

The schema is plain YAML. Edit by hand if you want; the CLI re-reads on
every command.

## Status

`v0.x` — used in production by the author, contracts may shift before
`v1.0`. Issues and PRs welcome.

## Where this came from

`work` was extracted from
[`agent-repo-kit`](https://github.com/gh-xj/agent-repo-kit), a kit for
making repos legible to coding agents. ARK still ships a
[`work-cli` skill](https://github.com/gh-xj/agent-repo-kit/tree/main/skills/work-cli)
that teaches agents how to drive this binary.

## License

MIT.
