# Research: Trace Analysis UI (Open & Axial Coding)

`spec.md`'s Assumptions explicitly leave "the exact sidecar JSON schema,
HTTP API surface, package layout, and frontend implementation approach"
for this phase. There are no outstanding `NEEDS CLARIFICATION` markers in
Technical Context — every item below is an implementation-detail decision,
not a product ambiguity (those were already closed in spec.md's
Clarifications session). Each entry follows the Decision / Rationale /
Alternatives format.

## 1. Trace-source dispatch: shape-sniffing rule

**Decision**: Read the whole input file once. Try `json.Unmarshal` of the
full byte slice into `map[string]json.RawMessage`. If that succeeds *and*
the resulting map has an `"outcomes"` key, dispatch to the
`SuiteRunReport` path (decode fully as `report.SuiteRunReport`, post
FR-001, and extract `Traces`). Otherwise, dispatch to the standalone
JSONL raw-trace path (FR-003) and parse the file line-by-line.

**Rationale**: This mirrors `internal/suite` and `internal/dataset`'s
existing "dispatch by shape, not extension" convention (`code-internals.md`
§4, §5), adapted because there's only one file argument here, not an
extension signal. The naive alternative — "does the whole file parse as
one JSON document?" — is not sufficient by itself: a standalone raw-trace
file containing exactly **one** trace is also valid as a single JSON
object, so it would be indistinguishable from a `SuiteRunReport` on shape
alone unless a further, more specific check is applied. Keying on the
`"outcomes"` key resolves this because `report.SuiteRunReport.Outcomes`
has no `omitempty` (`internal/report/report.go`) — it is always present
as a JSON key (even `"outcomes":null` for zero outcomes) in every report
`evalctl run --output` has ever written, and a raw single-trace object has
no reason to carry a field with that exact name.
A multi-line JSONL file (N > 1 traces) fails the initial whole-file
unmarshal outright (`json.Unmarshal` rejects trailing data after the first
value), which already routes it to the JSONL path without needing the key
check at all — the key check only matters for the single-trace-object
edge case.

**Alternatives considered**:
- A `--format` flag forcing the choice: rejected — FR-002 requires *no
  different setup steps* between the two input kinds; adding a flag the
  user must remember defeats that.
- File extension sniffing (`.json` vs `.jsonl`): rejected — a user may
  reasonably name either file `results.json`/`traces.json`; the spec's own
  edge cases talk about "file shape," not extension, and `internal/suite`
  already sets the precedent that when a real signal (extension) exists
  it's used, but here it doesn't reliably exist.

## 2. FR-001: extending `SuiteRunReport` with trace data

**Decision**: Add one new field, `Traces []Trace`, to
`report.SuiteRunReport`, where `Trace` is a new exported struct in
`internal/report`:

```go
type Trace struct {
    TaskID string          `json:"task_id"`
    Input  json.RawMessage `json:"input"`
    Output json.RawMessage `json:"output"`
    Steps  []protocol.Step `json:"steps,omitempty"`
}
```

`cmd/evalctl/run.go`'s `runTask` returns its `protocol.Transcript` (or a
zero value on consumer failure) alongside the outcomes it already returns;
the fan-in loop in `runCommand` appends a `Trace` to a new `allTraces`
slice **only when a transcript was actually produced** (i.e., the consumer
subprocess succeeded), using the task's own `Input` for `Trace.Input`.
`report.BuildSuiteRunReport` gains a `traces []Trace` parameter and copies
it straight into the returned report, unchanged by aggregation.

