# Feature Specification: Universal Evaluation Framework

## Overview

A standalone, language-agnostic evaluation framework for AI applications. It is not tied to
any specific product — any system that can read JSON from stdin and write JSON to stdout can be
evaluated by it, regardless of what language that system is written in. The framework owns
dataset management, grading, calibration (human vs. model-graded agreement), and suite
orchestration; it never owns or imports the code of the thing it's evaluating.

## Problem Statement

Teams building AI features (generation tools, agents, RAG pipelines) need a way to catch
regressions and measure quality as prompts/models/data change — without hand-rolling eval infra
per project, and without every project needing to be written in the same language as the eval
tool. Existing tools (LangSmith, DeepEval, Promptfoo) solve this well within a single language
ecosystem (mostly Python/TS) but assume in-process integration, which forces every consumer into
that ecosystem.

## Goals

- A consumer in any language can be evaluated without writing a single line in the framework's
  implementation language.
- A suite (tasks + graders + target system) is fully defined declaratively (YAML/JSON) —
  no framework-specific code required to define what gets evaluated.
- Support both single-output evaluation (input → one output) and multi-step trajectory
  evaluation (input → sequence of state/action steps → output) under the same core contract.
- A calibration engine: given human-graded and model-graded outcomes on the same tasks, report
  an agreement score, so a model-based grader can be trusted (or not) before running unsupervised.
- Ship as a single binary with no runtime dependency required on the user's machine.

## Non-Goals (out of scope for this spec)

- The pluggable simulator (synthetic task generation) — later milestone, not in v1.
- A hosted/SaaS version, live dashboards, or a web UI — v1 is CLI-only.
- Live/online evaluation of production traffic in real time — v1 is offline, suite-driven.
- Built-in support for every possible LLM provider — v1 ships a generic HTTP-callable judge
  interface; provider-specific niceties are a consumer's own concern.

## User Scenarios

1. **A developer with a Python CLI tool** wants to eval its output quality. They write a ~10-line
   Python script implementing the stdio protocol, a YAML suite file, and a JSONL dataset. They
   run `evalctl run --suite suite.yaml` with no Python dependency on the framework itself.
2. **A developer with a Rust service** wants to eval an agent's tool-call trajectory. They
   implement the same stdio protocol in Rust, returning a trajectory (sequence of steps) instead
   of a single output. The same `evalctl` binary grades it, using a trajectory-aware grader.
3. **A team wants to trust an LLM-as-judge grader before relying on it.** They collect human
   scores on 20 tasks, run the model-judge on the same 20, and run `evalctl calibrate` to get an
   agreement report.
4. **A team wants to catch regressions in CI.** `evalctl run` returns a non-zero exit code if any
   grader's suite-level score drops below a configured threshold, so it can gate a pipeline.

## Functional Requirements

### FR1 — Core data contracts
- `Task`: `{id: string, input: any, metadata: object}`
- `Transcript`: `{task_id: string, output: any, steps?: Step[], raw?: object}` — `steps` present
  only for trajectory-based consumers; absent means single-output.
- `Step` (trajectory only): `{index: int, state: any, action: any, result: any}`
- `Outcome`: `{task_id: string, grader_name: string, score: float (0.0-1.0), passed: bool|null,
  reasoning: string|null, evidence: object}`
- `Suite`: `{name: string, system: string, dataset: string, graders: GraderConfig[]}`
- All contracts are plain JSON — language-agnostic by construction, no framework-specific types
  leak into the wire format.

### FR2 — Consumer protocol (the language boundary)
- The framework invokes a consumer as a subprocess per task (v1): writes one `Task` as a single
  line of JSON to the subprocess's stdin, reads one `Transcript` as a single line of JSON from
  stdout. Non-zero exit code or malformed JSON = task-level failure, recorded as an Outcome with
  `passed: false`, not a suite-level crash.
- Protocol version is included in every request (`{"protocol_version": "1.0", "task": {...}}`)
  so the contract can evolve without silently breaking old consumers.
- A future v2 MAY add a persistent-process HTTP mode for consumers wanting to avoid per-task
  process startup cost; v1 ships stdio-only.

### FR3 — Suite configuration (declarative)
- YAML or JSON file specifying: suite name, path to the consumer executable (`system`), path to
  the dataset file, and a list of graders with their config.
