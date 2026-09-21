# Data Model: Trace Analysis UI (Open & Axial Coding)

Entities are plain JSON on disk (sidecar file) and over the loopback HTTP
API (`contracts/http-api.md`); this document describes their shape and
validation rules independent of the Go types that back them internally
(`internal/analyze`, `internal/report`). See `research.md` for the
rationale behind each design choice referenced below.

## AnalysisTrace

The unit this feature annotates. Read-only at the HTTP layer — traces
come only from the `--traces` input file, never created/edited/deleted
through the API.

| Field    | Type    | Required | Notes |
|----------|---------|----------|-------|
| `id`     | string  | yes      | Unique within the loaded trace set (FR-014 rejects duplicates at load). |
| `input`  | any     | yes      | The input given to the evaluated system. |
| `output` | any     | yes      | The evaluated system's output. |
| `steps`  | Step[]  | no       | Ordered trajectory steps, when present (same shape as `protocol.Step`: `index`, `state`, `action`, `result`). |

**Sourced from** (research.md §1, §2):
- A (post-FR-001) `SuiteRunReport`'s `traces[]` array — one `AnalysisTrace`
  per `report.Trace{task_id, input, output, steps}` entry, `id` =
  `task_id`.
- A standalone raw trace JSONL file (FR-003) — one `AnalysisTrace` per
  non-blank line, `{id, input, output, steps}`.

**Validation** (at load time, both source shapes):
- Missing `id`, `input`, or `output` on any record → whole file rejected
  with a line/index-numbered error (mirrors `internal/dataset`'s
  "whole dataset or nothing" rule, `code-internals.md` §5).
- Duplicate `id` across two or more records → whole file rejected, naming
  the duplicate id (FR-014).
- A `SuiteRunReport` with `outcomes` present but no `traces` → rejected as
  a pre-FR-001 report (research.md §8), not treated as zero traces.

## OpenCodeAnnotation

One free-text label (plus optional note) attached to one `AnalysisTrace`.

| Field              | Type   | Required | Notes |
|--------------------|--------|----------|-------|
| `id`               | string | yes      | Server-generated, sequential (`oc-<n>`, research.md §3). Stable across edits; never reused after delete. |
| `trace_id`         | string | yes      | The `AnalysisTrace.id` this annotation is attached to. May reference a trace no longer present in a re-supplied trace file (orphaned — see Edge Cases in spec.md); such annotations are preserved, never dropped, and the UI flags them. |
| `label`            | string | yes      | Free text, as typed by the user. Preserved verbatim for display. |
| `normalized_label` | string | yes      | `label` trimmed of leading/trailing whitespace and case-folded (FR-008). Computed and stored at write time, not re-derived on every read. |
| `note`             | string | no       | Optional free-text elaboration. Defaults to `""`. |
| `created_at`       | string | yes      | RFC 3339 timestamp, server-assigned at creation. Used to determine "first-used casing" for a normalized label (earliest `created_at`/insertion order wins). |

**Validation**:
- `label` must be non-empty after trimming.
- A trace MUST NOT carry two annotations whose `normalized_label` values
  are equal (FR-004): creating or editing-into a duplicate is rejected
  with an error directing the user to the existing annotation's id.

**Derived**: the aggregate "open codes" view (FR-008) groups all
`OpenCodeAnnotation`s by `normalized_label`, showing the earliest-created
annotation's `label` as the display casing and a count equal to the
number of distinct `trace_id`s carrying that normalized label (equivalent
to annotation-instance count, since FR-004 already forbids a trace
carrying the same normalized label twice).

## AxialCode

A named, user-defined category grouping distinct open-code labels.

| Field           | Type     | Required | Notes |
|-----------------|----------|----------|-------|
| `id`            | string   | yes      | Server-generated, sequential (`ax-<n>`). |
| `name`          | string   | yes      | Non-empty, user-supplied. |
| `description`   | string   | no       | Defaults to `""`. |
| `member_labels` | string[] | yes      | Normalized open-code labels assigned to this axial code. May be empty (FR-009: creation and assignment are separate actions; an axial code may also reach zero members later and is not auto-deleted). |

**Validation**:
- `name` must be non-empty.
- Each entry in `member_labels` is a `normalized_label` value; a given
  normalized label appears in at most one `AxialCode.member_labels` array
  across the whole store at any time (single membership, per
  clarification) — assigning a label to a different axial code removes it
  from whichever one it was previously in, as one atomic store mutation.
- Deleting an `AxialCode` removes it from the store but leaves every
  `OpenCodeAnnotation` referencing its (now-unassigned) labels untouched;
  those labels revert to "uncategorized."

**Derived**: an `AxialCode`'s displayed trace count is the number of
*distinct* `trace_id`s among all `OpenCodeAnnotation`s whose
`normalized_label` is in its `member_labels` (a trace carrying two member
labels of the same axial code counts once — union, not sum, per
clarification).

## AnnotationStore

The JSON sidecar file holding all `OpenCodeAnnotation`s and `AxialCode`s
for one trace input file (FR-006/FR-007). One store instance per running
`evalctl analyze` process, guarded in-memory by a single mutex
(research.md §6).

| Field                 | Type                 | Notes |
|-----------------------|----------------------|-------|
| `version`             | int                  | Schema version, `1` for this feature. Present so a future incompatible schema change can be detected explicitly rather than guessed from field presence. |
| `open_codes`          | OpenCodeAnnotation[] | In insertion order (this order is what supplies "first-used casing," research.md §3). |
| `axial_codes`         | AxialCode[]          | In creation order. |
| `next_open_code_id`   | int                  | Next numeric suffix for a generated `oc-<n>` id. |
| `next_axial_code_id`  | int                  | Next numeric suffix for a generated `ax-<n>` id. |

**Lifecycle**:
- On `evalctl analyze` startup: if the `--annotations` path exists, decode
  it into the in-memory store (reject startup if `version` is present and
  unrecognized, or the file is malformed JSON — fail fast rather than
  silently discarding existing annotations). If it does not exist, start
  from `{version: 1, open_codes: [], axial_codes: [], next_open_code_id:
  1, next_axial_code_id: 1}`.
- On every mutating API call: lock → mutate in-memory structs → serialize
  the whole store → write to a temp file in the same directory →
  `os.Rename` over the real path → unlock (research.md §6). No batching,
  no debounce — every mutation is immediately durable (FR-006).

## Example sidecar file

```json
{
  "version": 1,
  "open_codes": [
    {
      "id": "oc-1",
      "trace_id": "t1",
      "label": "Hallucinated citation",
      "normalized_label": "hallucinated citation",
      "note": "cited a paper that doesn't exist",
      "created_at": "2026-09-20T10:00:00Z"
    },
    {
      "id": "oc-2",
      "trace_id": "t2",
      "label": "hallucinated citation ",
      "normalized_label": "hallucinated citation",
      "note": "",
      "created_at": "2026-09-20T10:05:00Z"
    }
  ],
  "axial_codes": [
    {
      "id": "ax-1",
      "name": "Factual errors",
      "description": "",
      "member_labels": ["hallucinated citation"]
    }
  ],
  "next_open_code_id": 3,
  "next_axial_code_id": 2
}
```

Here both `oc-1` and `oc-2` normalize to `"hallucinated citation"`; the
aggregate open-codes view displays `"Hallucinated citation"` (the
`oc-1` casing, since it was created first) with a usage count of 2.
