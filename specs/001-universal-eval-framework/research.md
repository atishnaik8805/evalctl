# Phase 0 Research: Universal Evaluation Framework

Three items were explicitly deferred from the spec to planning (see spec.md
Assumptions and the original root `spec.md` "Open Questions" section). Each
is resolved below.

## 1. Retry/timeout policy for hanging subprocess and HTTP calls

**Decision**: A single `timeout_seconds` field, settable in suite config
(sensible default: 30s), applies uniformly to every external call the
framework makes on behalf of a task: consumer subprocess calls, custom
grader subprocess calls, and `llm_judge` HTTP calls. On timeout, the
framework kills the subprocess (or cancels the HTTP request) and records a
failing outcome with reason `"timeout"` for that task (or task-grader pair),
using the exact same failure-recording path as FR-003/FR-018. **No
automatic retries in v1.**

**Rationale**: The spec already establishes one canonical failure-handling
pattern — record a failing outcome with a reason, keep the suite running
(FR-003, FR-018, and the edge case for trajectory-less graders). A hang is
just another failure mode along that same axis, so reusing it needs no new
concept. Automatic retries were rejected because SC-004 requires
deterministic gating ("consistently reports failure... across repeated
runs on unchanged inputs") — silent retries would let flaky consumers pass
on a lucky attempt, undermining that guarantee. A single suite-level
default keeps the config declarative and dependency-free, consistent with
FR-005's "no custom code required."

**Alternatives considered**:
- *N retries with backoff*: rejected — hides flakiness, breaks
  deterministic gating (SC-004), adds config surface (retry count, backoff
  curve) not requested by any user story.
- *No timeout at all*: rejected — the edge case "process hangs and never
  returns" is explicitly called out and must have defined behavior; an
  unbounded wait would hang the whole suite run (violates FR-017's
  concurrent-execution intent — a stuck worker should not blockers).
- *Per-call-type timeouts (separate consumer/grader/judge values)*: rejected
  for v1 as unnecessary config surface; a single suite-level knob covers the
  documented need. Can be split later without breaking existing suites
  (additive fields).

## 2. `llm_judge` HTTP contract shape

**Decision**: A minimal, framework-defined JSON contract, not an
OpenAI-compatible chat-completions passthrough:

Request (POST to `judge_endpoint`):
```json
{
  "protocol_version": "1.0",
  "rubric": "string",
  "task": { "...Task fields..." },
  "output": "<transcript output, as given by the evaluated system>"
}
```

Expected response:
```json
{ "score": 0.0, "reasoning": "string" }
```

A missing/unparseable `score` or `reasoning` field is treated as a grader
failure per FR-018 ("malformed response").

**Rationale**: FR-009 requires the judge grader to work "without being tied
to any single model provider," and the root spec's non-goals explicitly
rule out built-in per-provider integrations ("v1 ships a generic
HTTP-callable judge interface; provider-specific niceties are a consumer's
own concern"). An OpenAI-compatible chat-completions shape would force
every judge endpoint operator to speak that one vendor's request format
(messages array, model field, etc.) even for hand-rolled or non-OpenAI
endpoints, and would push prompt-templating logic into the framework —
duplicating work the endpoint operator is already responsible for under the
spec's Assumptions ("Provider-specific integration details... are the
responsibility of the party operating the judge endpoint").

**Alternatives considered**:
- *OpenAI-compatible chat completions endpoint*: rejected — couples the
  contract to one vendor's wire format, contradicts FR-009 and the
  Assumptions section.
- *Fully free-form/schemaless response*: rejected — the framework must be
  able to mechanically extract `score`/`reasoning` to build an `Outcome`;
  FR-018 requires detecting "missing expected score/reasoning fields,"
  which needs a fixed minimal schema to check against.

## 3. Concurrency configuration granularity

**Decision**: A single `concurrency` integer field at suite-config level
(default: 5), bounding how many tasks run concurrently within one suite
run. No global (cross-suite/daemon) config, no per-grader override in v1.

**Rationale**: FR-017 states concurrency is configurable "within a suite,"
scoping it there explicitly. There is no persistent daemon or shared
process (each `evalctl run` is a standalone invocation per the Assumptions'
one-process-per-task model), so a global config layer would need its own
file/env-var mechanism that duplicates the suite file — contradicting
FR-005's single-declarative-source intent. Per-grader limits were
considered because graders can also make external calls (custom grader
subprocess, `llm_judge` HTTP), but within a task, graders for that task run
after the task's transcript is available and are naturally bounded by the
same per-task worker slot; adding a second independent limit dimension is
config surface no user story asks for and can be added later (additive)
if a specific grader turns out to need stricter rate limiting than its
suite's task concurrency.

**Alternatives considered**:
- *Per-grader concurrency limits*: rejected for v1 — no user story
  requires it; task-level concurrency already bounds total in-flight
  external calls per task.
- *Global config file/env var*: rejected — adds a second configuration
  surface outside the suite file, contradicting FR-005.

## Resolved: credential indirection format (FR-019)

**Decision**: Suite config references a judge API key (or any grader
secret) by environment variable name, e.g. `api_key_env: "JUDGE_API_KEY"`.
At run time the framework reads `os.Getenv("JUDGE_API_KEY")` and uses the
value; the suite file itself never contains a literal secret. `evalctl
validate` checks that the *field* is present and well-formed but does not
require the referenced environment variable to actually be set (that's a
run-time concern, not a config-shape concern), so validation stays usable
in environments where secrets aren't provisioned yet.
