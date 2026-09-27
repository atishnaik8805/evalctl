---

description: "Task list template for feature implementation"
---

# Tasks: Trace Analysis UI (Open & Axial Coding)

**Input**: Design documents from `/specs/002-trace-analysis-ui/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/ (http-api.md, raw-trace-format.md), quickstart.md

**Tests**: plan.md's Testing section commits to Go stdlib `testing`, colocated `_test.go` files per package, and `httptest`/`tests/integration` fixture-driven coverage as core to the design (frontend logic is kept thin enough that "all substantive logic ... is covered by Go tests server-side") — so test tasks are included throughout, matching the convention already established in `specs/001-universal-eval-framework/tasks.md`.

**Organization**: Tasks are grouped by user story (spec.md priorities P1–P3, four stories) to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: Which user story this task belongs to (US1, US2, US3, US4)
- File paths are exact, per plan.md's Project Structure

## Path Conventions

Single project, extending the existing Go CLI layout (plan.md):
- `cmd/evalctl/` — CLI entrypoint and per-command files
- `internal/analyze/` — this feature's new package (trace loading, annotation store, HTTP server, LLM-suggest client), with colocated `_test.go`, plus `internal/analyze/web/` (embedded static frontend)
- `internal/report/` — modified additively for FR-001
- `tests/integration/` — end-to-end tests against the real binary and a real sidecar file on a temp directory

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Package scaffolding — no new Go dependency, no build tooling change (go.mod is unchanged per plan.md)

- [X] T001 Create the `internal/analyze/` package directory and `internal/analyze/web/` subdirectory per plan.md Project Structure
- [X] T002 [P] Create `internal/analyze/web/embed.go` with a `//go:embed` directive exposing an `embed.FS` over `web/`, plus minimal placeholder `internal/analyze/web/index.html`, `internal/analyze/web/app.js`, `internal/analyze/web/style.css` so the package compiles (content is built out story-by-story below)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T003 Add a `Trace` struct (`TaskID`, `Input`, `Output`, `Steps`) and a `Traces []Trace` field to `SuiteRunReport`; update `BuildSuiteRunReport` to accept and copy a `traces []Trace` parameter (FR-001, research.md §2), in `internal/report/report.go`
- [X] T004 [P] Update `internal/report/report_test.go` for the new `Traces` field and `BuildSuiteRunReport` signature (depends on T003)
- [X] T005 Modify `runTask` to also return the produced `protocol.Transcript` (zero value on consumer failure) alongside the outcomes it already returns, and `runCommand` to accumulate a new `[]report.Trace` slice — appending a `Trace` only for tasks where a transcript was actually produced, using the task's own `Input` — then pass both outcomes and traces to `report.BuildSuiteRunReport` (research.md §2), in `cmd/evalctl/run.go` (depends on T003)
- [X] T006 Implement the `AnalysisTrace` type and `LoadTraces(path string) ([]AnalysisTrace, error)` in `internal/analyze/trace.go`: shape-sniffing dispatch (research.md §1 — whole-file unmarshal into `map[string]json.RawMessage`, dispatch to the `SuiteRunReport` path when an `"outcomes"` key is present, else the JSONL path), `SuiteRunReport.Traces` → `[]AnalysisTrace` extraction, JSONL line-by-line parsing per contracts/raw-trace-format.md (blank lines skipped, 1-based line-numbered errors), whole-file rejection on a missing `id`/`input`/`output` field, duplicate-id rejection naming the id (FR-014), and pre-FR-001 report detection — `len(Outcomes) > 0 && len(Traces) == 0` fails fast naming the file (research.md §8) (depends on T003)
- [X] T007 [P] Write `internal/analyze/trace_test.go` covering: `SuiteRunReport` dispatch, JSONL dispatch, the single-trace-object shape-sniffing edge case, missing-field rejection (each of `id`/`input`/`output`), duplicate-id rejection, and pre-FR-001 report detection (depends on T006)
- [X] T008 Implement the `AnnotationStore`, `OpenCodeAnnotation`, and `AxialCode` types (data-model.md), a label-normalization helper (trim whitespace, case-fold), and `Store.Load`/`Store.Save` — decode-if-exists-else-empty on load, temp-file-then-rename atomic write on save (research.md §6) — guarded by a `sync.Mutex`, in `internal/analyze/store.go`
- [X] T009 [P] Write `internal/analyze/store_test.go` covering: load from a nonexistent sidecar path (fresh empty store at counters 1/1), load-existing-file round-trip matching data-model.md's example shape, malformed-JSON/unrecognized-`version` rejection, and atomic save (temp file written, then renamed over the real path) (depends on T008)

