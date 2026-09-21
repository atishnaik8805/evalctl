# Feature Specification: Trace Analysis UI (Open & Axial Coding)

**Feature Branch**: `002-trace-analysis-ui`

**Created**: 2026-09-20

**Status**: Draft

**Input**: User description: "Support the analysis part of the evals workflow in the tool. Create a UI, served by evalctl itself, that takes traces as input and helps with analysis: adding annotations to the output of an AI trace that act as open codes, which are then categorized — on a button click — into axial codes."

## Relationship to the existing spec

`specs/001-universal-eval-framework/spec.md`'s Assumptions section explicitly
deferred this: *"The pluggable simulator..., a hosted/SaaS offering with live
dashboards, **a web UI**, and real-time evaluation of live production traffic
are explicitly out of scope for this feature (deferred to a later
milestone)."* This spec is that milestone — scoped narrowly to the
**analysis** phase (open coding → axial coding), not a general dashboard.

This phase sits *before* grader-building in the broader workflow (raw traces
→ open codes → axial codes → grader/`llm_judge` rubric → `calibrate` →
CI gate via `fail_below`). Per clarification below, this feature's output
stops at annotated/categorized data — turning axial codes into a grader
config is explicitly out of scope here.

## Clarifications

### Session 2026-09-20

- Q: What should count as "traces" fed into the analysis UI? → A: Both — accept an `evalctl run --output` `SuiteRunReport` when one exists, or a standalone raw trace file for traces that never went through `evalctl` (e.g. pulled from production logs).
- Q: How should the "categorize open codes into axial codes" button behave? → A: Manual grouping is the core mechanic (always works, no external dependency); an optional "suggest with LLM" button pre-populates groupings the user then reviews/edits — never auto-applies silently.
- Q: Where should annotations (open codes + axial-code groupings) be persisted? → A: A local, human-diffable JSON sidecar file next to the input traces, written by the tool directly — no embedded database.
- Q: What should this feature produce at the end, to connect to grader-building? → A: Annotated data only (open codes + axial-code groupings as JSON). Scaffolding a suite/grader config from that data is explicitly deferred to a future feature.
- Q: When you group an open-code label into a category, can that same label sit inside more than one axial code at once, or does putting it in a new one always mean taking it out of whatever it was in before? → A: Single membership — a label belongs to exactly one axial code (or none, i.e. "uncategorized"); assigning it to a new axial code removes it from the previous one.
- Q: Should the standalone raw-trace input file be one JSON array of trace objects, or newline-delimited JSON (JSONL)? → A: JSONL — one JSON trace object per line, no enclosing array.
- Q: When two annotations use labels that differ only in case or surrounding whitespace, should they count as the same open-code label for aggregation and axial-code grouping, or as two distinct labels? → A: Normalize for matching (trim whitespace, case-fold) but display the first-used original casing; two labels differing only in case/whitespace are the same label.
- Q: When the last open-code label assigned to an axial code gets reassigned elsewhere (or deleted), leaving that axial code with zero members, should the now-empty axial code be deleted automatically, or kept around as an empty category? → A: Keep it visible in the list until the user explicitly deletes it; no automatic deletion.
- Q: Can a user attach the same open-code label to the same trace more than once, and does the FR-008 usage count reflect distinct traces or every annotation instance? → A: Prevent duplicates — attaching an already-present (normalized) label to the same trace is rejected, directing the user to edit the existing annotation instead; the aggregate count is the number of distinct traces carrying the label.
- Q: When a user clicks "suggest with LLM" for axial-code grouping, should the proposal cover only currently-uncategorized open-code labels, or re-propose groupings for all distinct labels including ones already manually assigned? → A: Uncategorized labels only — the suggestion request and draft exclude labels already assigned to an axial code, so existing manual groupings are never touched by a suggestion.
- Q: When a trace carries two open-code labels that are both grouped into the same axial code, does that axial code's displayed "combined trace count" count that trace once, or sum the per-label counts (counting it twice)? → A: Distinct trace count (union) — a trace carrying two member labels in the same axial code is counted once, consistent with the distinct-trace approach used for per-label counts (FR-008).
- Q: Can a user create a new axial code with zero open-code labels assigned yet (an empty category to populate later), or must at least one open-code label be assigned at creation time? → A: Allow empty creation — creating an axial code and assigning labels to it are two separate actions; a freshly created axial code can have zero members, the same state it can also reach later by losing its last member (FR-009).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Load a set of traces into the analysis UI (Priority: P1)

