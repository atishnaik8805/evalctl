# evalctl

A single-binary, language-agnostic evaluation framework. `evalctl` evaluates
any external program that speaks a versioned stdio JSON protocol
(`contracts/stdio-protocol.md` under `specs/001-universal-eval-framework/`),
without ever importing that program's code.

## Install (build locally)

Prerequisites: [Go](https://go.dev/dl/) 1.22+. That's it — the `evalctl`
binary itself needs nothing else at runtime; the systems and graders it
evaluates can be written in any language.

```sh
git clone <this repo's URL>
cd evals
go build -o evalctl ./cmd/evalctl
```

This produces a single `evalctl` (or `evalctl.exe` on Windows) binary in
the current directory. Move it onto your `PATH`, or invoke it as `./evalctl`
/ `.\evalctl.exe`.

To cross-compile static binaries for all supported platforms instead:

```powershell
./build.ps1
```

To run the project's own test suite (unit + integration):

```sh
go test ./...
```

## Commands

- `evalctl run --suite <path> [--output <path>]` — run a suite's dataset
  against its `system`, through every configured grader.
- `evalctl validate --suite <path>` — check a suite config and dataset
  without spawning any process.
- `evalctl calibrate --human <path> --model <path>` — compare human- and
  model-produced `Outcome[]` files.
- `evalctl list-graders` — print the built-in graders and their `params`.

See `specs/001-universal-eval-framework/contracts/cli-commands.md` for the
full contract, and `specs/001-universal-eval-framework/quickstart.md` for a
scenario-level walkthrough.

## Example: evaluating a system with a built-in grader

`examples/basic/` contains a complete, runnable suite:

- `system.py` — a dummy "system under test" that reads a `Task` from
  stdin and writes a `Transcript` to stdout, uppercasing its input. This
  stands in for whatever real system you want to evaluate; `evalctl` only
  cares that it speaks the stdio protocol.
- `tasks.jsonl` — a 3-row dataset, one JSON object per line:
  `{"id": "...", "input": "...", "metadata": {"expected": "..."}}`.
- `suite.yaml` — wires them together with the built-in `exact_match`
  grader and a `fail_below: 0.8` CI gate.

```yaml
name: basic-example
system: system.py
dataset: tasks.jsonl
fail_below: 0.8
graders:
  - name: exact_match
    params: {}
```

On macOS/Linux, make the script executable first:

```sh
chmod +x examples/basic/system.py
```

On **Windows**, `evalctl` (like any `os/exec`-based tool) cannot run a
`.py` file directly — there's no interpreter association at the process
level. `examples/basic/` also ships `system.cmd`, a one-line wrapper
(`python "%~dp0system.py"`); edit `suite.yaml`'s `system:` field to point
at `system.cmd` instead of `system.py` before running on Windows. The
same applies to any other Python script used as a `system` or custom
grader in these examples.

Validate the suite (no process is spawned — just config/dataset checks):

```sh
evalctl validate --suite examples/basic/suite.yaml
```

Run it:

```sh
evalctl run --suite examples/basic/suite.yaml --output examples/basic/results.json
```

Expected stdout:

```
Suite: basic-example
  exact_match: 1.0000
Overall score: 1.0000
Result: PASS
```

Exit code `0`. `examples/basic/results.json` holds the full
`SuiteRunReport`: per-grader mean scores, the overall score, and a
per-task `Outcome` from every grader. Try editing one row's `expected`
value in `tasks.jsonl` and rerunning — the score drops below `0.8` and
`evalctl run` exits non-zero, which is what makes it usable as a CI gate
(see below).

## Example: a custom grader

Built-in graders (`exact_match`, `schema_validity`, `entity_grounding`,
`llm_judge`) cover common cases, but any suite can point a grader at an
external executable instead. `examples/custom-grader/` shows this:

- `responder.py` — a dummy system that answers a fixed set of questions.
- `grader_keyword.py` — a **custom grader**: an executable that reads a
  `{task, transcript}` request from stdin and writes an `Outcome` to
  stdout (the grader half of the same stdio protocol), scoring the
  fraction of `task.metadata.keywords` found in the system's answer.
- `suite.yaml` — since `grader_keyword.py` isn't a recognized built-in
  name, `evalctl` resolves it as a path (relative to the suite file) and
  runs it as a subprocess per task, exactly like `system`.

```yaml
name: custom-grader-example
system: responder.py
dataset: tasks.jsonl
fail_below: 1.0
graders:
  - name: grader_keyword.py
    params: {}
```

Custom graders can be written in any language — `evalctl` never imports
their code, only runs them as subprocesses and parses their stdout.

```sh
evalctl run --suite examples/custom-grader/suite.yaml
```

(On Windows, switch `system:`/the grader `name:` to `responder.cmd` /
`grader_keyword.cmd` first, per the note above.)

## Example: calibration

`evalctl calibrate` compares a human-graded `Outcome[]` file against a
model/grader-produced one for the same task ids, to check whether an
automated grader agrees with human judgment. `examples/calibrate/`
has a matched pair:

```sh
evalctl calibrate --human examples/calibrate/human.json --model examples/calibrate/model.json
```

```json
{
  "mean_absolute_diff": 0.2125,
  "pass_fail_agreement": 0.75,
  "n_compared": 4
}
```

`n_compared` is the number of shared `task_id`s between the two files
(unmatched ids on either side are ignored); `pass_fail_agreement` is the
fraction of those where `passed` agreed.

## CI gating

`evalctl run` is meant to be used as a CI pipeline gate:

- Exit code `0` when `fail_below` is unset, or when the suite's
  `overall_score >= fail_below`.
- Exit code `1` when `fail_below` is set and breached.
- No automatic retries: a suite run against unchanged inputs produces the
  same `overall_score`, `passed` value, and exit code every time.

Example CI step:

```yaml
- name: Run evaluation suite
  run: evalctl run --suite suite.yaml --output results.json
```

A non-zero exit from this step fails the pipeline; `results.json` holds the
full per-task, per-grader `SuiteRunReport` for inspection.
