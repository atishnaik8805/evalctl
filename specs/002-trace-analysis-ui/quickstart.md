# Quickstart: `evalctl analyze`

Validates the feature end-to-end against the User Stories in spec.md.
Run from the repo root once `evalctl analyze` is implemented (per
`tasks.md`, not part of this planning phase).

## Prerequisites

- Built `evalctl` binary (`go build -o evalctl ./cmd/evalctl` or
  `./build.ps1`).
- No network access required for Scenarios 1-4 below; Scenario 5 needs a
  reachable HTTP endpoint (a local fixture server is enough — no real LLM
  vendor required to validate the contract).

## Scenario 1 — Load a `SuiteRunReport` (User Story 1, Scenario 1)

```sh
evalctl run --suite examples/basic/suite.yaml --output /tmp/results.json
evalctl analyze --traces /tmp/results.json
```

Open `http://localhost:8420`. Expected: one entry per task id from
`examples/basic/tasks.jsonl`; selecting one shows its input and output.

## Scenario 2 — Load a standalone raw trace file (User Story 1, Scenario 2)

```sh
cat > /tmp/traces.jsonl <<'EOF'
{"id": "t1", "input": "What's 2+2?", "output": "4"}
{"id": "t2", "input": "Summarize X", "output": "X is...", "steps": [{"index": 0, "state": {}, "action": {"tool": "search"}, "result": {"hits": 3}}]}
EOF
evalctl analyze --traces /tmp/traces.jsonl
```

Expected: same UI, same interaction — `t1` and `t2` listed; `t2`'s step
sequence renders in order (User Story 1, Scenario 3). No different setup
step was needed versus Scenario 1 (SC-002).

## Scenario 3 — Open-code annotate and survive a restart (User Story 2)

1. With Scenario 2's server running, select `t1`, add an open-code label
   `hallucinated citation` with a note, then add a second label
   `wrong tone` to `t2`.
2. Check `cat /tmp/traces.jsonl.annotations.json` — both annotations
   should already be present (write-through, FR-006), matching
   `data-model.md`'s example shape.
3. Kill the server (`Ctrl+C`, or `kill -9` to simulate an unclean
   shutdown per SC-003) immediately after step 1's last add.
4. Re-run `evalctl analyze --traces /tmp/traces.jsonl` and reload the
   page. Expected: both annotations still present, attached to the
   correct trace (User Story 2, Scenario 4).
5. `GET http://localhost:8420/api/open-codes` — expected: two entries,
   each with `count: 1`.

## Scenario 4 — Group into axial codes, then restart (User Story 3)

1. From the axial-codes view, create `Factual errors` with no labels
   assigned yet (FR-009's create-empty path), confirm it appears with
   `trace_count: 0` and zero members.
2. Assign `hallucinated citation` to it. Confirm `wrong tone` still shows
   as "uncategorized" (FR-010).
3. Restart `evalctl analyze` against the same files. Expected: the axial
   code and its membership are unchanged.
4. Reassign `wrong tone` into a newly created `Tone issues` axial code,
   then delete `Tone issues`. Expected: `wrong tone` reverts to
   uncategorized (not deleted, not stuck under the deleted axial code),
   and the `wrong tone` `OpenCodeAnnotation` on `t2` is still present
   (`GET /api/traces/t2`) — grouping actions never delete Story 2 data.

## Scenario 5 — LLM-suggest happy path and failure path (User Story 4)

Start any HTTP server on `localhost:9000` that accepts a POST and returns
`{"groupings": [{"name": "Test group", "labels": ["wrong tone"]}]}` (a
five-line script is enough — this validates the contract in
`contracts/http-api.md`, not a real model).

```sh
evalctl analyze --traces /tmp/traces.jsonl --llm-endpoint http://localhost:9000
```

1. `GET /api/config` → `{"llm_suggest_available": true}`.
2. Click "suggest with LLM" → expect the draft grouping rendered,
   store untouched (`cat` the sidecar file — no new axial code yet).
3. Edit the draft's group name, click "apply" → expect the *edited* name
   persisted to the sidecar file (FR-011 Scenario 2).
4. Stop the fixture server, click "suggest with LLM" again → expect a
   clear in-UI failure message, server still responsive, sidecar file
   unchanged (FR-013).
5. Restart `evalctl analyze` **without** `--llm-endpoint` → `GET
   /api/config` returns `{"llm_suggest_available": false}`; confirm
   manual grouping (Scenario 4's flow) still works unaffected (FR-012,
   SC-005).

## Scenario 6 — Duplicate ids and pre-FR-001 reports (Edge Cases)

```sh
printf '{"id":"t1","input":"a","output":"b"}\n{"id":"t1","input":"c","output":"d"}\n' > /tmp/dup.jsonl
evalctl analyze --traces /tmp/dup.jsonl
```

Expected: fails immediately with an error naming `t1` as a duplicate
(FR-014), no server starts.

```sh
echo '{"suite_name":"x","grader_scores":{},"overall_score":0,"outcomes":[{"task_id":"t1","grader_name":"g","score":1}],"passed":true}' > /tmp/old.json
evalctl analyze --traces /tmp/old.json
```

Expected: fails immediately with an error stating the report predates
persisted transcripts (research.md §8), not a blank UI.
