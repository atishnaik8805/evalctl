# Implementation Plan: Trace Analysis UI (Open & Axial Coding)

**Branch**: `002-trace-analysis-ui` | **Date**: 2026-09-20 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/002-trace-analysis-ui/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command; its definition describes the execution workflow.

## Summary

A new `evalctl analyze` subcommand starts a local, single-user web UI
(embedded static HTML/CSS/vanilla JS, no Node/build step) for open-coding
and axial-coding raw AI traces. It accepts either a (post-FR-001)
`SuiteRunReport` or a standalone JSONL raw-trace file as equally valid
input, dispatching between them by content shape (research.md §1).
Annotations write through immediately to a human-diffable JSON sidecar
file via temp-file-then-rename (research.md §6), surviving an unclean
process kill. Open-code labels group into axial codes with single
membership per label; an optional, explicitly-triggered "suggest with
LLM" call proposes a draft grouping over currently-uncategorized labels
without ever auto-applying or touching the sidecar file until the user
accepts. All four implementation-detail questions the spec left open
(sidecar schema, HTTP API surface, package layout, frontend approach) are
resolved in research.md.

## Technical Context

**Language/Version**: Go 1.26 (matches `go.mod`; no version bump needed).

**Primary Dependencies**: Go standard library only —
`net/http` (enhanced `ServeMux` method+path routing, research.md §4),
`embed` (frontend assets, research.md §5), `encoding/json`, `os`/`os.Rename`
(atomic sidecar writes, research.md §6), `bufio` (JSONL parsing, reusing
the pattern already in `internal/dataset`). No new external Go module and
no JS/CSS build tooling — this feature needs neither `gopkg.in/yaml.v3`
(no YAML input here) nor anything new.

**Storage**: A local JSON sidecar file per trace input (`AnnotationStore`,
data-model.md), read on startup and written through atomically on every
mutation. No database, no daemon persisting across `evalctl analyze`
invocations beyond that one file.

**Testing**: Go stdlib `testing`, table-driven, colocated `_test.go` files
in the new `internal/analyze` package (trace-format dispatch/parsing,
store mutation/normalization logic, HTTP handlers via `net/http/httptest`).
Integration coverage lives in `tests/integration/analyze_test.go`, driving
the real HTTP server against a real sidecar file on a temp directory (same
convention as `tests/integration/run_test.go`). No frontend test
tooling is introduced (would require Node.js, contradicting FR-016); the
frontend's own logic is kept thin enough that `quickstart.md`'s manual
scenarios are the frontend's verification, with all substantive logic
(validation, normalization, persistence, aggregation) covered by Go tests
server-side.

**Target Platform**: Same as `001` — cross-platform CLI binary
(Linux/macOS/Windows, amd64+arm64); the analyze server binds to
`localhost` only (FR-015).

**Project Type**: Single project — extends the existing Go CLI layout, no
separate frontend project (research.md §5).

**Performance Goals**: Browsing/annotating one trace at a time must stay
responsive for a trace set in the thousands (spec.md Edge Cases) — the UI
fetches one trace's detail per selection (`GET /api/traces/{id}`) rather
than shipping the whole set's outputs to the browser up front; the trace
*list* endpoint returns only ids, not full input/output, to keep initial
page load flat regardless of trace count.

**Constraints**: Offline-by-default beyond the one explicit, optional
LLM-assist HTTP call (FR-015). No auth (FR-015/spec.md Assumptions,
single trusted local user). No Node.js/npm required to build, run, or
modify the UI (FR-016). Single mutex-guarded writer; concurrent
tabs/windows serialize with last-write-wins (spec.md Edge Cases).

**Scale/Scope**: One trace file per `evalctl analyze` invocation; up to
low-thousands of traces; open-code/axial-code counts expected in the tens
to low-hundreds for a single analysis session — no pagination is designed
in for the open-codes/axial-codes list endpoints, only for avoiding
shipping full trace bodies up front (see Performance Goals).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

`.specify/memory/constitution.md` still contains only unfilled template
placeholders — no principles have been ratified for this project. There
are no gates to evaluate against. This gate passes vacuously, same as
`001`'s plan.md recorded. If the constitution is ratified later, both
feature plans should be re-checked against it together.

## Project Structure

### Documentation (this feature)