**Rationale**: This is additive (spec's own framing) — existing consumers
of `SuiteRunReport` (that only read `outcomes`/`grader_scores`/
`overall_score`/`passed`) are unaffected. Skipping traces for failed
consumer calls avoids inventing a placeholder `Output` value for a trace
that never had one; the *outcomes* for that task already fully record the
failure (`ErrConsumerError`/`ErrTimeout`), so nothing about the failure is
lost, and the analysis UI simply won't offer a trace with no real output
to annotate for that task id. This also gives a direct, cheap way to
detect the "pre-FR-001 report" edge case (§8 below): decode the file as
`SuiteRunReport`, and if `Outcomes` is non-empty but `Traces` is empty,
it's an old-format report.

**Alternatives considered**:
- A wholly separate `--output-traces <path>` file from `evalctl run`:
  rejected — spec explicitly calls this "an additive change to the
  existing report shape, not a new artifact/command" (FR-001).
- Reusing `protocol.Transcript` directly instead of a new `report.Trace`:
  rejected — `Transcript` has no field for the task's original `Input`,
  which the analysis UI needs to display (`AnalysisTrace.Input`); pairing
  input+output+steps in one struct at the report layer avoids the
  consumer needing to zip two parallel arrays back together by `task_id`.

## 3. Sidecar schema and ID scheme

**Decision**: One JSON object per sidecar file:

```json
{
  "version": 1,
  "open_codes": [ { "id": "oc-1", "trace_id": "t1", "label": "Hallucinated citation", "normalized_label": "hallucinated citation", "note": "", "created_at": "2026-09-20T10:00:00Z" } ],
  "axial_codes": [ { "id": "ax-1", "name": "Factual errors", "description": "", "member_labels": ["hallucinated citation"] } ],
  "next_open_code_id": 2,
  "next_axial_code_id": 2
}
```

IDs are short, sequential, store-scoped strings (`oc-<n>`, `ax-<n>`)
generated from the two persisted counters, not UUIDs. Deleting an item
never reuses its id (the counter only increments).

**Rationale**: Sequential ids keep the file's diffs small and readable
(a new annotation is one new array element plus a one-line counter bump)
— directly serving the "human-diffable" requirement from spec.md's
Clarifications. Persisting the counters (rather than deriving next-id from
`len(open_codes)+1`) avoids id reuse after a delete, which matters because
`AxialCode.member_labels` and future references key off the *label
string*, not the annotation id, but annotation ids still need to stay
stable/unique across the store's lifetime for edit/delete addressing
(FR-005). `normalized_label` (trimmed, case-folded per FR-008) is stored
alongside the as-typed `label` so aggregation/matching never has to
re-normalize on every read, and the *first* annotation ever recorded for a
given normalized label is what supplies the display casing (found by
scanning `open_codes` in file order, which is insertion order — no
separate "canonical label" table needed).

**Alternatives considered**:
- UUIDs (`google/uuid` or stdlib `crypto/rand`-based): rejected — adds
  either a new dependency (against the project's minimal-dependency
  posture, `code-internals.md` §Technical Context in `001`'s plan) or
  boilerplate hex-encoding for no benefit in a single-local-user tool with
  no id-collision risk across machines.
- A separate `labels` table keyed by normalized label (storing the
  canonical display casing once) instead of deriving it from `open_codes`
  order: rejected as premature normalization — deriving it is O(n) over a
  list that's already being scanned for aggregation (FR-008) in the same
  request, so a separate table only adds a second place that could drift
  out of sync with the annotations it's derived from.

## 4. HTTP routing

**Decision**: Go 1.22+'s enhanced `net/http.ServeMux` (method+path
patterns, e.g. `mux.HandleFunc("POST /api/traces/{id}/annotations", ...)`)
— no third-party router.

**Rationale**: `go.mod` already targets `go 1.26.5`, well past the 1.22
baseline where this shipped, so no new capability is being reached for;
using it keeps this feature's dependency footprint exactly where `001`
left it (stdlib + `gopkg.in/yaml.v3`, and `yaml.v3` isn't even needed
here). Matches `code-internals.md`'s repeated emphasis on stdlib-first,
dependency-light design (no CLI framework, no router).

**Alternatives considered**: `gorilla/mux` or `chi` — rejected, no
capability gap to justify a new dependency for four resource groups worth
of routes.

## 5. Frontend delivery

**Decision**: Static HTML/CSS/vanilla JS under `internal/analyze/web/`,
embedded via `go:embed`, served by the same `net/http.ServeMux` at `/`
(the API lives under `/api/`). No SPA framework, no build step, no
`package.json`.

**Rationale**: Directly required by FR-016 (no Node/npm/extra runtime)
and matches `001`'s own single-binary distribution story. The UI's scope
per spec.md (trace list, trace detail + annotate form, open-codes list,
axial-codes board) is well within what plain JS + `fetch()` against the
JSON API can do without a framework's benefits (routing, component state)
paying for themselves.

**Alternatives considered**: A separate `frontend/` npm project built and
copied in at release time (the "Option 2: Web application" layout the
plan template offers) — rejected outright by FR-016; a prebuilt
`frontend/dist` checked into git would still require Node.js for anyone
who needs to change it, silently reintroducing the dependency the
requirement forbids.

## 6. Persistence mechanics (write-through + reload)

**Decision**: One in-memory `*Store` guarded by a `sync.Mutex`. Every
mutating HTTP handler takes the lock, mutates the in-memory structs,
serializes the whole store to JSON, writes it to a temp file in the same
directory as the sidecar path, then `os.Rename`s it over the real sidecar
path, then releases the lock. Startup reads the sidecar path if it exists
(`os.Stat` + decode) and populates the initial in-memory state; if it does
not exist, starts from an empty store with counters at 1.

**Rationale**: temp-file-then-rename is the standard atomic-write pattern
on POSIX (and `os.Rename` is atomic on the same filesystem on Windows
too, for a same-volume replace) — a crash mid-write leaves either the old
complete file or the new complete file, never a torn/partial one,
directly satisfying FR-006's "no data loss window beyond the single write
in flight" and the equivalent language in SC-003. A single mutex around
the whole read-modify-write cycle is sufficient (not a `sync.RWMutex` or
per-resource locking) because this is explicitly a single-local-user tool
(spec.md Assumptions) — write throughput under concurrent load is a
non-goal, and the edge case about two browser tabs is explicitly resolved
as "serialize, last-write-wins," which one coarse mutex gives for free.

**Alternatives considered**: `flock`-style file locking instead of an
in-process mutex — rejected as solving a problem that doesn't exist here
(a single `evalctl analyze` process is the only writer; there's no
multi-process contention to guard against, only multi-*request*
contention within that one process).

## 7. LLM-suggest call shape (FR-011)

**Decision**: A new small HTTP client in `internal/analyze/suggest.go`,
structurally parallel to `internal/grader/judge/judge.go`: POST a JSON
body `{protocol_version, labels: [{label, sample_notes: [...]}, ...]}` to
the configured endpoint, `Authorization: Bearer <env var value>` if
`--llm-api-key-env` is set, bounded by a fixed timeout via
`context.WithTimeout`. Expected response:
`{"groupings": [{"name": "...", "labels": ["..."]}]}`. Any transport
error, non-2xx, or malformed-JSON response is surfaced to the HTTP caller
as a `4xx`/`5xx` JSON error body — never a panic, never a partial write to
the sidecar file (the suggestion is never written to disk in the first
place per FR-011; only "apply" — a normal axial-code create/assign call —
touches the store).

**Rationale**: Reuses a proven pattern (`code-internals.md` §7.3) rather
than inventing a new "call an external HTTP thing safely" idiom; the
project already accepts judge.go and this suggest-endpoint client as two
independent, non-shared implementations of the same idea (per §7.3's own
"deliberate-by-omission asymmetry" note, extended here to a third
call site — still not enough to justify extracting a shared helper).

**Alternatives considered**: Reusing `internal/grader/judge` directly by
shoehorning the suggestion into an `llm_judge`-shaped request/response —
rejected, the request/response shapes are unrelated (grading a single
transcript vs. proposing a grouping over N labels); forcing a shared type
would couple two unrelated contracts for no reuse benefit.

## 8. Pre-FR-001 report detection (edge case)

**Decision**: After dispatching to the `SuiteRunReport` path (§1), if the
decoded report has `len(Outcomes) > 0` and `len(Traces) == 0`, fail fast
with an error naming the file and stating it predates persisted
transcripts (FR-001), instructing the user to re-run `evalctl run
--output` to regenerate it.

**Rationale**: Directly matches the spec's own edge case wording
("fails fast with an actionable error naming the missing data"). The
`Outcomes > 0 && Traces == 0` check cannot false-positive on a *new*-format
report that legitimately has zero traces, because that can only happen
when `Outcomes` is also empty (a run with zero tasks) — the empty-input
case is already independently rejected by `dataset.Load`'s "dataset is
empty" check before `evalctl run` ever gets to produce a report.

## 9. CLI shape and default port

**Decision**: `evalctl analyze --traces <path> [--annotations <path>]
[--addr host:port] [--llm-endpoint <url>] [--llm-api-key-env <VAR>]`.
`--annotations` defaults to `<traces>.annotations.json` next to the traces
file if omitted. `--addr` defaults to `localhost:8420`; if the address is
already in use, `net.Listen` fails and `analyzeCommand` returns that error
immediately (wrapped with a clear message) rather than retrying on another
port, matching the spec's edge case ("fails fast ... rather than silently
picking a different port").

**Rationale**: Consistent with `cmd/evalctl`'s existing per-subcommand
`flag.NewFlagSet` pattern (`code-internals.md` §9); a fixed, documented
default port is simpler to reason about and script against than an
ephemeral one, while still failing loudly (not silently) on conflict.

**Alternatives considered**: Binding to `:0` (OS-assigned ephemeral port)
and printing the chosen port: rejected — the spec's edge case explicitly
wants a clear failure on the *chosen* port being busy, implying a
user-predictable default is expected, not an always-random one.
