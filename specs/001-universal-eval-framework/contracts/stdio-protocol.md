# Contract: Consumer / Custom Grader stdio Protocol

Applies to two roles that use the identical mechanism (FR-002, FR-010):

- **Evaluated system** (`suite.system`): `Task` in → `Transcript` out.
- **Custom grader** (`suite.graders[].name` as an executable path):
  `Transcript` in → `Outcome` out.

## Invocation

- One subprocess per task (v1; see research.md #1 — no persistent-process
  mode in this version).
- Exactly one line of JSON written to the subprocess's stdin, then stdin is
  closed.
- Exactly one line of JSON read from the subprocess's stdout after it
  exits (or after `timeout_seconds` elapses, whichever first).
- stderr is not parsed; captured only for debugging/error reporting.

## Request envelope (both roles)

```json
{
  "protocol_version": "1.0",
  "task": { "id": "...", "input": ..., "metadata": {} }
}
```

For the custom-grader role, the envelope additionally carries the
transcript to grade:

```json
{
  "protocol_version": "1.0",
  "task": { "id": "...", "input": ..., "metadata": {} },
  "transcript": { "task_id": "...", "output": ..., "steps": [...] }
}
```

`protocol_version` lets the wire format evolve without silently breaking
existing consumers/graders (FR-004).

## Response

- Evaluated system must write a `Transcript` (see data-model.md).
- Custom grader must write an `Outcome` (see data-model.md).

## Failure handling (FR-003, FR-018)

Any of the following is recorded as a failing `Outcome` for the affected
task (or task-grader pair) with an `error` reason, and the rest of the
suite continues — never a suite-level abort:

| Condition                                   | `error` reason      |
|----------------------------------------------|----------------------|
| Non-zero exit code                            | `"consumer_error"` / `"grader_error"` |
| stdout is not a single valid JSON line        | `"consumer_error"` / `"grader_error"` |
| No output before `timeout_seconds` elapses    | `"timeout"`          |
| Trajectory-aware grader, transcript has no `steps` | `"no trajectory data"` |