```text
specs/002-trace-analysis-ui/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
│   ├── http-api.md
│   └── raw-trace-format.md
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
cmd/
└── evalctl/
    ├── analyze.go                 # NEW: `analyze` subcommand — flag parsing, wires
    │                               #      analyze.LoadTraces + analyze.Store + analyze.Server,
    │                               #      thin composition layer per code-internals.md §9
    ├── run.go                     # MODIFIED (FR-001): runTask also returns the produced
    │                               #      Transcript; runCommand accumulates []report.Trace
    │                               #      alongside []protocol.Outcome and passes both to
    │                               #      report.BuildSuiteRunReport
    └── main.go                    # MODIFIED: add "analyze" to the subcommand dispatch + usage

internal/
├── report/
│   └── report.go                 # MODIFIED (FR-001): SuiteRunReport gains `Traces []Trace`;
│                                   #      new exported `Trace` struct; BuildSuiteRunReport
│                                   #      takes an additional []Trace parameter
├── analyze/                       # NEW package — this feature's business logic
│   ├── trace.go                   #   AnalysisTrace type; LoadTraces (shape dispatch,
│   │                               #   research.md §1; SuiteRunReport + JSONL parsing,
│   │                               #   pre-FR-001 detection, research.md §8)
│   ├── trace_test.go
│   ├── store.go                   #   AnnotationStore, OpenCodeAnnotation, AxialCode;
│   │                               #   label normalization; Load/Save (temp-file+rename,
│   │                               #   research.md §6); all mutation methods (create/edit/
│   │                               #   delete annotation; create/rename/delete axial code;
│   │                               #   assign/unassign membership with single-membership
│   │                               #   enforcement); aggregation (open-codes view, FR-008;
│   │                               #   axial trace-count union, per clarification)
│   ├── store_test.go
│   ├── server.go                  #   net/http.ServeMux wiring per contracts/http-api.md;
│   │                               #   one handler per route, all mutation routes going
│   │                               #   through Store's mutex-guarded methods
│   ├── server_test.go             #   httptest-based handler tests covering every route's
│   │                               #   success/4xx/409 paths in contracts/http-api.md
│   ├── suggest.go                 #   LLM-assist HTTP client (research.md §7), only wired
│   │                               #   into the mux when configured (FR-012)
│   ├── suggest_test.go
│   └── web/                       #   go:embed'd static frontend (research.md §5)
│       ├── embed.go               #   //go:embed directive exposing an embed.FS
│       ├── index.html
│       ├── app.js                 #   vanilla JS: fetch() against /api/*, no framework
│       └── style.css
├── protocol/                      # UNCHANGED — Step/Task/Transcript already shaped correctly
├── dataset/                       # UNCHANGED — not reused directly (raw-trace parsing has
│                                   #   its own required `output` field dataset.Task lacks),
│                                   #   but trace.go's JSONL scanning mirrors its pattern
├── suite/, runner/, grader/*, calibrate/   # UNCHANGED

tests/
├── integration/
│   └── analyze_test.go            # NEW: end-to-end — real HTTP server, real sidecar file
│                                   #   on a temp dir, covering quickstart.md's scenarios
└── fixtures/                      # UNCHANGED (analyze needs no subprocess fixtures — it has
                                   #   no consumer/grader boundary of its own)

go.mod, go.sum                     # UNCHANGED — no new dependency
```

**Structure Decision**: Extends the existing single-project Go CLI layout
rather than introducing a `frontend/`+`backend/` split — `internal/analyze`
follows the same "one package, colocated tests" convention as every other
`internal/*` package (`code-internals.md` §2), and the embedded `web/`
directory keeps the frontend physically inside that package rather than a
sibling top-level project, since FR-016 rules out a Node-built frontend
that would justify separating it. `cmd/evalctl/analyze.go` stays a thin
composition layer, matching `run.go`/`validate.go`/`calibrate.go`'s
existing shape (§9's "adding a fifth subcommand" recipe, followed
directly). The one cross-cutting change (`internal/report`) is additive
only, per FR-001's own framing.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

Not applicable — no ratified constitution exists to violate (see
Constitution Check above).

## Post-Design Constitution Check

*Re-evaluated after Phase 1 (data-model.md, contracts/, quickstart.md).*

Unchanged from the pre-design check: no ratified principles exist to
re-evaluate against. No new external dependency was introduced during
design (research.md §4/§5 both explicitly reject third-party options in
favor of stdlib); the one new package (`internal/analyze`) mirrors the
existing package-per-concern convention rather than adding a new
architectural layer, and the one modification to an existing package
(`internal/report`) is additive, matching what Technical Context already
committed to.
