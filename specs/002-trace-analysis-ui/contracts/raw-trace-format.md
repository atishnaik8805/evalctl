# Contract: Standalone Raw Trace File Format

Satisfies FR-003. Independent of any `evalctl run` — for traces pulled
from production logs or any other tool.

## Format

JSONL (newline-delimited JSON): exactly one JSON object per non-blank
line, no enclosing array (per spec.md's Clarifications). Blank lines are
skipped, matching `internal/dataset`'s existing JSONL loader convention
(`internal/dataset/dataset.go`'s `loadJSONL`).

## Row shape

```json
{"id": "t1", "input": {"query": "..."}, "output": {"answer": "..."}, "steps": [{"index": 0, "state": {}, "action": {}, "result": {}}]}
```

| Field    | Type           | Required | Notes |
|----------|----------------|----------|-------|
| `id`     | string         | yes      | Unique across the file (FR-014). |
| `input`  | any JSON value | yes      | Passed through verbatim; not decoded/interpreted by the analysis UI beyond display. |
| `output` | any JSON value | yes      | Passed through verbatim. |
| `steps`  | array          | no       | Same shape as `protocol.Step` (`index`, `state`, `action`, `result`) if present. Omit entirely for a single-output trace with no trajectory. |

## Load-time validation (fail the whole file, no partial load)

- A line that is not valid JSON → reject with a 1-based line number and
  parse error.
- A record missing `id`, `input`, or `output` → reject naming the line
  number and the missing field.
- Two or more records sharing the same `id` → reject naming the duplicate
  id (FR-014), independent of which lines they're on.

## Example file (3 lines)

```jsonl
{"id": "t1", "input": "What's the capital of France?", "output": "Paris is the capital of France."}
{"id": "t2", "input": "Summarize this contract.", "output": "This contract...", "steps": [{"index": 0, "state": {"doc_loaded": true}, "action": {"tool": "read_doc"}, "result": {"chars": 4200}}, {"index": 1, "state": {}, "action": {"tool": "summarize"}, "result": {"summary": "..."}}]}
{"id": "t3", "input": {"query": "list open PRs"}, "output": {"prs": []}}
```
