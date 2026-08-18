# Quickstart: Validating the Universal Evaluation Framework

Proves User Stories 1–4 end-to-end. Assumes `evalctl` is built and on
`PATH` (see plan.md Project Structure for build instructions).

## Prerequisites

- `evalctl` binary (single static build, no other runtime required for
  the framework itself).
- Python 3 available on this machine *only* to play the role of "some
  other language" for the evaluated system in this walkthrough — proves
  SC-001 (the framework itself needs nothing beyond the consumer's own
  runtime).

## Story 1 — Single-output consumer, built-in grader, CI gate

1. Consumer script (`run_my_app.py`) implementing the stdio protocol
   (`contracts/stdio-protocol.md`): reads one JSON `Task` line from stdin,
   writes one JSON `Transcript` line to stdout with `output` echoing
   `task.input` uppercased.
   (This and the custom grader in Story 2 are both added purely as external
   scripts/executables — no change to the `evalctl` binary itself — proving
   SC-005.)
2. Dataset (`tasks.jsonl`), 3 rows, each `{"id": "...", "input": "...",
   "metadata": {"expected": "..."}}`.
3. Suite (`suite.yaml`) per `contracts/suite-config-schema.md`, using the
   built-in `exact_match` grader and `fail_below: 0.8`.
4. Run `evalctl validate --suite suite.yaml` → expect exit code `0`.
5. Run `evalctl run --suite suite.yaml --output results.json` → expect a
   per-grader summary on stdout, `results.json` written, exit code `0`
   (all 3 rows should exact-match by construction).
6. Break one dataset row's `expected` value so the suite's score drops
   below `0.8`, rerun → expect non-zero exit code (proves FR-006 /
   SC-004).
7. Make the consumer script `exit(1)` for one task id → rerun → expect the
   run still completes (other tasks graded), with that task's outcome
   showing `error: "consumer_error"` in `results.json` (proves FR-003).

## Story 2 — Trajectory consumer

1. Second consumer script returning `steps: [...]` alongside `output`.
2. Suite pointing at it with a trajectory-aware custom grader (a small
   script implementing the custom-grader half of
   `contracts/stdio-protocol.md`) that inspects `transcript.steps`.
3. Run the *same* `evalctl run --suite ...` command (no separate code
   path) → expect success and outcomes referencing step data (proves
   FR-011, SC-002).
4. Point the same trajectory-aware grader at a task from Story 1's
   suite (no `steps`) → expect a failing outcome with `error: "no
   trajectory data"` (proves the edge case in spec.md).

## Story 3 — Calibration

1. Hand-write `human.json` and `model.json`, each an array of `Outcome`
   for the same 5 `task_id`s, with known score differences worked out by
   hand.
2. Run `evalctl calibrate --human human.json --model model.json` →
   compare `mean_absolute_diff`, `pass_fail_agreement`, `n_compared`
   against the hand-calculated values (proves SC-003).
3. Rerun with a `model.json` sharing zero `task_id`s with `human.json` →
   expect `n_compared: 0`, not an error or misleading figure (Story 3
   Scenario 2).

## Story 4 — CI gate

Already covered by Story 1 step 6 — rerun the same suite unmodified
several times and confirm exit code is stable (deterministic gating,
SC-004).

## Cleanup

No persistent state is created outside the working directory (`results.json`,
any hand-written fixture files) — delete the scratch directory when done.
