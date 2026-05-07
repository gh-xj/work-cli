# Invariants

These are the facts about `work` that must remain true. Each is grounded in
the current code and verifiable without running the binary. Cite by ID in
PR review (e.g. "violates INV-4").

For aspirational identity, see [`core-belief.md`](core-belief.md).
For explicit refusals, see [`anti-patterns.md`](anti-patterns.md).

---

## INV-1 &nbsp; `os.Exit` lives only in `cmd/work/main.go`

Every other path returns an `int` through `cliruntime.Execute`. This is what
keeps the binary end-to-end testable via `execWriters` in `root_test.go`.

**Verify** &nbsp; `grep -rn 'os\.Exit' cmd/ internal/` returns exactly one
match — `cmd/work/main.go`.

---

## INV-2 &nbsp; Every leaf subcommand has a `Run(*CLI) error` method

No package-level state. Every `*Cmd` struct registered as a Kong leaf
command in `internal/workcli/` reads from the `*CLI` globals (verbose,
JSON, store path, writers).

**Verify** &nbsp; `grep -rn 'func .*Run.*\*CLI.*error' internal/workcli/`
matches every leaf command (Init, InboxAdd, InboxList, TriageAccept, New,
Claim, Migrate, View, Show, Version).

---

## INV-3 &nbsp; The `workStore` interface is the only seam tests substitute

`internal/workcli/service.go` declares the `workStore` interface and a
package-level `openWorkStore` var. Tests install a fake by reassigning
that var (`installFakeStore` in `root_test.go`). No production code may
reach around the interface.

**Verify** &nbsp; `grep -rn 'openWorkStore' internal/workcli/` shows the
declaration, exactly one production caller (`(*CLI).workStore()`), and
test reassignments only.

---

## INV-4 &nbsp; Mutations to durable records are atomic and serialised

Every write to `.work/items/W-NNNN.yaml`, `.work/inbox/IN-NNNN.yaml`,
`.work/leases/W-NNNN.yaml`, or `.work/config.yaml` goes through
`writeFileAtomic` in `internal/work/store.go`. Multi-step mutations are
wrapped in `withMutationLock`, a cross-process file lock at `.work/.lock`.
Type-scaffold materialisation is atomic at the directory level
(stage-and-rename in `internal/work/presets.go`).

**Verify** &nbsp; In `internal/work/store.go`, every public mutation method
calls `withMutationLock`; YAML writes pass through `writeYAMLFile` →
`writeFileAtomic`.

---

## INV-5 &nbsp; `--json` output is stamped with `schema_version: "v1"` automatically

`emitJSON` in `internal/workcli/output.go` stamps `schema_version: "v1"` on
every payload before delegating to `appio.WriteJSON`. Subcommands must not
hand-roll JSON.

**Verify** &nbsp; `grep -rn 'json\.Marshal\|json\.NewEncoder'
internal/workcli/` returns no matches.

---

## INV-6 &nbsp; `WorkStatus` is exactly `{ready, active, blocked, done, cancelled}`

Adding, removing, or renaming a status is a breaking change to every store
ever written.

**Verify** &nbsp; `internal/work/types.go` declares exactly these five
`WorkStatus` constants.

---

## INV-7 &nbsp; Every durable record carries `schema_version`

`WorkItem` and `InboxItem` both have `SchemaVersion int` as a YAML/JSON
field. Records without `schema_version` read as v1. Records above the
current supported version are rejected, never silently rewritten.
`work migrate` backfills the field on older records.

**Verify** &nbsp; `internal/work/types.go` shows the field on both types
with `yaml:"schema_version"` and `json:"schema_version"` tags.

---

## INV-8 &nbsp; `WorkItem.status` encodes lifecycle only

Coordination state lives in `.work/leases/W-NNNN.yaml`. History (attempts,
outcomes) lives in `.work/spaces/W-NNNN/attempts/`. PR/review state, if
ever modeled, lives in its own record. Folding any of these into `status`
silently overloads the field and is a breaking change to its meaning.

**Verify** &nbsp; Review — no production code path sets `WorkItem.status`
to anything outside the five values from INV-6.

---

## INV-9 &nbsp; `actor.id` is treated as an opaque string

The schema does not parse, validate, or interpret model name, runtime, or
vendor from `actor.id`. Recommended forms (`xj@laptop`,
`agent:codex:xj-mac`, `automation:github-actions`) are documentation
conventions, not parsed contracts.

**Verify** &nbsp; `internal/work/types.go` types `Actor.ID` as plain
`string`; no parser anywhere reads structure from its content.

---

## INV-10 &nbsp; Core operation is offline

No subcommand requires network access for its primary function. Each
command runs to completion against a cold disk and exits. The single
opt-in network call lives in `internal/workcli/version.go`, where
`work version --check` queries `api.github.com/repos/gh-xj/work-cli/releases/latest`
to detect stale installed binaries. Removing that path leaves the binary
fully functional.

**Verify** &nbsp; `grep -rn 'net/http' internal/workcli/` matches only
`version.go`.

---

*Companion docs: [`core-belief.md`](core-belief.md) for identity,
[`anti-patterns.md`](anti-patterns.md) for refusals,
[`AGENTS.md`](../AGENTS.md) for engineering style.*
