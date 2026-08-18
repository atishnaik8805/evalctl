# Data Model: Universal Evaluation Framework

All entities are plain JSON on the wire (FR-001) — this document describes
their shape and validation rules independent of the Go types that back
them internally.

## Task

A single unit of evaluation input.

| Field      | Type   | Required | Notes                                                        |
|------------|--------|----------|---------------------------------------------------------------|
| `id`       | string | yes      | Unique within a dataset (FR-007). Dedup violation → hard load error. |
| `input`    | any    | yes      | Passed verbatim to the evaluated system as part of the request. |
| `metadata` | object | no       | Arbitrary grader inputs, e.g. `expected`, `known_entities`, `required_fields`. Defaults to `{}`. |

**Validation** (FR-007, at dataset load time):
- Missing `id` or `input` → malformed record → whole dataset rejected.
- Duplicate `id` across two or more records in the same dataset → whole
  dataset rejected.
- No partial/best-effort load: either the whole dataset loads or none of it
  does.

## Transcript

The evaluated system's response to one `Task`.

| Field         | Type    | Required | Notes                                                     |
|---------------|---------|----------|------------------------------------------------------------|
| `task_id`     | string  | yes      | Must match the originating `Task.id`.                     |
| `output`       | any     | yes      | The system's single-output result.                        |
| `steps`       | Step[]  | no       | Present only for trajectory-based consumers (User Story 2). Absence means single-output. |
| `raw`         | object  | no       | Opaque passthrough for consumer-specific debug data; graders may ignore it. |

**Derived state**: a task-grader pair produces a **failing outcome**
(never a crash) when:
- the consumer subprocess exits non-zero, times out, or emits unparseable
  JSON (FR-003) → no `Transcript` is produced; a synthetic failing
  `Outcome` is recorded per configured grader with reason
  `"consumer_error"` / `"timeout"`.
- a trajectory-aware grader is applied to a `Transcript` with no `steps`
  → failing `Outcome`, reason `"no trajectory data"` (FR-011, FR-018).

## Step (trajectory only)

One state/action/result unit within a trajectory, ordered by index.

| Field    | Type | Required | Notes                          |
|----------|------|----------|----------------------------------|
| `index`  | int  | yes      | 0-based, strictly increasing within a `Transcript`. |
| `state`  | any  | yes      | Snapshot of state before the action. |
| `action` | any  | yes      | The action taken.                |
| `result` | any  | yes      | The outcome of taking the action. |

## Outcome

The result of grading one `Task` with one grader.

| Field         | Type          | Required | Notes                                                        |
|---------------|---------------|----------|----------------------------------------------------------------|
| `task_id`     | string        | yes      | The graded task's id.                                          |
| `grader_name` | string        | yes      | Which grader produced this outcome.                             |
| `score`       | float 0.0–1.0 | yes      | Normalized quality score.                                        |
| `passed`      | bool \| null  | no       | Pass/fail verdict, when the grader supports one; `null` if score-only. |
| `reasoning`   | string \| null| no       | Human-readable justification, when available (e.g. from `llm_judge`). |
| `evidence`    | object        | no       | Supporting data (e.g. matched entities, missing fields). Defaults to `{}`. |
| `error`       | string \| null| no       | Set (and `score` treated as `0.0`/`passed: false`) when this outcome represents a grader- or consumer-level failure (FR-003, FR-018). Reasons: `"consumer_error"`, `"grader_error"`, `"timeout"`, `"no trajectory data"`, `"judge_endpoint_error"`. |

## GraderConfig

One grader entry inside a `Suite`.

| Field    | Type   | Required | Notes                                                                 |
|----------|--------|----------|------------------------------------------------------------------------|
| `name`   | string | yes      | Built-in grader name (`exact_match`, `schema_validity`, `entity_grounding`, `llm_judge`) or a path to a custom grader executable. |
| `params` | object | no       | Grader-specific configuration. For `llm_judge`: `rubric`, `judge_endpoint`, `api_key_env` (FR-019). For `schema_validity`: `required_fields`. |

## Suite

The declarative definition of one evaluation run (FR-005).

| Field              | Type            | Required | Notes                                                                 |
|--------------------|-----------------|----------|--------------------------------------------------------------------------|
| `name`             | string          | yes      | Suite identifier, used in reports.                                       |
| `system`           | string          | yes      | Path to the evaluated system's executable.                               |
| `dataset`          | string          | yes      | Path to a JSONL/CSV/JSON dataset file.                                    |
| `graders`          | GraderConfig[]  | yes      | At least one grader.                                                      |
| `fail_below`       | float 0.0–1.0   | no       | CI gate threshold (FR-006). Omitted → run always reports success regardless of score. |
| `concurrency`      | int             | no       | Max tasks run concurrently (FR-017). Default: 5.                          |
| `timeout_seconds`  | int             | no       | Per-call timeout for consumer/grader subprocess calls and judge HTTP calls. Default: 30. |

**Validation** (`evalctl validate`, FR-014):
- `system` executable path must exist.
- `dataset` file must exist and load cleanly (see Task validation above).
- Every `graders[].name` must be a recognized built-in or a path to an
  existing executable.
- `llm_judge` grader entries must use `api_key_env` (or omit credentials
  entirely for unauthenticated endpoints) — a literal `api_key` value in
  the file is a validation error (FR-019).
- No subprocess is spawned and no grader runs during validation.

## CalibrationReport

Output of `evalctl calibrate` (FR-012).

| Field                    | Type   | Notes                                                        |
|--------------------------|--------|----------------------------------------------------------------|
| `mean_absolute_diff`     | float  | Mean of `\|human.score - model.score\|` over shared `task_id`s. |
| `pass_fail_agreement`    | float  | Fraction of shared `task_id`s where `human.passed == model.passed`. |
| `n_compared`             | int    | Count of shared `task_id`s. `0` when human/model sets don't overlap — never a misleading figure (Story 3, Scenario 2). |

## SuiteRunReport

Output of `evalctl run` (FR-013).

| Field           | Type              | Notes                                                    |
|-----------------|-------------------|-------------------------------------------------------------|
| `suite_name`    | string            | Echoed from `Suite.name`.                                    |
| `grader_scores` | map[string]float  | Per-grader mean score across all tasks.                       |
| `overall_score` | float             | Mean across all grader scores (compared against `fail_below`). |
| `outcomes`      | Outcome[]         | Full per-task-per-grader detail; written to `--output` file when given, always summarized to stdout. |
| `passed`        | bool              | `true` unless `fail_below` is set and `overall_score < fail_below`. |
