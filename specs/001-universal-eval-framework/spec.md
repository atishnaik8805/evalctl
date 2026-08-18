# Feature Specification: Universal Evaluation Framework

**Feature Branch**: `[001-universal-eval-framework]`

**Created**: 2026-08-15

**Status**: Draft

**Input**: User description: "A standalone, language-agnostic evaluation framework for AI applications. Any system that can read JSON from stdin and write JSON to stdout can be evaluated by it, regardless of implementation language. The framework owns dataset management, grading, calibration (human vs. model-graded agreement), and suite orchestration; it never owns or imports the code of the thing it's evaluating. Ships as a single dependency-free CLI binary (`evalctl`)."

## Clarifications

### Session 2026-08-15

- Q: When a dataset contains two or more rows sharing the same task identifier, what should the framework do? → A: Reject the whole dataset at load/validate time (hard validation error, no run occurs)
- Q: When a grader itself fails to produce a valid outcome for a task (custom grader process crash/malformed output, or model-judge endpoint unreachable/timeout/malformed response), what should happen to that task's result? → A: Record a failing outcome for that task+grader pair (with an error reason) and continue the rest of the suite, consistent with FR-003
- Q: When loading a dataset, if a row is malformed or missing required fields, what should the framework do? → A: Reject the whole dataset at load/validate time — one malformed row fails the whole load, consistent with duplicate-ID handling
- Q: Should suite configuration files support putting secret values (like a model-judge endpoint's API key) directly in plain text, or should they be required to reference credentials indirectly? → A: Suite config MUST reference credentials indirectly (e.g., environment variable name) — literal secret values in the file are disallowed/not supported
- Q: When a suite mixes tasks that return trajectory steps with tasks that don't, what should a trajectory-aware grader do with a task that has no steps? → A: Trajectory-aware grader records a failing outcome for that task ("no trajectory data"), consistent with FR-018's grader-failure handling

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Evaluate a consumer's output quality without adopting the framework's language (Priority: P1)

A developer has a program (in any language) that produces a single output per input. They want to measure output quality without rewriting any part of their program in the evaluation tool's own language, and without installing that language's runtime.

**Why this priority**: This is the framework's core value proposition and the minimum slice that delivers value: language-agnostic evaluation via a declarative suite. Without this, nothing else in the spec matters.

**Independent Test**: Can be fully tested by writing a small script in a language other than the evaluation tool's implementation language that implements the stdio contract, pointing a suite configuration at it plus a dataset file and a built-in grader, and running the suite — it produces per-grader scores with zero dependency on the evaluated program's language being installed alongside the tool.

**Acceptance Scenarios**:

1. **Given** a working program that reads one task as JSON from standard input and writes one result as JSON to standard output, and a dataset file, **When** a suite referencing both is run, **Then** the tool produces an outcome and a per-grader average score for every task in the dataset.
2. **Given** a suite configured with a `fail_below` quality threshold, **When** the suite's overall score is below that threshold, **Then** the run reports failure (non-zero result); **When** the score is at or above threshold, **Then** the run reports success.
3. **Given** an evaluated program that exits with a non-zero code or returns text that isn't valid JSON for a given task, **When** the suite is run, **Then** that task is recorded as a failing outcome and the rest of the suite continues to completion.

---

### User Story 2 - Evaluate a multi-step agent trajectory (Priority: P2)

A developer has an agent-like system (in any language) that performs a sequence of state/action/result steps before producing a final output. They want the same evaluation tool to grade the trajectory, not just the final output.

**Why this priority**: Extends the core contract to a second major evaluation shape (trajectories) that many AI agent teams need, but is not required for the simplest single-output use case to deliver value.

**Independent Test**: Can be fully tested by implementing the same stdio contract with a response that includes a `steps` sequence, running a suite with a trajectory-aware grader, and confirming outcomes are produced using the same suite run process as single-output evaluation — no separate command or configuration path.

**Acceptance Scenarios**:

1. **Given** an evaluated program that returns a sequence of steps along with its output, **When** a suite with a trajectory-aware grader is run against it, **Then** the grader receives the full step sequence and produces an outcome referencing it.
2. **Given** one suite with a consumer that returns steps and another suite with a consumer that does not, **When** both are run, **Then** both complete successfully using the same run command and reporting format.

---

### User Story 3 - Establish trust in a model-based grader before relying on it (Priority: P3)

A team wants to know whether an automated (model-based) grader agrees closely enough with human judgment to be trusted for unsupervised use.

**Why this priority**: Valuable and differentiating, but only relevant once a team already has suites and graders running (Stories 1–2) and outcomes to compare — it's a trust-building step on top of the core evaluation loop, not a prerequisite for it.

**Independent Test**: Can be fully tested by hand-constructing a small set of human-produced outcomes and a matching set of model-produced outcomes for the same tasks, running the calibration comparison, and checking the reported agreement figures against independently hand-calculated values.

**Acceptance Scenarios**:

1. **Given** a set of human-graded outcomes and a set of model-graded outcomes sharing the same task identifiers, **When** the calibration comparison is run, **Then** it reports the mean absolute score difference, the exact pass/fail agreement rate, and the number of tasks compared.
2. **Given** human and model outcome sets with no overlapping task identifiers, **When** the calibration comparison is run, **Then** the tool reports zero comparable samples rather than an incorrect or misleading agreement figure.

---

### User Story 4 - Gate a CI pipeline on evaluation quality (Priority: P4)

A team wants regressions in output quality to block a pipeline automatically, the same way a failing test suite would.

**Why this priority**: High operational value once suites already exist, but it is a thin layer (a threshold check) on top of Story 1's run behavior rather than new evaluation capability — teams can adopt it last without losing core value.

**Independent Test**: Can be fully tested by running the same suite twice against known outcome data — once producing a score below the configured threshold and once at/above it — and confirming the run's success/failure result matches expectations both times, suitable for direct use as a pipeline gate.

**Acceptance Scenarios**:

1. **Given** a suite with a configured quality threshold wired into a pipeline step, **When** a code/prompt change causes the suite score to drop below threshold, **Then** the pipeline step fails.
2. **Given** the same setup, **When** quality is at or above threshold, **Then** the pipeline step succeeds, on every run with the same inputs (deterministic gating behavior).

---

### Edge Cases

- What happens when an evaluated program or grader process hangs and never returns a result for a task?
- A dataset row that is malformed or missing required fields: dataset loading treats this as a hard validation error — the dataset is rejected and no run occurs, consistent with duplicate task identifier handling (see FR-007).
- Two tasks in the same dataset sharing the same task identifier: dataset loading treats this as a hard validation error — the dataset is rejected and no run occurs (see FR-007).
- A grader process that itself crashes or returns malformed output for a task (as opposed to the evaluated system failing), or a model-judge endpoint that is unreachable, times out, or returns a response missing the expected score/reasoning fields: the system records a failing outcome for that task-grader pair with an error reason and continues the rest of the suite (see FR-018).
- A suite mixing tasks that return trajectory steps with tasks that don't, within the same run: a trajectory-aware grader applied to a task with no steps records a failing outcome for that task ("no trajectory data"), consistent with grader-failure handling (see FR-011, FR-018).
- What happens when `evalctl validate` is run against a suite whose dataset file doesn't exist or whose referenced grader name isn't recognized?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST define task, transcript (result), step, and outcome data using plain, language-neutral data structures, so no framework-specific types are required for a consumer to participate.
- **FR-002**: The system MUST evaluate any external program capable of reading one task as a line of JSON from standard input and writing one result as a line of JSON to standard output, regardless of the language that program is implemented in.
- **FR-003**: The system MUST treat a non-zero exit code or an unparseable/malformed response from an evaluated program as a task-level failure recorded as a failing outcome, without aborting the rest of the suite run.
- **FR-004**: The system MUST include a protocol version identifier in every request sent to an evaluated program or external grader, so the contract can change over time without silently breaking programs built against an earlier version.
- **FR-005**: The system MUST let a user fully define an evaluation suite — name, evaluated system, dataset location, and list of graders with their configuration — in a declarative configuration file, with no custom code required to define what gets evaluated.
- **FR-006**: The system MUST support an optional quality threshold in the suite configuration that, when the suite's score falls below it, causes the run to report failure (supporting automated pipeline gating); the run MUST report success when the score is at or above threshold.
- **FR-007**: The system MUST load evaluation datasets from at least JSONL, CSV, and JSON file formats, converting each record into a task, and MUST reject a dataset as invalid if two or more records share the same task identifier, or if any record is malformed or missing a required field, rather than silently deduplicating, skipping bad rows, or proceeding with ambiguous/incomplete data.
- **FR-008**: The system MUST provide built-in graders that run without spawning an external process: an exact-match grader (`exact_match`), a required-fields grader (`schema_validity`), and a known-entity grounding grader (`entity_grounding`).
- **FR-009**: The system MUST provide a built-in model-based judge grader that sends a rubric, task, and output to a user-configured evaluation endpoint and consumes back a score and reasoning, without being tied to any single model provider.
- **FR-010**: The system MUST allow a user to add a custom grader by pointing the suite configuration at any external program implementing the same evaluation contract, without requiring any change to the evaluation tool itself.
- **FR-011**: The system MUST evaluate both single-output results and multi-step trajectory results (a sequence of state/action/result steps) through the same suite run process, with no separate command or configuration path required. When a trajectory-aware grader is applied to a task whose transcript has no steps, the system MUST record a failing outcome for that task-grader pair (e.g., "no trajectory data"), consistent with FR-018's grader-failure handling, rather than silently skipping or crashing.
- **FR-012**: The system MUST compare a set of human-produced outcomes against a set of model-produced outcomes for shared task identifiers and report: mean absolute score difference, exact pass/fail agreement rate, and the number of tasks compared.
- **FR-013**: The system MUST let a user run a full suite and see a per-grader summary of results, with an option to save the complete results to a file.
- **FR-014**: The system MUST let a user validate that a suite configuration and its referenced dataset are well-formed without executing the evaluated program or any graders.
- **FR-015**: The system MUST let a user list the built-in graders and the configuration parameters each one accepts.
- **FR-016**: The system MUST be distributable and runnable as a single self-contained tool that does not require its user to install a separate language runtime to use it (evaluated systems and custom graders remain responsible for their own runtime).
- **FR-017**: The system MUST run the tasks within a suite concurrently, up to a configurable limit, since each task's evaluated-system call is independent of the others.
- **FR-018**: The system MUST record a failing outcome for the specific task-grader pair — with an error reason — when a custom grader process crashes or returns malformed output, or when a model-judge endpoint is unreachable, times out, or returns a response missing required score/reasoning fields, without aborting the rest of the suite run (consistent with FR-003's treatment of evaluated-system failures).
- **FR-019**: The system MUST require suite configuration to reference credentials for a model-judge endpoint (e.g., API keys) indirectly, such as by naming an environment variable, rather than accepting literal secret values embedded directly in the configuration file, so that a suite config checked into version control never carries credentials in plain text.

### Key Entities

- **Task**: A single unit of evaluation input — an identifier, the input given to the evaluated system, and arbitrary metadata (e.g., expected answers, known reference entities) used by graders.
- **Transcript**: The evaluated system's response to a task — its output, and optionally a sequence of steps when the system being evaluated is trajectory-based rather than single-output.
- **Step**: One state/action/result unit within a trajectory, ordered by index; present only for trajectory-based transcripts.
- **Outcome**: The result of grading one task with one grader — a score, an optional pass/fail verdict, optional reasoning, and supporting evidence.
- **Suite**: The declarative definition of one evaluation run — its name, the evaluated system, the dataset to use, the graders to apply, and optional pass/fail gating configuration.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer whose evaluated program is written in a different language than the evaluation tool itself can complete a full suite run without installing anything beyond their own program's language runtime.
- **SC-002**: Suites covering single-output systems and suites covering multi-step trajectory systems both complete successfully and produce comparable outcome reports, using the same setup and run steps.
- **SC-003**: A calibration report computed against a reference set of human- and model-graded outcomes matches independently hand-calculated agreement figures exactly.
- **SC-004**: A quality-gated run consistently reports failure when suite quality is below the configured threshold and consistently reports success when it is at or above threshold, across repeated runs on unchanged inputs.
- **SC-005**: A team can add a new grader or connect a new evaluated system to an existing installation of the tool with zero changes to the tool's own distributed binary or package.
- **SC-006**: A new team can go from nothing set up to a first completed suite run — dataset, suite configuration, and a minimal evaluated-system script — in under 30 minutes.

## Assumptions

- The pluggable simulator (synthetic task generation), a hosted/SaaS offering with live dashboards, a web UI, and real-time evaluation of live production traffic are explicitly out of scope for this feature (deferred to a later milestone) — this is v1, offline and suite-driven, CLI-only.
- Provider-specific integration details for any given LLM API (authentication, request shape beyond a common score/reasoning response) are the responsibility of the party operating the judge endpoint, not the framework.
- The framework treats the evaluated system and any custom graders strictly as external black-box processes it never imports code from; this is a governing constraint, not just an implementation preference.
- One-process-per-task invocation is assumed to be an acceptable overhead for the target workloads, since evaluated-system latency (e.g., LLM response time) is expected to dominate process startup cost; a lower-overhead persistent-process mode is explicitly deferred to a future version.
- The exact retry/timeout policy for slow or hanging evaluated-system and grader calls, the precise request/response shape of the model-judge HTTP contract, and where concurrency limits are configured (globally, per grader, or per suite) are left as open decisions to be resolved during the planning phase — they affect implementation detail and defaults, not the scope or user-facing behavior described above.
- Distribution as a single self-contained tool with no separate runtime dependency for the end user (FR-016) is treated as a hard product requirement; the specific technology used to build and ship it is an implementation decision made during planning, not part of this specification.
