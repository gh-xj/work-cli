# AGENTS.md — work-cli

Source of `work`, an agent-native local-first work tracker. Single Go
binary; no service, no database. The on-disk contract is plain YAML under
`.work/`.

Consumer/intake for this CLI lives in the sibling repo
[`gh-xj/work-cli-dev`](https://github.com/gh-xj/work-cli-dev) — that's
where design notes and inbox capture for evolving `work` happen. This
repo contains source only.

## Philosophy

Read before adding a feature, before opening a PR that changes the on-disk
schema, and before arguing for a richer lifecycle:

- [`docs/core-belief.md`](docs/core-belief.md) — manifesto (what `work` is, what it isn't).
- [`docs/invariants.md`](docs/invariants.md) — verifiable hard constraints, citable in PR review (`violates INV-N`).
- [`docs/anti-patterns.md`](docs/anti-patterns.md) — explicit refusals, citable in PR review (`matches ANTI-N`).

## Architecture

- `cmd/work/main.go` — entry point. One line: `os.Exit(workcli.Execute(os.Args[1:]))`.
- `internal/workcli/` — CLI surface. One file per subcommand
  (`init.go`, `inbox.go`, `triage.go`, `new.go`, `claim.go`, `done.go`,
  `view.go`, `show.go`, `version.go`), plus `root.go` (global flags + `CLI` struct) and
  `service.go` (the `workStore` seam used by tests).
- `internal/cliruntime/` — shared kong-runner plumbing (see invariant
  below). May host a sibling binary in the future.
- `internal/work/` — store, schema, presets. The durable on-disk contract
  lives here.
- `internal/appctx/` — exit-code policy (`ExitSuccess=0`, `ExitError=1`,
  `ExitUsage=2`, `ResolveExitCode(err)`).
- `internal/io/`, `internal/log/` — output and logging primitives.

## Non-Negotiable Invariants

- **No `os.Exit` outside `cmd/work/main.go`.** All paths must return an
  `int` from `cliruntime.Execute`. This is what makes the CLI testable
  end-to-end via `execWriters` in `root_test.go`. kong-managed exits
  (VersionFlag, HelpFlag) are captured by `kong.Exit(...)` in
  `cliruntime/execute.go` for the same reason.
- **Subcommand structs implement `Run(globals *CLI) error`.** No
  package-level state; everything reads from `globals` (verbose, JSON,
  store path, writers).
- **The `workStore` interface is the only seam tests cross.** Tests
  install a fake via the package-level `openWorkStore` var declared in
  `internal/workcli/service.go` (see `installFakeStore` in
  `root_test.go`). Don't bypass it.
- **`--json` output goes through `emitJSON` in `workcli/output.go`,**
  which stamps `schema_version: "v1"` on every payload before delegating
  to `appio.WriteJSON`. Don't hand-roll JSON in a subcommand.
- **The on-disk schema is the public contract.** Adding/renaming fields
  in `internal/work/` types is a breaking change to anything that has
  written `.work/` on disk. Treat with the same care as a CLI flag rename.

## Verification

Canonical gate:

```sh
task ci    # lint + test + build + smoke
```

Individual gates:

```sh
task fmt:check   # gofmt -l . must be empty
task lint        # go vet ./...
task test        # go test ./...
task build       # binary into bin/work
task smoke       # exercises the built binary end-to-end via .work/
task qa          # black-box QA via work-cli-qa skill harness
```

`task smoke` is ground truth for "the binary actually works." Any change
that touches CLI flags, output formats, or store layout must keep `smoke`
green. If smoke needs to change, that's a contract change — call it out.

## Style

- One subcommand per file under `internal/workcli/`. Don't merge two.
- Kong tags follow the patterns in `~/.claude/skills/go-scripting/references/kong-patterns.md`.
- Logger is `slog` configured via `internal/log/log.Setup` from
  `BeforeRun` in `cliruntime`. Don't introduce a second logger.
- Errors that should map to a non-zero exit go through `appctx`. Don't
  invent new exit codes inline.

## Pointers

- Consumer / intake: `../work-cli-dev/` on disk.
- Public OSS: https://github.com/gh-xj/work-cli
- Releases: tarballs for `darwin`/`linux` × `amd64`/`arm64`.
