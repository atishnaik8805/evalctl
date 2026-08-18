---

description: "Task list template for feature implementation"
---

# Tasks: Universal Evaluation Framework

**Input**: Design documents from `/specs/001-universal-eval-framework/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/ (stdio-protocol.md, judge-http-contract.md, suite-config-schema.md, cli-commands.md), quickstart.md

**Tests**: plan.md's Testing section commits to Go stdlib `testing`, colocated `_test.go` files per package, and `tests/integration` fixture-driven tests as core to the design (the stdio protocol boundary is called out as the product's core value that "must be tested as a real subprocess call, not mocked") — so test tasks are included throughout, not TDD-gated but written alongside their implementation.

**Organization**: Tasks are grouped by user story (spec.md priorities P1–P4) to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: Which user story this task belongs to (US1, US2, US3, US4)
- File paths are exact, per plan.md's Project Structure (Go idiomatic layout, `cmd/evalctl` + `internal/*`)

## Path Conventions

Single project, idiomatic Go CLI layout (plan.md):
- `cmd/evalctl/` — CLI entrypoint and per-command files
- `internal/{protocol,dataset,suite,grader/builtin,grader/judge,grader/external,runner,calibrate,report}/` — packages with colocated `_test.go`
- `tests/integration/`, `tests/fixtures/` — end-to-end tests against the real binary and fixture executables

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [X] T001 Create project directory structure per plan.md: `cmd/evalctl/`, `internal/protocol/`, `internal/dataset/`, `internal/suite/`, `internal/grader/builtin/`, `internal/grader/judge/`, `internal/grader/external/`, `internal/runner/`, `internal/calibrate/`, `internal/report/`, `tests/integration/`, `tests/fixtures/`
- [X] T002 Initialize `go.mod` (module path, `go 1.22`) and add `gopkg.in/yaml.v3` as the sole external dependency, in `go.mod` / `go.sum`
- [X] T003 [P] Add a build script (`Makefile` or `build.sh`/`build.ps1`) that cross-compiles static `evalctl` binaries for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, windows/arm64 with `CGO_ENABLED=0`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T004 [P] Implement `Task`, `Transcript`, `Step`, `Outcome` types with JSON (de)serialization and `protocol_version` handling (data-model.md, contracts/stdio-protocol.md) in `internal/protocol/protocol.go`
- [X] T005 [P] Write protocol package tests (marshal/unmarshal round-trips, required-field validation) in `internal/protocol/protocol_test.go`
- [X] T006 Implement JSONL/CSV/JSON dataset loaders converting records into `[]Task`, enforcing hard validation (whole-dataset rejection) on duplicate `id` or malformed/missing-field records per FR-007 in `internal/dataset/dataset.go` (depends on T004)
- [X] T007 [P] Write dataset package tests covering all three formats, duplicate-id rejection, and malformed-row rejection in `internal/dataset/dataset_test.go`
- [X] T008 Implement `Suite`/`GraderConfig` YAML+JSON parsing and schema validation per contracts/suite-config-schema.md, including the FR-019 literal-credential rejection check, in `internal/suite/suite.go` (depends on T004)
- [X] T009 [P] Write suite package tests (valid/invalid configs, missing required fields, literal-credential detection) in `internal/suite/suite_test.go`
- [X] T010 Implement bounded-concurrency subprocess runner: spawn one process per task, write one JSON line to stdin then close it, read one JSON line from stdout, enforce `timeout_seconds`, classify non-zero exit / malformed JSON / timeout into structured error reasons (FR-002, FR-003, FR-017, contracts/stdio-protocol.md) in `internal/runner/runner.go` (depends on T004)
- [X] T011 [P] Write runner package tests (success path, non-zero exit, malformed stdout, timeout, worker-pool concurrency bound) in `internal/runner/runner_test.go`
- [X] T012 Create CLI entrypoint with subcommand dispatcher for `run`/`calibrate`/`validate`/`list-graders` (flag-based, no external CLI framework) in `cmd/evalctl/main.go` (depends on T008)

**Checkpoint**: Foundation ready - user story implementation can now begin

---

## Phase 3: User Story 1 - Evaluate a consumer's output quality without adopting the framework's language (Priority: P1) 🎯 MVP

**Goal**: Run a full suite against a single-output evaluated system (any language) using built-in graders, with CI-gate exit codes.

**Independent Test**: Write a fixture consumer in a language other than Go implementing the stdio contract, point a suite config at it plus a dataset and a built-in grader, run `evalctl run`, and confirm per-grader scores are produced with zero dependency on Go being installed for the consumer.

### Implementation for User Story 1

- [X] T013 [P] [US1] Create fixture consumer script (single-output, echoes `task.input` uppercased) implementing contracts/stdio-protocol.md in `tests/fixtures/consumer_uppercase.py`
- [X] T014 [P] [US1] Create fixture consumer script that exits non-zero for one specific task id (for FR-003 failure-handling test) in `tests/fixtures/consumer_fails_on_id.py`
- [X] T015 [P] [US1] Implement `exact_match` built-in grader in `internal/grader/builtin/exact_match.go`
- [X] T016 [P] [US1] Implement `schema_validity` (required-fields) built-in grader in `internal/grader/builtin/schema_validity.go`
- [X] T017 [P] [US1] Implement `entity_grounding` (known-entity) built-in grader in `internal/grader/builtin/entity_grounding.go`
- [X] T018 [P] [US1] Implement `llm_judge` HTTP grader client per contracts/judge-http-contract.md (request build, `api_key_env` bearer auth, response parsing, `[0.0,1.0]` score validation, timeout/error classification into `judge_endpoint_error`/`timeout`) in `internal/grader/judge/judge.go`
- [X] T019 [P] [US1] Write builtin graders tests (table-driven, all four graders) in `internal/grader/builtin/builtin_test.go` and `internal/grader/judge/judge_test.go` (judge tests use an `httptest` mock server)
- [X] T020 [US1] Implement `SuiteRunReport` aggregation (per-grader mean score, `overall_score`, `passed` via `fail_below`) and stdout/`--output` JSON formatting in `internal/report/report.go` (depends on T015, T016, T017, T018)
- [X] T021 [US1] Implement `evalctl run` command orchestration: load suite + dataset, invoke `system` per task via the runner with bounded `concurrency`, record failing outcomes for consumer errors per FR-003, run each configured built-in grader per task, aggregate via `internal/report`, write `--output` file, and set process exit code per `fail_below` (FR-006, SC-004) in `cmd/evalctl/run.go` (depends on T006, T010, T020)
- [X] T022 [US1] Implement `evalctl validate` command: run all contracts/suite-config-schema.md checks (file parses, required fields, `system` path exists, dataset loads cleanly, grader names recognized, no literal credentials) without spawning any process, in `cmd/evalctl/validate.go` (depends on T008)
- [X] T023 [US1] Implement `evalctl list-graders` command printing the four built-in graders and their accepted `params` keys (FR-015) in `cmd/evalctl/list_graders.go` (depends on T015, T016, T017, T018)
- [X] T024 [US1] Write integration test: full suite run producing per-grader scores and correct `fail_below` pass/fail exit codes (both above- and below-threshold cases), plus consumer-failure handling (`error: "consumer_error"` recorded, rest of suite continues) in `tests/integration/run_test.go` (depends on T013, T014, T021)
- [X] T024a [P] [US1] Create fixture custom grader script (non-trajectory, checks `transcript.output` against `task.metadata.expected`) implementing the custom-grader half of contracts/stdio-protocol.md, in `tests/fixtures/grader_custom_exact.py`
- [X] T024b [US1] Write integration test: suite using only the custom grader from T024a against the Story-1 single-output consumer produces correct outcomes via `evalctl run` — proves FR-010's happy path independent of trajectory support, in `tests/integration/custom_grader_test.go` (depends on T013, T024a, T021)
- [X] T025 [US1] Write integration test: `evalctl validate` against a valid suite (exit 0) and against invalid suites — missing dataset file, unrecognized grader name, literal credential value — each failing with a distinct non-zero exit and identifying message, in `tests/integration/validate_test.go` (depends on T022)

**Checkpoint**: At this point, User Story 1 should be fully functional and testable independently (MVP)

---

## Phase 4: User Story 2 - Evaluate a multi-step agent trajectory (Priority: P2)

**Goal**: Grade multi-step agent trajectories through the same `evalctl run` path used for single-output consumers, with no separate command.

**Independent Test**: Implement the stdio contract with a response including a `steps` sequence, run a suite with a trajectory-aware custom grader via the same `evalctl run` command as Story 1, and confirm outcomes reference the step data.

### Implementation for User Story 2

- [X] T026 [P] [US2] Create fixture trajectory consumer script returning `steps: [...]` alongside `output` in `tests/fixtures/consumer_trajectory.py`
- [X] T027 [P] [US2] Create fixture trajectory-aware custom grader script that inspects `transcript.steps` and implements the custom-grader half of contracts/stdio-protocol.md in `tests/fixtures/grader_trajectory.py`
- [X] T028 [US2] Implement subprocess-based custom grader invocation (one JSON `Transcript`-in/`Outcome`-out call per task-grader pair, sharing the runner's stdin/stdout/timeout mechanism; classify non-zero exit / malformed JSON into `grader_error`/`timeout`) in `internal/grader/external/external.go` (depends on T010)
- [X] T029 [P] [US2] Write external grader package tests (success path, non-zero exit, malformed stdout, timeout) in `internal/grader/external/external_test.go`
- [X] T030 [US2] Extend `evalctl run` orchestration to invoke custom grader executables via `internal/grader/external`, and to record a failing outcome with reason `"no trajectory data"` when a trajectory-aware grader is applied to a `Transcript` with no `steps` (FR-011, FR-018) in `cmd/evalctl/run.go` (depends on T021, T028)
- [X] T031 [US2] Write integration test: suite with trajectory consumer + trajectory-aware custom grader succeeds via the same `evalctl run` command, with outcomes referencing step data; the same grader applied to a Story-1 (no-`steps`) transcript produces a failing outcome with `error: "no trajectory data"` in `tests/integration/trajectory_test.go` (depends on T026, T027, T030)

**Checkpoint**: At this point, User Stories 1 AND 2 both work independently

---

## Phase 5: User Story 3 - Establish trust in a model-based grader before relying on it (Priority: P3)

**Goal**: Compare human-produced and model-produced outcomes for shared task identifiers and report agreement figures.

**Independent Test**: Hand-construct human and model outcome sets for the same tasks, run `evalctl calibrate`, and check the reported figures against independently hand-calculated values.

### Implementation for User Story 3

- [X] T032 [P] [US3] Implement human-vs-model outcome comparison (`mean_absolute_diff`, `pass_fail_agreement`, `n_compared`, with `n_compared: 0` on zero task-id overlap) per FR-012 in `internal/calibrate/calibrate.go`
- [X] T033 [P] [US3] Write calibrate package tests (known agreement figures, zero-overlap case) in `internal/calibrate/calibrate_test.go`
- [X] T034 [US3] Implement `CalibrationReport` JSON formatting in `internal/report/report.go` (depends on T032)
- [X] T035 [US3] Implement `evalctl calibrate --human <path> --model <path>` command: load both `Outcome[]` JSON files, run the comparison, print the `CalibrationReport` as JSON, exit non-zero only on malformed input files in `cmd/evalctl/calibrate.go` (depends on T032, T034)
- [X] T036 [P] [US3] Create fixture `human.json`/`model.json` outcome files (5 shared task ids, hand-worked-out score differences) in `tests/fixtures/calibration/`
- [X] T037 [US3] Write integration test: `evalctl calibrate` against the overlapping fixture set matches hand-calculated `mean_absolute_diff`/`pass_fail_agreement`/`n_compared`, and against a zero-overlap `model.json` reports `n_compared: 0` (not an error) in `tests/integration/calibrate_test.go` (depends on T035, T036)

**Checkpoint**: All user stories should now be independently functional

---

## Phase 6: User Story 4 - Gate a CI pipeline on evaluation quality (Priority: P4)

**Goal**: Confirm the `fail_below` gating built in User Story 1 is deterministic and documented as a pipeline gate — a thin verification layer, not new evaluation capability (per spec.md's stated priority rationale).

**Independent Test**: Run the same suite twice against known outcome data — once below threshold, once at/above — and confirm the run's success/failure result matches expectations both times.

### Implementation for User Story 4

- [X] T038 [US4] Write integration test verifying deterministic CI gating: repeated `evalctl run` invocations of an unmodified suite produce identical exit codes and `passed` values, and the exit code flips correctly when `fail_below` is moved above/below the suite's `overall_score` (SC-004) in `tests/integration/gating_test.go` (depends on T021)
- [X] T039 [P] [US4] Document CI pipeline usage (exit-code contract of `evalctl run`, example CI step) in `README.md`

**Checkpoint**: All user stories independently functional; gating behavior proven deterministic

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [X] T040 [P] Run the full quickstart.md walkthrough (Stories 1–4) against the built binary end-to-end and fix any discrepancies found
- [X] T041 [P] Add package-level godoc comments to all `internal/*` packages
- [X] T042 [P] Verify cross-platform static builds from T003 run and produce working binaries with no CGO/dynamic-linking dependency, for all six target platforms
- [X] T043 Run `go vet` and `gofmt -l` across the repository and resolve all findings

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Setup — BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Foundational only — no dependency on other stories
- **User Story 2 (Phase 4)**: Depends on Foundational; extends US1's `evalctl run` orchestration (T021) and reuses the runner's subprocess mechanism (T010) — built after US1 for a working `run` command to extend, but independently testable per its own Independent Test
- **User Story 3 (Phase 5)**: Depends on Foundational only (protocol's `Outcome` type, T004) — no dependency on US1/US2 code paths; can be built in parallel with US1/US2 by a separate contributor
- **User Story 4 (Phase 6)**: Depends on US1's `fail_below`/exit-code logic (T021) already existing — purely a verification/documentation layer, no new production code
- **Polish (Phase 7)**: Depends on all desired user stories being complete

### User Story Dependencies

- **US1 (P1)**: No dependency on other stories — the MVP slice
- **US2 (P2)**: Extends US1's `run.go` orchestration file (T030 depends on T021); independently testable via its own suite/consumer/grader
- **US3 (P3)**: No dependency on US1/US2 — only needs Foundational's `Outcome` type; could be implemented in parallel with US1/US2
- **US4 (P4)**: Verification-only layer on top of US1; no dependency on US2/US3

### Within Each User Story

- Fixtures and independent package implementations first (marked `[P]`)
- Package tests alongside their implementation
- Aggregation/report logic before CLI command wiring
- CLI command wiring before integration tests that exercise it

### Parallel Opportunities

- All Setup `[P]` tasks (T003) run alongside T001/T002 once directories exist
- Foundational `[P]` tasks T004/T005 can start immediately; T007, T009, T011 run once their respective implementation task lands
- Within US1: T013–T019 (fixtures + all four graders + their tests) are all `[P]` — different files, no cross-dependencies
- Within US2: T026, T027, T029 are `[P]`
- Within US3: T032, T033, T036 are `[P]`
- US3 (calibration) has no code dependency on US1/US2 and can be staffed in parallel with them once Foundational is done
- Within Polish: T040, T041, T042 are `[P]`

---

## Parallel Example: User Story 1

```bash
# Launch fixtures and all four built-in graders together:
Task: "Create fixture consumer script in tests/fixtures/consumer_uppercase.py"
Task: "Create fixture consumer script (fails on one id) in tests/fixtures/consumer_fails_on_id.py"
Task: "Implement exact_match grader in internal/grader/builtin/exact_match.go"
Task: "Implement schema_validity grader in internal/grader/builtin/schema_validity.go"
Task: "Implement entity_grounding grader in internal/grader/builtin/entity_grounding.go"
Task: "Implement llm_judge grader in internal/grader/judge/judge.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: run `evalctl validate` + `evalctl run` against a real non-Go fixture consumer; confirm `fail_below` gating and consumer-failure handling both work (T024, T025)
5. Demo the MVP: language-agnostic single-output evaluation with CI gating

### Incremental Delivery

1. Setup + Foundational → foundation ready
2. Add User Story 1 → test independently → MVP demo
3. Add User Story 2 → test independently → trajectory support demo (same `run` command)
4. Add User Story 3 → test independently → calibration demo
5. Add User Story 4 → verify determinism → CI-gate documentation complete
6. Polish → cross-platform build verification, quickstart validation, lint cleanup

### Parallel Team Strategy

With multiple contributors, after Foundational (Phase 2) completes:
- Contributor A: User Story 1 (Phase 3) — MVP path
- Contributor B: User Story 3 (Phase 5) — no code dependency on US1/US2, can start immediately after Foundational
- User Story 2 (Phase 4) should follow User Story 1 since it extends `run.go`
- User Story 4 (Phase 6) should follow User Story 1 since it only verifies US1's gating logic

---

## Notes

- `[P]` tasks = different files, no dependencies on incomplete tasks
- `[Story]` label maps task to specific user story for traceability
- `internal/grader/external` (T028) is deliberately introduced in US2, not Foundational, per its Independent Test — but it shares the exact stdin/stdout/timeout mechanism built for the consumer runner (T010) per plan.md's own note that it is "shared with consumer invocation"
- Verify each story's integration test independently before moving to the next story's phase
- Commit after each task or logical group
- Avoid: vague tasks, same-file conflicts marked `[P]`, cross-story dependencies that break independent testability