A developer has either a `results.json` from a prior `evalctl run`, or a
standalone file of raw traces pulled from somewhere else (e.g. production
logs, a different eval tool), and wants to browse them one at a time in a
local web UI before annotating anything.

**Why this priority**: Nothing else in this feature is reachable without a
loaded, browsable set of traces — it's the minimum slice.

**Independent Test**: Point `evalctl analyze` at a `SuiteRunReport` file and,
separately, at a standalone raw-trace JSONL file; in both cases the UI lists
every trace by id and renders one trace's input/output (and steps, if
present) when selected, with no annotation actions required to pass this
test.

**Acceptance Scenarios**:

1. **Given** a `results.json` produced by `evalctl run --output`, **When**
   `evalctl analyze` is started against it, **Then** the UI lists one entry
   per task id and displays that task's input and the system's output for
   the selected entry.
2. **Given** a standalone raw-trace file that was never produced by
   `evalctl run`, **When** `evalctl analyze` is started against it, **Then**
   the UI lists and displays traces the same way as for a `SuiteRunReport`,
   with no different setup steps.
3. **Given** a trace with `steps` (trajectory data), **When** it is
   selected, **Then** the UI renders the step sequence in order, not just
   the final output.
4. **Given** an existing annotation JSON sidecar file for this trace set
   (from a previous session), **When** `evalctl analyze` starts, **Then**
   previously recorded open codes and axial-code groupings are loaded and
   shown, not discarded.

---

### User Story 2 - Open-code annotate a trace's output (Priority: P1)

A developer reviewing one trace at a time wants to attach one or more free-
text labels ("open codes") — optionally with a short note — to what they
observe in that trace's output, capturing recurring phenomena (e.g.
"hallucinated citation", "refused valid request", "wrong tone") as they go.

**Why this priority**: This is the actual analysis act the feature exists
for; Story 1 only delivers value once this exists.

**Independent Test**: With a trace set loaded, add two open-code annotations
to one trace and one to another, restart `evalctl analyze` against the same
files, and confirm all three annotations are still present and attributed
to the correct trace.

**Acceptance Scenarios**:

1. **Given** a selected trace, **When** the user adds an open-code label
   (plain text) with an optional note, **Then** it is saved immediately
   (write-through to the sidecar file) and appears attached to that trace.
2. **Given** a trace with existing annotations, **When** the user edits or
   deletes one, **Then** the sidecar file reflects the change immediately
   and no other trace's annotations are affected.
3. **Given** annotations already exist across several traces, **When** the
   user views an aggregate "open codes" list, **Then** it shows each
   distinct open-code label with a count of how many distinct traces carry
   it (a trace can carry a given normalized label at most once, per FR-004).
4. **Given** the `evalctl analyze` process is killed without a clean
   shutdown right after an annotation is added, **When** it is restarted
   against the same files, **Then** that annotation is still present (no
   data loss window beyond the single write that was in flight).

---

### User Story 3 - Group open codes into axial codes (Priority: P2)

Once enough traces have been open-coded, a developer wants to define a
smaller set of named categories ("axial codes") and manually assign each
distinct open-code label to one or more of them, to see the phenomena
consolidate into a handful of graded categories worth building a grader
around.

**Why this priority**: Delivers the second half of the "open coding →
axial coding" workflow the feature is named for, but only makes sense once
Story 2 has produced open codes to group.

**Independent Test**: With at least two distinct open-code labels already
recorded (from Story 2), create a named axial code, assign both open-code
labels to it, restart `evalctl analyze`, and confirm the axial code and its
membership are still present.

**Acceptance Scenarios**:

1. **Given** a list of distinct open-code labels, **When** the user creates
   a new axial code with a name (and optional description) — with or
   without assigning any open-code labels to it at creation time, since
   creating an axial code and assigning labels to it are separate actions —
   **Then** the grouping is saved immediately and the axial-code view shows
   each axial code with its member open codes and their combined trace
   count, counted as the number of distinct traces carrying any member
   label (a trace carrying two member labels of the same axial code is
   counted once, not twice).
2. **Given** an existing axial-code grouping, **When** the user reassigns an
   open code to a different axial code, renames an axial code, or deletes
   one, **Then** the sidecar file reflects the change and no annotation data
   (Story 2) is deleted as a side effect.
