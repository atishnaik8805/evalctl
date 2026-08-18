# evalctl

A single-binary, language-agnostic evaluation framework. `evalctl` evaluates
any external program that speaks a versioned stdio JSON protocol
(`contracts/stdio-protocol.md` under `specs/001-universal-eval-framework/`),
without ever importing that program's code.

## Build

```sh
go build -o evalctl ./cmd/evalctl
```

Or cross-compile static binaries for all supported platforms:

```powershell
./build.ps1
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
walkthrough.

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
