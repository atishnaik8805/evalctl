# Contract: `llm_judge` Grader HTTP Endpoint

Applies to any endpoint referenced by a suite's `judge_endpoint` param
(FR-009). See research.md #2 for why this is a minimal custom contract
rather than an OpenAI-compatible passthrough.

## Request

`POST` to `judge_endpoint`, JSON body:

```json
{
  "protocol_version": "1.0",
  "rubric": "Does the output fully address the input?",
  "task": { "id": "...", "input": ..., "metadata": {} },
  "output": "<the evaluated system's Transcript.output, verbatim>"
}
```

Authentication (if any) is carried by whatever header/scheme the endpoint
operator's own service expects; the framework only supplies a bearer value
read from the environment variable named by `api_key_env` (FR-019) as an
`Authorization: Bearer <value>` header. Endpoints requiring a different
auth scheme are out of scope for v1 — use a custom grader executable that
wraps the call instead (FR-010).

## Expected response

```json
{ "score": 0.85, "reasoning": "Addresses 4 of 5 rubric points; missing X." }
```

- `score`: required, float in `[0.0, 1.0]`.
- `reasoning`: required, string (may be empty string, not absent).

## Failure handling (FR-018)

Any of the following produces a failing `Outcome` with `error` reason
`"judge_endpoint_error"` for that task-grader pair, without aborting the
suite:

- Connection refused / DNS failure / non-2xx HTTP status.
- Response body is not valid JSON.
- `score` missing, non-numeric, or outside `[0.0, 1.0]`.
- `reasoning` missing or non-string.
- No response before `timeout_seconds` elapses (`error: "timeout"`).