3. **Given** an open-code label with zero axial-code assignment, **When**
   viewing the axial-code view, **Then** it is visibly listed as
   "uncategorized" rather than silently omitted, so nothing is lost track
   of.

---

### User Story 4 - Get LLM-suggested axial-code groupings (Priority: P3)

A developer facing dozens of distinct open-code labels wants a starting-
point grouping suggestion instead of grouping every label by hand, while
keeping full control to accept, edit, or reject what's suggested.

**Why this priority**: A real time-saver at scale, but Story 3's manual
grouping is fully functional and sufficient on its own — this is additive,
not required for the feature to deliver its core value, and depends on an
optional external endpoint the user may not have configured.

**Independent Test**: With a configured LLM endpoint and a set of distinct
open-code labels, click "suggest with LLM," confirm a proposed grouping is
displayed as an editable draft (not yet saved), and confirm that dismissing
it without accepting leaves the existing axial-code state (Story 3)
unchanged.

**Acceptance Scenarios**:

1. **Given** an LLM endpoint is configured, **When** the user clicks
   "suggest with LLM" from the axial-code view, **Then** the tool sends only
   the distinct open-code labels not yet assigned to any axial code (and,
   where helpful, a sample of their attached notes) to the endpoint and
   displays a proposed set of named groupings for those labels as an
   editable draft, not yet persisted; labels already assigned to an axial
   code are excluded from the request and left untouched.
2. **Given** a displayed suggestion, **When** the user edits it and clicks
   "apply," **Then** the edited version (not the raw suggestion) is what
   gets saved to the sidecar file, following the same persistence path as
   manual grouping (Story 3).
3. **Given** no LLM endpoint is configured, **When** the user opens the
   axial-code view, **Then** the "suggest with LLM" action is visibly
   disabled/absent with an explanation, and manual grouping remains fully
   usable.
4. **Given** the LLM endpoint is unreachable, times out, or returns a
   malformed response, **When** "suggest with LLM" is clicked, **Then** the
   UI reports the failure clearly and manual grouping remains unaffected —
   this MUST NOT crash the analyze server or corrupt the sidecar file.

---

### Edge Cases