- Example:
  ```yaml
  name: my-suite
  system: ./run_my_app.py       # any executable implementing the stdio protocol
  dataset: tasks.jsonl
  graders:
    - name: schema_validity
      params: { required_fields: [title, steps] }
    - name: llm_judge
      params: { rubric: "Does the output fully address the input?", judge_endpoint: "http://localhost:8080/judge" }
  fail_below: 0.8               # optional — CI gate threshold
  ```

### FR4 — Dataset loaders
- Support JSONL, CSV, and JSON as input dataset formats. Each row/entry maps to a `Task`.
- Loader is a pure function: file path → `Task[]`. No product-specific parsing built in.

### FR5 — Built-in graders (ship in the binary, no external process needed)
- `exact_match` — output equals `task.metadata.expected`.
- `schema_validity` — output (parsed as JSON) contains all `required_fields`.
- `entity_grounding` — output references at least one entity from `task.metadata.known_entities`.
- `llm_judge` — generic model-based grader; calls a configurable HTTP endpoint (`judge_endpoint`)
  with a rubric + task + output, expects back `{"score": float, "reasoning": string}`. Works with
  Cerbo, Ollama, any LLM provider's API, or a hand-rolled endpoint — the grader doesn't know or
  care which.

### FR6 — Custom graders
- A custom grader is implemented the same way as a consumer: an executable implementing a
  `Transcript in → Outcome out` stdio contract. Referenced in suite config by path, same as
  `system`. No framework code changes needed to add one.

### FR7 — Calibration engine
- `evalctl calibrate --human <results.json> --model <results.json>` compares Outcomes for shared
  `task_id`s and reports: mean absolute score difference, exact pass/fail agreement rate, and
  sample count.
- Output is itself JSON, so it can be gated in CI the same way suite runs are.

### FR8 — CLI (`evalctl`)
- `evalctl run --suite <path> [--output <path>]` — runs a suite, prints per-grader averages,
  optionally writes full results JSON.
- `evalctl calibrate --human <path> --model <path>` — agreement report.
- `evalctl validate --suite <path>` — checks a suite config and dataset are well-formed without
  running anything (fast feedback before a real run).
- `evalctl list-graders` — lists built-in graders and their expected `params`.

## Technical Architecture

- **Language**: Go. Single static binary (`evalctl`), no runtime dependency for the user.
  Rationale: distribution (matches kubectl/terraform/docker precedent — adopted regardless of
  the user's own stack), goroutines fit "fan out N concurrent consumer/grader subprocess calls"
  naturally, and the workload is I/O-bound (LLM latency dominates) so Go's simpler concurrency
  model is a better fit than Rust's steeper learning curve for the marginal performance gain.
- **Consumer/grader boundary**: subprocess + line-delimited JSON over stdio, versioned protocol.
  Same pattern as Language Server Protocol / Debug Adapter Protocol — proven for "one tool, many
  language ecosystems." No FFI, no per-language bindings, no shared runtime requirement.
- **Concurrency**: suite runner executes tasks concurrently (bounded worker pool) since each task
  is an independent subprocess call; concurrency limit configurable per suite (LLM rate limits
  vary by consumer).
- **Extensibility boundary**: the binary ships built-in graders (FR5) for zero-setup common
  cases; anything beyond that is an external executable, never a framework code change.

## Acceptance Criteria

- [ ] A Python script implementing the stdio protocol can be evaluated with zero Python
      dependency installed alongside `evalctl`.
- [ ] A Rust or Go consumer implementing the same protocol works identically — protocol
      conformance is the only requirement, not language.
- [ ] `evalctl run` against a suite with a trajectory-returning consumer (steps present) and a
      suite with a single-output consumer (steps absent) both work under the same core contract.
- [ ] `evalctl calibrate` produces a correct agreement report against a hand-constructed
      human/model outcome pair with a known expected agreement rate.
- [ ] `fail_below` threshold causes `evalctl run` to exit non-zero when breached, zero otherwise.
- [ ] Adding a new grader or a new consumer requires zero changes to the `evalctl` binary itself.

## Open Questions (resolve during /plan)

- Exact retry/timeout policy for subprocess calls that hang (LLM calls can be slow).
- Whether `llm_judge`'s HTTP contract should be OpenAI-compatible by default for easier reuse of
  existing endpoints, or a minimal custom contract.
- Config format for concurrency limits — global vs. per-grader vs. per-suite.