**Checkpoint**: Foundation ready - user story implementation can now begin

---

## Phase 3: User Story 1 - Load a set of traces into the analysis UI (Priority: P1) 🎯 MVP

**Goal**: `evalctl analyze --traces <path>` starts a local web UI listing every trace by id, from either a `SuiteRunReport` or a standalone JSONL raw-trace file, rendering a selected trace's input/output (and steps, if present), with prior annotations reloaded if a sidecar file already exists.

**Independent Test**: Point `evalctl analyze` at a `SuiteRunReport` file and, separately, at a standalone raw-trace JSONL file; in both cases the UI lists every trace by id and renders one trace's input/output (and steps, if present) when selected, with no annotation actions required to pass this test.

### Implementation for User Story 1

- [X] T010 [US1] Implement HTTP handlers `GET /api/traces` (id-ordered summary list), `GET /api/traces/{id}` (detail + embedded `annotations`, `404` if unknown), and `GET /api/orphaned-annotations` per contracts/http-api.md, plus serving the embedded static frontend at `/`, wired on a `net/http.ServeMux`, in `internal/analyze/server.go` (depends on T002, T006, T008)
- [X] T011 [P] [US1] Write `internal/analyze/server_test.go` (`net/http/httptest`) covering `GET /api/traces`, `GET /api/traces/{id}` success and `404`, and `GET /api/orphaned-annotations` (depends on T010)
- [X] T012 [US1] Implement the `evalctl analyze` subcommand: `flag.NewFlagSet` with `--traces` (required), `--annotations` (default `<traces>.annotations.json`, research.md §9), `--addr` (default `localhost:8420`), `--llm-endpoint`, `--llm-api-key-env`; call `analyze.LoadTraces`, `Store.Load`, build the `analyze.Server`, `net.Listen`+serve, and fail fast (no retry on another port) when the listen fails, in `cmd/evalctl/analyze.go` (depends on T006, T008, T010)
- [X] T013 [US1] Wire `"analyze"` into `cmd/evalctl/main.go`'s subcommand dispatch and update `printUsage` with the new command's flags (depends on T012)
- [X] T014 [P] [US1] Implement the frontend trace list + trace detail view, including ordered step-sequence rendering when `steps` is present, fetching `GET /api/traces` and `GET /api/traces/{id}`, in `internal/analyze/web/index.html`, `internal/analyze/web/app.js`, `internal/analyze/web/style.css` (depends on T010)
- [X] T015 [US1] Write `tests/integration/analyze_test.go` covering quickstart.md Scenarios 1, 2, and 6: `SuiteRunReport`-sourced load, standalone-JSONL-sourced load (with a `steps`-bearing trace), duplicate-id rejection naming the id, and pre-FR-001 report rejection — against the real HTTP server and a real sidecar path on a temp directory (depends on T012)

**Checkpoint**: At this point, User Story 1 should be fully functional and testable independently (MVP)

---

## Phase 4: User Story 2 - Open-code annotate a trace's output (Priority: P1)

**Goal**: Attach, edit, and delete free-text open-code labels (with an optional note) on a trace, written through immediately to the sidecar file, with an aggregate "open codes" view showing distinct labels and their usage counts.

**Independent Test**: With a trace set loaded, add two open-code annotations to one trace and one to another, restart `evalctl analyze` against the same files, and confirm all three annotations are still present and attributed to the correct trace.

### Implementation for User Story 2