- A raw trace file or `SuiteRunReport` with two entries sharing the same
  trace/task id: rejected at load time with a clear error (consistent with
  `internal/dataset`'s existing duplicate-id handling), the same way a
  malformed suite dataset is today.
- A user tries to attach an open-code label to a trace that already carries
  that label after normalization (FR-008's trim/case-fold rule): rejected
  with a clear message pointing them to edit the existing annotation
  instead of creating a duplicate.
- An `evalctl analyze` invocation where the sidecar annotations file exists
  but references a trace id no longer present in the (re-supplied or
  edited) trace file: those orphaned annotations are preserved in the file
  and flagged in the UI as referencing a missing trace, never silently
  dropped.
- Two browser tabs/windows open against the same running `evalctl analyze`
  server, both editing annotations concurrently: server-side writes are
  serialized (last write wins per annotation/axial-code id); this is a
  single-local-user tool, not a multi-user collaboration surface — see
  Assumptions.
- The chosen port is already in use: `evalctl analyze` fails fast with a
  clear error rather than silently picking a different port.
- A very large trace set (thousands of traces): the UI must remain usable
  for browsing/annotating one trace at a time without requiring the whole
  set to be re-rendered per keystroke; exact performance targets are a
  planning-phase concern.
- `evalctl analyze` pointed at a `SuiteRunReport` produced by a version of
  `evalctl run` that predates this feature (i.e. one without persisted
  transcripts — see FR-001): fails fast with an actionable error naming the
  missing data, rather than rendering blank/empty outputs.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `evalctl run` MUST retain each task's original input and the
  evaluated system's full `Transcript` (output and, when present, steps) in
  the `SuiteRunReport` it can write via `--output`, since today's report
  only carries grading `Outcome`s (score/passed/reasoning), which have
  nothing for this feature to annotate. This is an additive change to the
  existing report shape, not a new artifact/command.
- **FR-002**: The system MUST provide a way to start a local, single-user
  web UI (a new CLI subcommand) pointed at either (a) a `SuiteRunReport`
  file (post FR-001) or (b) a standalone raw trace file, and treat both as
  equally valid input without different setup steps for the user.
- **FR-003**: The system MUST define a standalone raw trace file format
  (independent of any suite run), serialized as JSONL (one JSON trace
  object per line, no enclosing array), carrying, per trace: a unique id,
  the input given to the system, the system's output, and optionally an
  ordered sequence of trajectory steps — so traces that never went through
  `evalctl run` can still be analyzed.
- **FR-004**: The system MUST let a user attach a free-text open-code label,
  with an optional note, to a specific trace, and MUST support attaching
  more than one (distinct) open code to the same trace. A trace MUST NOT
  carry the same normalized label (per the FR-008 comparison rule) more
  than once: an attempt to add an already-present label to a trace MUST be
  rejected with a clear message directing the user to edit the existing
  annotation instead.
- **FR-005**: The system MUST let a user edit or delete a previously
  recorded open-code annotation.
- **FR-006**: The system MUST persist every open-code annotation, axial
  code, and axial-code-to-open-code assignment to a local JSON sidecar
  file immediately on creation/edit/deletion (write-through), so no
  explicit "save" action is required and no annotation is lost if the
  server process is killed between edits.
- **FR-007**: The system MUST reload prior annotation state from the
  sidecar file on startup if one already exists for the given trace input,
  rather than starting a fresh, empty annotation set every time.
- **FR-008**: The system MUST show, in aggregate, the distinct open-code
  labels recorded so far with a usage count per label, to support deciding
  how to group them into axial codes. Distinctness for this aggregation
  MUST use normalized comparison (leading/trailing whitespace trimmed,
  case-folded), not exact string match, so "Hallucinated Citation" and
  "hallucinated citation " count as the same label; the label shown MUST be
  the original casing as first entered. Because a trace can carry a given
  normalized label at most once (FR-004), this usage count is equivalently
  the number of distinct traces and the number of annotation instances
  carrying that label.
- **FR-009**: The system MUST let a user create a named axial code, with or
  without assigning any open-code labels to it at creation time (creating
  an axial code and assigning labels to it are separate actions, so a
  freshly created axial code MAY start with zero members), manually assign
  one or more distinct open-code labels to it, reassign an open code to a
  different axial code, and rename or delete an axial code, all without
  deleting the underlying open-code annotations (Story 2 data) those
  actions reference. Each distinct open-code label (per the
  normalized-comparison rule in FR-008) belongs to at most one axial code
  at a time; assigning it to a different axial code removes it from the one
  it was previously in. An axial code that loses its last member (via
  reassignment or open-code deletion), or that was created with none, is
  NOT auto-deleted — it remains visible in the axial-code view as an empty
  category until the user explicitly deletes it. An axial code's displayed
  trace count is the number of distinct traces carrying any of its member
  labels (a trace carrying two member labels of the same axial code is
  counted once, not twice).
- **FR-010**: The system MUST visibly distinguish open-code labels that
  have not yet been assigned to any axial code ("uncategorized") rather
  than omitting them from the axial-code view.
- **FR-011**: The system MAY offer an optional, explicitly user-triggered
  action that calls a configured LLM endpoint to propose an axial-code
  grouping of the current distinct open-code labels that are not yet
  assigned to any axial code (labels already assigned are excluded from the
  request and left unchanged, so a suggestion never reshuffles existing
  manual groupings); this suggestion MUST be shown as an editable draft and
  MUST NOT be persisted to the sidecar file unless and until the user
  explicitly applies it.
- **FR-012**: If no LLM endpoint is configured, the system MUST still
  provide fully functional manual axial-code grouping (FR-009/FR-010) with
  the LLM-assist action simply unavailable, not blocking or degrading the
  rest of the feature.
- **FR-013**: If the configured LLM endpoint is unreachable, times out, or
  returns a malformed response, the system MUST report the failure in the
  UI without crashing the analyze server or corrupting the sidecar file,
  consistent with how `internal/grader/judge` already treats judge-endpoint
  failures as reportable outcomes rather than fatal errors.
- **FR-014**: The system MUST reject, at load time, a trace input (either
  format) containing two or more entries sharing the same trace/task id,
  with a clear error naming the duplicate id, consistent with
  `internal/dataset`'s existing duplicate-id validation for suite datasets.
- **FR-015**: The system MUST be reachable and usable without any network
  access beyond localhost (aside from the optional, explicit LLM-assist
  call in FR-011), consistent with the project's offline-by-default,
  single-binary distribution model.
- **FR-016**: Starting the analyze UI MUST NOT require installing Node.js,
  npm, or any other runtime beyond what `evalctl` itself already requires,
  consistent with FR-016 of `specs/001-universal-eval-framework/spec.md`
  (single self-contained tool, no separate runtime dependency for the
  user).

### Key Entities

- **AnalysisTrace**: The unit this feature annotates — an id, the input
  given to the system, the system's output, and optional ordered steps.
  Sourced either by reading `Task`+`Transcript` pairs out of a (post
  FR-001) `SuiteRunReport`, or by parsing a standalone raw trace file
  directly into this shape.
- **OpenCodeAnnotation**: One free-text label (plus optional note) attached
  to one `AnalysisTrace`, with a stable id so it can be individually edited
  or deleted. Labels are compared for distinctness (aggregation, axial-code
  assignment) using a normalized form (trimmed, case-folded); the original
  casing as first entered is preserved for display. A given normalized
  label can be attached to a given trace at most once (FR-004).
- **AxialCode**: A named, user-defined category with an optional
  description, and a set of distinct open-code labels assigned to it. It
  may exist with zero members — either because it was created without any
  labels assigned (creation and assignment are separate actions) or
  because it lost its last member — and is not auto-deleted in either case.
  Membership is one-to-many at the *label* level (not per-annotation
  instance): each distinct open-code label belongs to at most one axial
  code at a time (or none, i.e. "uncategorized"), and grouping
  "hallucinated citation" into an axial code applies to every trace
  annotated with that label. Assigning a label to a different axial code
  removes it from its previous one. Its displayed trace count is the
  number of distinct traces carrying any of its member labels (a trace
  carrying two member labels of the same axial code is counted once).
- **AnnotationStore**: The JSON sidecar file holding all `OpenCodeAnnotation`s
  and `AxialCode`s for a given trace input file, written through on every
  mutation and reloaded on startup (FR-006/FR-007).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer with an existing `results.json` from `evalctl run
  --output` can start the analyze UI and see every task's input/output
  without any manual data-massaging step.
- **SC-002**: A developer with raw traces that never touched `evalctl` can
  analyze them through the same UI and workflow, with no feature gap
  relative to `SuiteRunReport`-sourced traces.
- **SC-003**: Annotation work (open codes and axial-code groupings) survives
  an unclean process kill immediately following the last edit, verified by
  restarting `evalctl analyze` against the same files and finding no lost
  annotation.
- **SC-004**: A developer can go from a loaded, unannotated trace set to a
  first complete axial-code grouping of all its open codes using only the
  UI — no hand-editing of the sidecar JSON file required (though it remains
  human-readable/diffable for those who want to).
- **SC-005**: Disabling or omitting the LLM-assist configuration entirely
  does not block or degrade any other part of the feature (Story 1–3 remain
  fully functional with zero external network calls).

## Assumptions

- This feature is single-local-user, single-machine, offline-first (aside
  from the explicit, optional LLM-assist call) — not a hosted/multi-user
  collaboration surface. Concurrent access (e.g. two tabs) is handled with
  simple last-write-wins semantics, not conflict resolution or presence
  indicators.
- Annotation granularity is trace-level (a whole trace's output), not
  span-level (highlighting a substring of the output) or step-level (a
  single trajectory step). Finer-grained annotation is a plausible future
  extension but not required for this feature to deliver value, since even
  trace-level open coding is a strict improvement over no tooling today.
- Turning axial codes into a grader/`llm_judge` rubric, a suite scaffold,
  or a `calibrate` input is explicitly out of scope for this feature (per
  clarification) and left for a later feature once axial-code output from
  real usage shows what shape a scaffold should take.
- Statistical inter-rater reliability (e.g. multiple humans coding the same
  traces and measuring agreement) is out of scope here; `evalctl
  calibrate` already exists for comparing two finished `Outcome[]` sets and
  is orthogonal to this feature's single-coder annotation workflow.
- No authentication/authorization is in scope — the analyze server binds to
  localhost for a single trusted local user, consistent with the project
  having no auth story anywhere else today.
- The exact sidecar JSON schema, HTTP API surface, package layout, and
  frontend implementation approach are left for the planning phase; what's
  binding here is the Functional Requirements and Success Criteria above.
  (`.ai/new-features/trace-analysis-ui.md` retains an initial non-binding
  design sketch as input for that phase.)
