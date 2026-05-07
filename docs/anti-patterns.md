# Anti-Patterns

Categories of feature that look reasonable on a feature request but invert
one of the invariants or beliefs. They are out of scope without an RFC.
Cite by ID in PR review (e.g. "matches ANTI-1").

For positive identity, see [`core-belief.md`](core-belief.md).
For verifiable hard constraints, see [`invariants.md`](invariants.md).

---

## ANTI-1 &nbsp; Daemons or long-running processes

**Looks like:** `work daemon`, `work watch`, `work serve`, a continuous
poller that reads the inbox or claims work in the background.

**Why not:** every `work` command must work from a cold disk. A daemon
introduces IPC, recovery, lifecycle, "is the daemon up?" failure modes,
and an in-memory source of truth that the file system cannot see.

**Instead:** if the use case is "run something on a tick", drive it from
`cron`/`launchd`/CI. If the use case is cross-repo aggregation, write a
read-only on-demand aggregator that fans out to per-store `work` calls.

*Inverts INV-10, belief #3.*

---

## ANTI-2 &nbsp; External service or account dependency

**Looks like:** required Linear/GitHub/Slack tokens, mandatory `work
login`, a remote registry of work items, a sync server, OAuth in the
binary.

**Why not:** the disk you own is the only authority. The moment a `work`
command requires a remote service to function, the local-first contract
is broken: offline use stops working, the user inherits the service's
uptime, and the store stops being self-contained.

**Instead:** read-only adapters that *augment* `.work/` with external
data are acceptable as separate tools. Centralising the store is not.

*Inverts INV-10, belief #4.*

---

## ANTI-3 &nbsp; Persistent caches, indices, or sidecar databases

**Looks like:** `.work/index.db`, `.work/cache/`, an in-memory cache
shared between subcommand invocations, a SQLite index of work items, a
"warm start" mode that skips re-reading YAML.

**Why not:** caches age, indices drift, half-written sidecars are their
own bug class. The YAML files are the truth; anything derived from them
is one race away from being a second truth that disagrees.

**Instead:** if YAML reading is too slow on a real `.work/`, profile and
fix the YAML reader, not the absence of caching.

*Inverts INV-7's recovery model, belief #3.*

---

## ANTI-4 &nbsp; Encoding coordination state into `WorkItem.status`

**Looks like:** adding a `claimed` status, an `in_progress` status, a
`reviewing` status. Mutating `status` to `active` to mean "an agent is
running on this".

**Why not:** `status` is lifecycle. Coordination is who-holds-the-thing.
History is what-was-attempted. Once a single field encodes two of these,
downstream readers cannot tell them apart.

**Instead:** lease records (`.work/leases/W-NNNN.yaml`) for coordination.
Append-only attempt logs for history. Both are first-class structures
with their own files.

*Inverts INV-6, INV-8, belief #6.*

---

## ANTI-5 &nbsp; Vendor-coupled identity

**Looks like:** an `agent` enum constrained to `codex`, `claude`, `gpt`.
Treating the model name as identity. Hard-coding `anthropic` or `openai`
into the schema. Requiring URN-style IDs like `agent://anthropic/opus-4.7`.

**Why not:** models are renamed; runtimes change shape; vendors come and
go. Any schema field that names a runtime inherits that runtime's
lifetime. A future runtime that wants to drive `work` should not need a
`work` release.

**Instead:** `actor.id` is opaque. `actor.kind` is one of three coarse
categories (`human`, `agent`, `automation`). `actor.runtime` and
`actor.model` are optional informational metadata.

*Inverts INV-9, belief #5.*

---

## ANTI-6 &nbsp; Configurable workflow or transition graph

**Looks like:** `.work/workflow.yaml` declaring "ready can transition to
reviewing only via this command", "transitions require a comment", "this
status is terminal and irreversible". Per-team status enums.

**Why not:** `work` is not a workflow engine. The five lifecycle statuses
are deliberately coarse. Adding configurability turns the tracker into a
state machine that has to validate, audit, and version transitions —
features for a different audience (Jira does this well).

**Instead:** if a team needs richer workflow, that lives outside `work`.
Compose `work update --status` with the team's own checks; do not push
the checks into the tracker.

*Inverts belief #9.*

---

## ANTI-7 &nbsp; Lifecycle hooks that execute code

**Looks like:** `.work/hooks/before-claim.sh`, an `after_create` shell
hook in a type scaffold, a `pre-status-change` validator script, a
"webhook on triage" emitter.

**Why not:** hooks turn the tracker into an execution environment. The
moment `work` runs arbitrary code on user state changes, it acquires the
sandbox, retry, error-propagation, and security surface of an
orchestrator — exactly the boundary `work` declines to cross.

**Instead:** prose files like `RULES.md` and `notes.md` shipped in type
scaffolds are descriptive and read by agents. The CLI does not execute
them.

*Inverts INV-10, belief #1.*

---

## ANTI-8 &nbsp; Friction at capture, automation at commitment

**Looks like:** required `--area` or `--priority` on `inbox add`.
Auto-promotion of inbox entries older than N days into work items.
Mandatory scope estimates. AI-suggested priorities applied without
review.

**Why not:** capture must stay cheap so ideas don't die in the mind.
Triage must stay deliberate so the backlog reflects intent. Inverting
either side of this asymmetry makes both halves worse.

**Instead:** all `inbox add` flags are optional. Triage is a manual
decision via `work triage accept`, not a timeout.

*Inverts belief #8.*

---

*Companion docs: [`core-belief.md`](core-belief.md) for identity,
[`invariants.md`](invariants.md) for verifiable constraints,
[`AGENTS.md`](../AGENTS.md) for engineering style.*