- [X] T016 [US2] Implement `OpenCodeAnnotation` mutation methods on `Store` — `CreateAnnotation` (FR-004: reject an already-present normalized label on that trace, naming the existing annotation's id), `UpdateAnnotation` (same duplicate check against the trace's *other* annotations), `DeleteAnnotation` — with sequential `oc-<n>` id generation (never reused) and a `Save` call after each mutation, in `internal/analyze/store.go` (depends on T008)
- [X] T017 [US2] Implement the aggregated open-codes view (FR-008: group by `normalized_label`, display the earliest-created annotation's `label` casing, count distinct `trace_id`s) as a `Store` method in `internal/analyze/store.go` (depends on T016)
- [X] T018 [P] [US2] Extend `internal/analyze/store_test.go` with tests for `CreateAnnotation`/`UpdateAnnotation`/`DeleteAnnotation` (including duplicate-label-on-trace rejection and edit-into-duplicate rejection) and the open-codes aggregation view (first-used casing, distinct-trace count) (depends on T017)
- [X] T019 [US2] Implement HTTP handlers `POST /api/traces/{id}/annotations` (`404`/`400`/`409` per contracts/http-api.md, `201` on success), `PUT /api/annotations/{id}` (`404`/`400`/`409`, `200`), `DELETE /api/annotations/{id}` (`404`/`204`), and `GET /api/open-codes`, in `internal/analyze/server.go` (depends on T016, T017)
- [X] T020 [P] [US2] Extend `internal/analyze/server_test.go` covering create/edit/delete annotation (including the `409` duplicate body with `existing_annotation_id`, and `404` cases) and `GET /api/open-codes` (depends on T019)
- [X] T021 [US2] Implement the frontend open-code annotate form (add/edit/delete a label+note on the selected trace) and the aggregate open-codes list view, in `internal/analyze/web/index.html`, `internal/analyze/web/app.js`, `internal/analyze/web/style.css` (depends on T014, T019)
- [X] T022 [US2] Extend `tests/integration/analyze_test.go` with quickstart.md Scenario 3: add annotations, verify the sidecar file reflects them immediately (write-through), kill the server unclean, restart against the same files, confirm no annotation lost, and check `GET /api/open-codes` counts (depends on T019)

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently

---

## Phase 5: User Story 3 - Group open codes into axial codes (Priority: P2)

**Goal**: Create named axial codes (with or without members at creation time), manually assign/reassign/unassign distinct open-code labels to them with single membership, rename/delete axial codes, and see each axial code's distinct-trace count — all without ever deleting the underlying open-code annotations.

**Independent Test**: With at least two distinct open-code labels already recorded (from Story 2), create a named axial code, assign both open-code labels to it, restart `evalctl analyze`, and confirm the axial code and its membership are still present.

### Implementation for User Story 3

- [X] T023 [US3] Implement `AxialCode` mutation methods on `Store` — `CreateAxialCode` (`name` required, `member_labels` optional per FR-009), `RenameAxialCode` (name/description only), `AssignMember` (single-membership: atomically removes the label from whichever axial code it currently belongs to), `UnassignMember` (label reverts to uncategorized), `DeleteAxialCode` (members revert to uncategorized; underlying `OpenCodeAnnotation`s untouched) — with sequential `ax-<n>` id generation and a `Save` call after each mutation, in `internal/analyze/store.go` (depends on T008, T016)
- [X] T024 [US3] Implement the `AxialCode` aggregation view (`GET /api/axial-codes` shape: `member_labels` + `trace_count` as the distinct-trace union across member labels, per data-model.md's Derived rule) and extend the open-codes aggregation to include `axial_code_id` (`null` when uncategorized, FR-010) as `Store` methods in `internal/analyze/store.go` (depends on T023, T017)
- [X] T025 [P] [US3] Extend `internal/analyze/store_test.go` covering axial-code CRUD, single-membership reassignment (label moves atomically out of its prior axial code), empty-at-creation and empty-after-last-member-removed (not auto-deleted), delete-doesn't-cascade-to-annotations, the `trace_count` union computation (a trace with two member labels counted once), and uncategorized open-codes reflecting `axial_code_id: null` (depends on T024)
- [X] T026 [US3] Implement HTTP handlers `GET /api/axial-codes`, `POST /api/axial-codes` (`400` on empty name, `201`), `PUT /api/axial-codes/{id}` (rename/redescribe, `404`/`200`), `POST /api/axial-codes/{id}/members` (`404`/`200`), `DELETE /api/axial-codes/{id}/members/{normalized_label}` (`404`/`200`), `DELETE /api/axial-codes/{id}` (`404`/`204`) per contracts/http-api.md, in `internal/analyze/server.go` (depends on T023, T024)
- [X] T027 [P] [US3] Extend `internal/analyze/server_test.go` covering the axial-code endpoints' success and 4xx paths, including reassignment-via-members and unassign-then-uncategorized (depends on T026)
- [X] T028 [US3] Implement the frontend axial-codes board — create/rename/delete an axial code, assign/unassign open-code labels (with single-membership move reflected in the UI), and a visible "uncategorized" section (FR-010) — in `internal/analyze/web/index.html`, `internal/analyze/web/app.js`, `internal/analyze/web/style.css` (depends on T021, T026)
- [X] T029 [US3] Extend `tests/integration/analyze_test.go` with quickstart.md Scenario 4: create an empty axial code, assign a member, confirm the other label still shows uncategorized, restart and confirm persistence, reassign a label into a new axial code then delete that axial code and confirm the label reverts to uncategorized (not deleted) while its underlying annotation is still present via `GET /api/traces/{id}` (depends on T026)

**Checkpoint**: All user stories 1-3 should now be independently functional

---

## Phase 6: User Story 4 - Get LLM-suggested axial-code groupings (Priority: P3)

**Goal**: An optional, explicitly-triggered "suggest with LLM" action proposes a draft axial-code grouping over currently-uncategorized open-code labels, shown as an editable draft never auto-applied or persisted, with fully functional manual grouping whether or not an LLM endpoint is configured.

**Independent Test**: With a configured LLM endpoint and a set of distinct open-code labels, click "suggest with LLM," confirm a proposed grouping is displayed as an editable draft (not yet saved), and confirm that dismissing it without accepting leaves the existing axial-code state (Story 3) unchanged.

### Implementation for User Story 4

- [X] T030 [US4] Implement `internal/analyze/suggest.go`: an HTTP client that `POST`s `{protocol_version, labels: [{label, sample_notes: [...]}]}` — built only from currently-uncategorized normalized labels — to the configured `--llm-endpoint`, sets `Authorization: Bearer <env var value>` when `--llm-api-key-env` is set, bounds the call with `context.WithTimeout`, and parses `{"groupings": [{"name", "labels"}]}`, returning a descriptive error (never panicking) on any transport error, non-2xx, or malformed response (research.md §7) (depends on T008, T024)
- [X] T031 [P] [US4] Write `internal/analyze/suggest_test.go` (`httptest` mock endpoint) covering the happy path, a non-2xx response, a malformed-JSON response, and a timeout (depends on T030)
- [X] T032 [US4] Implement HTTP handlers `GET /api/config` (`{"llm_suggest_available": bool}`) and `POST /api/axial-codes/suggest` — registered on the mux only when `--llm-endpoint` is configured (FR-012), builds its label set from the current uncategorized labels with no request body, returns the proposed groupings, never writes to the sidecar file, and returns `502` with `{"error": "..."}` on any `suggest.go` failure (FR-013) — in `internal/analyze/server.go` (depends on T030)
- [X] T033 [US4] Wire `--llm-endpoint`/`--llm-api-key-env` from `cmd/evalctl/analyze.go` into `Server` construction so `/api/axial-codes/suggest` is conditionally registered (depends on T012, T032)
- [X] T034 [P] [US4] Extend `internal/analyze/server_test.go` covering both `GET /api/config` states, `POST /api/axial-codes/suggest` success and `502`, and the route being absent (mux `404`) when no `--llm-endpoint` was configured (depends on T032, T033)
- [X] T035 [US4] Implement the frontend "suggest with LLM" control — hidden/disabled with an explanation per `GET /api/config` (FR-012), editable-draft rendering from `POST /api/axial-codes/suggest`, an "apply" action that issues normal `POST /api/axial-codes`/`POST /api/axial-codes/{id}/members` calls with the (possibly edited) result, and a clear in-UI failure message on `502` — in `internal/analyze/web/index.html`, `internal/analyze/web/app.js`, `internal/analyze/web/style.css` (depends on T028, T032)
- [X] T036 [US4] Extend `tests/integration/analyze_test.go` with quickstart.md Scenario 5: start a fixture LLM-suggest HTTP server, confirm the draft is returned but the sidecar file is untouched, confirm an edited draft's name is what gets persisted on apply, stop the fixture server and confirm a clear failure with the server still responsive and the sidecar file unchanged, then restart `evalctl analyze` without `--llm-endpoint` and confirm `GET /api/config` reports `false` while manual grouping (Scenario 4's flow) still works (depends on T033)

**Checkpoint**: All four user stories independently functional

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [X] T037 [P] Run the full quickstart.md walkthrough (Scenarios 1-6) against the built binary end-to-end and fix any discrepancies found
- [X] T038 [P] Add a package-level godoc comment to `internal/analyze` summarizing its open-coding/axial-coding responsibilities
- [X] T039 [P] Document `evalctl analyze` usage (flags, default port, sidecar file naming convention) in `README.md`
- [X] T040 Run `go vet` and `gofmt -l` across the repository and resolve all findings

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - US1 and US2 (both P1) should be built first, in order, since US2's `Store` mutation methods (T016) build on the same file as US1's read path and the frontend (T014/T021) is additive to the same files
  - US3 (P2) depends on US2's annotation mutation methods existing (T016) for single-membership bookkeeping
  - US4 (P3) depends on US3's axial-code aggregation (T024) since a suggestion is applied via the same create/assign endpoints
- **Polish (Final Phase)**: Depends on all four user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - no dependency on other stories; delivers the MVP slice (browse-only)
- **User Story 2 (P1)**: Can start after Foundational; its frontend work (T021) builds on US1's frontend files (T014) and its handlers (T019) share `internal/analyze/server.go` with US1's (T010) - built after US1 for a working browse UI to extend, but independently testable per its own Independent Test
- **User Story 3 (P2)**: Depends on US2's `OpenCodeAnnotation` machinery existing (open-code labels must exist to group) - not independently buildable before US2
- **User Story 4 (P3)**: Depends on US3's axial-code create/assign endpoints (a suggestion is only useful once axial codes exist to apply into) - purely additive on top, never required for US1-US3 to work (FR-012)

### Within Each User Story

- `Store` mutation/aggregation methods before the HTTP handlers that call them
- HTTP handlers before the frontend code that fetches them
- Server-side implementation and its `_test.go` before the cross-cutting integration-test extension for that story
- Story complete before moving to the next priority

### Parallel Opportunities

- T002 (Setup) can run alongside T001 once the directory exists
- T004 (report test) can be written in parallel with T005 once T003 lands; T007/T009 similarly parallel with the next task once T006/T008 land
- Within each story, the `*_test.go` extension task is marked `[P]` (different file from the implementation it tests) per the convention already used in `specs/001-universal-eval-framework/tasks.md`
- Frontend tasks (T014, T021, T028, T035) are sequential across stories (same three files, additive), not parallel with each other
- Within Polish: T037, T038, T039 are `[P]`

---

## Parallel Example: User Story 2

```bash
# Once T016/T017 (Store mutation + aggregation) land, these can proceed together:
Task: "Extend internal/analyze/store_test.go with CreateAnnotation/UpdateAnnotation/DeleteAnnotation and open-codes aggregation tests"
Task: "Implement POST/PUT/DELETE annotation handlers and GET /api/open-codes in internal/analyze/server.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: run quickstart.md Scenarios 1, 2, and 6 against the built binary; confirm both `SuiteRunReport` and standalone-JSONL traces browse identically and load-time validation fails fast and clearly
5. Demo the MVP: browse traces from either input shape, no annotation actions yet

### Incremental Delivery

1. Setup + Foundational → foundation ready
2. Add User Story 1 → test independently → MVP demo (browse traces)
3. Add User Story 2 → test independently → open-code annotation demo (write-through + restart survival)
4. Add User Story 3 → test independently → axial-code grouping demo
5. Add User Story 4 → test independently → LLM-assist demo (additive, never blocking)
6. Polish → full quickstart validation, godoc, README, lint cleanup

### Parallel Team Strategy

With multiple contributors, after Foundational (Phase 2) completes:
- Contributor A: User Story 1 (Phase 3) - MVP path
- User Story 2 (Phase 4) should follow User Story 1 since it extends the same `server.go`/frontend files
- User Story 3 (Phase 5) should follow User Story 2 since it needs open-code labels to exist
- User Story 4 (Phase 6) should follow User Story 3 since a suggestion is applied through US3's endpoints

---

## Notes

- `[P]` tasks = different files, no dependencies on incomplete tasks (test-extension tasks are marked `[P]` relative to the implementation they cover, matching this repo's `001` convention)
- `[Story]` label maps task to specific user story for traceability
- Every mutating `Store` method ends with a `Save` call (write-through, FR-006) - there is no batching/debounce anywhere in this feature
- Verify each story's integration-test extension independently before moving to the next story's phase
- Commit after each task or logical group
- Avoid: vague tasks, same-file conflicts marked `[P]`, cross-story dependencies that break independent testability
