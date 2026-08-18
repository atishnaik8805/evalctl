# Implementation Plan: Universal Evaluation Framework

**Branch**: `001-universal-eval-framework` | **Date**: 2026-08-15 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/001-universal-eval-framework/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command; its definition describes the execution workflow.

## Summary

A single-binary, language-agnostic CLI (`evalctl`) that evaluates any
external program capable of speaking a versioned stdio JSON protocol,
without ever importing that program's code. It owns dataset loading
(JSONL/CSV/JSON), four built-in graders plus arbitrary custom graders
(same stdio contract), a human-vs-model calibration comparator, and
suite orchestration with bounded task concurrency and CI-style pass/fail
gating. Built in Go as a dependency-light static binary; consumers and
custom graders bring their own runtime, the framework brings none. Three
implementation-detail questions left open by the spec (subprocess/HTTP
timeout policy, `llm_judge`'s wire contract, concurrency config
granularity) are resolved in research.md.

## Technical Context

**Language/Version**: Go 1.22+

**Primary Dependencies**: Go standard library (`os/exec`, `net/http`,
`encoding/json`, `encoding/csv`, `flag`, `context`) for everything except
YAML suite-config parsing, where `gopkg.in/yaml.v3` is added as the one
external dependency — the stdlib has no YAML support and suite configs
must accept YAML per FR-005. No CLI framework (cobra, etc.) — `flag` plus
a small manual subcommand dispatcher is sufficient for four commands and
keeps the dependency surface minimal, matching FR-016's "dependency-free"
distribution goal (that goal is about the *shipped binary having no
runtime dependency for its user*, not zero Go module dependencies).

**Storage**: N/A — stateless CLI; all state is suite config files,
dataset files, and optional `--output` result files on local disk. No
database, no daemon, no persistent process between invocations.

**Testing**: Go stdlib `testing` package, table-driven tests, colocated
`_test.go` files per package. Integration tests use small fixture
executables (shell/Python scripts) checked into `tests/fixtures/` to
exercise the real stdio protocol end-to-end (per quickstart.md), since
the protocol boundary is the framework's core value and must be tested
as a real subprocess call, not mocked.

**Target Platform**: Cross-platform CLI — Linux, macOS, Windows (amd64 +
arm64), each a single static binary produced by `go build`. No
platform-specific behavior beyond subprocess invocation, which Go's
`os/exec` abstracts.

**Project Type**: Single project — CLI tool (Go idiomatic layout, not
the Python-oriented `src/`+`tests/` template split).

**Performance Goals**: I/O-bound by design (LLM/consumer latency
dominates per the spec's Assumptions) — no throughput target beyond
"bounded worker pool keeps `concurrency` external calls in flight
without CPU-bound overhead from the framework itself."

**Constraints**: Offline/suite-driven only (no live traffic evaluation,
per Non-Goals). One-process-per-task subprocess overhead is accepted
(Assumptions). Deterministic pass/fail gating across repeated runs on
unchanged inputs (SC-004) — no automatic retries that could introduce
run-to-run flakiness (research.md #1).

**Scale/Scope**: Datasets in the tens-to-low-thousands of tasks per
suite run; 4 built-in graders + unlimited custom graders; single suite
per `evalctl run` invocation (no multi-suite batch orchestration in v1).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

`.specify/memory/constitution.md` contains only unfilled template
placeholders (`[PRINCIPLE_1_NAME]`, etc.) — no principles have been
ratified for this project yet. There are no gates to evaluate against.
This gate passes vacuously; nothing in this plan is exempted from a rule
because no rule exists. If the constitution is ratified later, this plan
should be re-checked against it.

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
cmd/
└── evalctl/
    └── main.go              # CLI entrypoint, subcommand dispatch (run/calibrate/validate/list-graders)

internal/
├── protocol/                # Task, Transcript, Step, Outcome types + JSON (de)serialization; protocol_version handling
│   └── protocol_test.go
├── dataset/                 # JSONL/CSV/JSON loaders → []Task; dedup + malformed-row validation (FR-007)
│   └── dataset_test.go
├── suite/                   # Suite/GraderConfig YAML+JSON parsing, schema validation (contracts/suite-config-schema.md)
│   └── suite_test.go
├── grader/
│   ├── builtin/              # exact_match, schema_validity, entity_grounding
│   │   └── builtin_test.go
│   ├── judge/                 # llm_judge HTTP client (contracts/judge-http-contract.md)
│   │   └── judge_test.go
│   └── external/               # subprocess-based custom grader invocation (shared with consumer invocation)
│       └── external_test.go
├── runner/                   # subprocess invocation for the evaluated system, bounded worker pool (FR-017), timeout handling
│   └── runner_test.go
├── calibrate/                 # human vs. model outcome comparison (FR-012)
│   └── calibrate_test.go
└── report/                    # SuiteRunReport/CalibrationReport formatting (stdout summary + --output JSON)
    └── report_test.go

tests/
├── integration/               # end-to-end: real evalctl binary + fixture consumer/grader executables
└── fixtures/                  # small consumer/grader scripts implementing the stdio protocol, used by quickstart.md and integration tests

go.mod
go.sum
```

**Structure Decision**: Single project, idiomatic Go CLI layout —
`cmd/evalctl` for the binary entrypoint, `internal/` packages split along
the spec's own seams (protocol contracts, dataset loading, suite config,
graders, runner, calibration, reporting) so each maps to a distinct
functional requirement group and can be tested in isolation with
colocated `_test.go` files. `tests/integration` exercises the real stdio
protocol against fixture executables rather than mocking subprocess
calls, since that boundary (FR-002) is the product's core value
proposition and must be proven with a real process, not simulated.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

Not applicable — no ratified constitution exists to violate (see
Constitution Check above).

## Post-Design Constitution Check

*Re-evaluated after Phase 1 (data-model.md, contracts/, quickstart.md).*

Unchanged from the pre-design check: the constitution file has no
ratified principles, so there is nothing to re-evaluate against. No new
complexity was introduced during design beyond what Technical Context
already accounts for (one external dependency — `gopkg.in/yaml.v3` — and
a single internal-package split mirroring the spec's own functional
groupings).
