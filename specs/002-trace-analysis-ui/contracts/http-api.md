# Contract: `evalctl analyze` HTTP API

Loopback-only (FR-015). Consumed by the embedded frontend
(`internal/analyze/web/`) via `fetch()`; documented here as the interface
this feature exposes, per `data-model.md`'s entities. All request/response
bodies are JSON. All mutating endpoints write through to the sidecar file
before returning (FR-006) — a `200`/`201` response guarantees the write
already landed on disk.

Errors are `{"error": "<message>"}` with a `4xx`/`5xx` status. A `409` is
used specifically for the FR-004 duplicate-label-on-a-trace rejection and
the FR-009 "label already belongs to another axial code" case is not an
error — reassignment is the normal, expected path (research.md's Decision
in data-model.md's AxialCode validation).

## Traces (read-only — see AnalysisTrace, data-model.md)

### `GET /api/traces`

Returns a summary list, ordered as loaded from the input file.

```json
[ { "id": "t1", "missing": false }, { "id": "t2", "missing": false } ]
```

`missing` is always `false` here (present for symmetry with orphaned
annotations, which are surfaced via `GET /api/traces/{id}` instead — a
missing trace id has no `AnalysisTrace` to list).

### `GET /api/traces/{id}`

```json
{
  "id": "t1",
  "input": "...",
  "output": "...",
  "steps": [ { "index": 0, "state": {}, "action": {}, "result": {} } ],
  "annotations": [ { "id": "oc-1", "label": "Hallucinated citation", "note": "...", "created_at": "..." } ]
}
```

`404` if `id` was never in the loaded trace set. `annotations` embeds this
trace's own `OpenCodeAnnotation`s so the UI can render a trace and its
open codes in one request.

### `GET /api/orphaned-annotations`

Returns `OpenCodeAnnotation`s whose `trace_id` is not in the currently
loaded trace set (spec.md Edge Cases: sidecar references a trace id no
longer present). The UI uses this to render the "referencing a missing
trace" flag without having to probe every annotation's trace individually.

```json
[ { "id": "oc-9", "trace_id": "t-deleted", "label": "...", "note": "...", "created_at": "..." } ]
```

## Open-code annotations (see OpenCodeAnnotation, data-model.md)

### `POST /api/traces/{id}/annotations`

Request: `{"label": "Hallucinated citation", "note": "optional"}`.
`404` if the trace id doesn't exist. `400` if `label` is empty after
trimming. `409` (FR-004) if this trace already carries an annotation whose
normalized label matches, with the existing annotation's `id` in the
error body: `{"error": "...", "existing_annotation_id": "oc-1"}`.
`201` with the created `OpenCodeAnnotation` on success.

### `PUT /api/annotations/{id}`

Request: `{"label": "...", "note": "..."}` (full replace of both fields).
Same `400`/`409` validation as create (a duplicate check against this
trace's *other* annotations, excluding itself). `404` if `id` doesn't
exist. `200` with the updated annotation on success.

### `DELETE /api/annotations/{id}`

`404` if `id` doesn't exist, else `204`. Never cascades to axial-code
membership beyond what aggregation naturally reflects (a label with zero
remaining annotations simply stops appearing in `GET /api/open-codes`,
but any `AxialCode.member_labels` entry for it is left as-is — FR-009's
"no annotation data deleted as a side effect" runs both directions: axial
membership doesn't get silently pruned just because its last annotation
went away, since a future annotation may resurrect that label).

## Aggregated open codes (FR-008)

### `GET /api/open-codes`

```json
[
  { "normalized_label": "hallucinated citation", "display_label": "Hallucinated citation", "count": 2, "axial_code_id": "ax-1" },
  { "normalized_label": "wrong tone", "display_label": "wrong tone", "count": 1, "axial_code_id": null }
]
```

`axial_code_id` is `null` for uncategorized labels (FR-010) — the UI
groups/badges on this field rather than needing a separate "uncategorized"
endpoint.

## Axial codes (see AxialCode, data-model.md)

### `GET /api/axial-codes`

```json
[
  {
    "id": "ax-1",
    "name": "Factual errors",
    "description": "",
    "member_labels": ["hallucinated citation"],
    "trace_count": 2
  }
]
```

`trace_count` is server-computed per data-model.md's AxialCode "Derived"
rule (distinct-trace union across member labels).

### `POST /api/axial-codes`

Request: `{"name": "...", "description": "", "member_labels": []}`
(`member_labels` optional, defaults to `[]` — FR-009 creation-without-
assignment). `400` if `name` is empty. If any requested `member_labels`
entry currently belongs to a different axial code, it is moved (single
membership) as part of this same request. `201` with the created
`AxialCode`.

### `PUT /api/axial-codes/{id}`

Request: `{"name": "...", "description": "..."}` — renames/redescribes
only; membership changes go through the two endpoints below so that a
single-label reassignment doesn't require resending the whole member
list. `404` if `id` doesn't exist. `200` with the updated axial code.

### `POST /api/axial-codes/{id}/members`

Request: `{"normalized_label": "hallucinated citation"}`. Assigns the
label to this axial code, atomically removing it from any other axial
code it currently belongs to (single membership, per clarification).
`404` if the axial code id doesn't exist. `200` with the updated axial
code.

### `DELETE /api/axial-codes/{id}/members/{normalized_label}`

Unassigns the label, which reverts to "uncategorized" (does not move it
anywhere else). `404` if the axial code id doesn't exist or the label
isn't currently a member. `200` with the updated axial code.

### `DELETE /api/axial-codes/{id}`

Deletes the axial code. Its former member labels become uncategorized.
Underlying `OpenCodeAnnotation`s are never touched (FR-009). `404` if
`id` doesn't exist, else `204`.

## LLM-assisted suggestion (FR-011, FR-012)

### `GET /api/config`

```json
{ "llm_suggest_available": false }
```

The frontend uses this to show/hide/disable the "suggest with LLM"
control (FR-012) without guessing from a failed call.

### `POST /api/axial-codes/suggest`

Only registered on the mux when `--llm-endpoint` was configured
(FR-012) — a request to this path otherwise gets the mux's normal `404`,
which is itself the "visibly absent" behavior FR-012 asks for at the API
layer (the frontend additionally hides the button using `GET /api/config`
so a user isn't left guessing why a visible button 404s).

No request body (the server already knows which labels are
uncategorized). Response:

```json
{ "groupings": [ { "name": "Factual errors", "labels": ["hallucinated citation", "wrong citation format"] } ] }
```

Only currently-uncategorized normalized labels are sent to the endpoint
and can appear in the response (per clarification); this response is
**never** written to the sidecar file — the frontend renders it as an
editable draft and, on "apply," issues normal `POST /api/axial-codes` /
`POST /api/axial-codes/{id}/members` calls with the (possibly edited)
result, so applying a suggestion follows the exact same persistence path
and validation as manual grouping (FR-011 Acceptance Scenario 2).

On endpoint failure (unreachable, timeout, malformed upstream response):
`502` with `{"error": "<reason>"}` (FR-013) — never a 500 that could be
confused with a bug in `evalctl analyze` itself, and never a partial
write to the store.
