# Contract: Suite Configuration File

YAML or JSON (FR-005). Field reference — see data-model.md `Suite` /
`GraderConfig` for types and defaults.

```yaml
name: my-suite                  # required
system: ./run_my_app.py         # required — any executable implementing
                                 # the stdio protocol (contracts/stdio-protocol.md)
dataset: tasks.jsonl            # required — .jsonl | .csv | .json
concurrency: 5                  # optional, default 5
timeout_seconds: 30             # optional, default 30
fail_below: 0.8                 # optional — omit to disable CI gating
graders:
  - name: schema_validity       # built-in
    params:
      required_fields: [title, steps]
  - name: llm_judge             # built-in
    params:
      rubric: "Does the output fully address the input?"
      judge_endpoint: "http://localhost:8080/judge"
      api_key_env: "JUDGE_API_KEY"   # NOT api_key: "sk-..." — see FR-019
  - name: ./graders/my_custom_grader.sh   # custom, path-referenced
    params: {}
```

## Credential rule (FR-019)

A grader's `params` MUST NOT contain a literal secret value under any key
(e.g. `api_key`, `token`, `secret`). Secrets are referenced indirectly by
naming an environment variable (`api_key_env`, or grader-specific
`*_env`-suffixed keys for custom graders). `evalctl validate` rejects a
suite file containing a literal secret-shaped key with a plain string
value where an `*_env` field is expected.

## `evalctl validate` checks against this schema

1. File parses as valid YAML or JSON.
2. All required top-level fields present with correct types.
3. `system` path exists and is executable.
4. `dataset` path exists and loads without validation errors (data-model.md
   Task validation rules).
5. Each `graders[].name` is either a recognized built-in
   (`exact_match`, `schema_validity`, `entity_grounding`, `llm_judge`) or
   an existing executable path.
6. No literal credential values present (see above).

Validation never spawns the `system` process or any grader.
