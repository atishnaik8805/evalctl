# Contract: `evalctl` CLI

The CLI is the product's user-facing interface (per the requirements
checklist note) — these commands and their exit-code semantics are
contract, not implementation detail.

## `evalctl run --suite <path> [--output <path>]`

- Runs every task in the suite's dataset against `system`, through every
  configured grader, up to `concurrency` tasks in flight.
- Prints a per-grader average score summary to stdout (human-readable).
- With `--output <path>`: writes the full `SuiteRunReport` (data-model.md)
  as JSON to `<path>`.
- **Exit code**: `0` if `fail_below` is unset, or `overall_score >=
  fail_below`. Non-zero (`1`) if `fail_below` is set and breached (FR-006,
  SC-004). Deterministic across repeated runs on unchanged inputs — no
  retry-induced flakiness (research.md #1).

## `evalctl calibrate --human <path> --model <path>`

- `<path>` arguments are JSON files containing arrays of `Outcome`
  (data-model.md), keyed by shared `task_id`.
- Prints/writes a `CalibrationReport` (data-model.md) as JSON to stdout.
- **Exit code**: `0` on successful comparison, including the
  zero-overlap case (`n_compared: 0` is a valid, non-error result — Story
  3 Scenario 2). Non-zero only on malformed input files.

## `evalctl validate --suite <path>`

- Runs the checks in `contracts/suite-config-schema.md` without spawning
  `system` or any grader.
- **Exit code**: `0` if the suite config and dataset are well-formed.
  Non-zero with a message identifying the specific problem (missing file,
  unrecognized grader name, malformed dataset row, duplicate task id,
  literal credential in config) otherwise.

## `evalctl list-graders`

- Prints the built-in graders (`exact_match`, `schema_validity`,
  `entity_grounding`, `llm_judge`) and each one's accepted `params` keys
  (FR-015).
- **Exit code**: always `0`.
