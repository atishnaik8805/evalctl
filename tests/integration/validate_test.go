package integration

import (
	"path/filepath"
	"testing"
)

func TestValidateValidSuite(t *testing.T) {
	dir := t.TempDir()
	datasetPath := writeTaskFile(t, dir, "tasks.jsonl", `{"id":"a","input":"hello","metadata":{"expected":"HELLO"}}`+"\n")
	suitePath := writeTaskFile(t, dir, "suite.yaml", exactMatchSuiteYAML(fixturePath("consumer_uppercase.cmd"), datasetPath, "0.8"))

	res := runEvalctl(t, "validate", "--suite", suitePath)
	if res.ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0. stdout=%s stderr=%s", res.ExitCode, res.Stdout, res.Stderr)
	}
}

func TestValidateMissingDatasetFile(t *testing.T) {
	dir := t.TempDir()
	suiteYAML := `
name: bad-suite
system: ` + fixturePath("consumer_uppercase.cmd") + `
dataset: ` + filepath.Join(dir, "does-not-exist.jsonl") + `
graders:
  - name: exact_match
`
	suitePath := writeTaskFile(t, dir, "suite.yaml", suiteYAML)

	res := runEvalctl(t, "validate", "--suite", suitePath)
	if res.ExitCode == 0 {
		t.Fatal("expected non-zero exit code for missing dataset file")
	}
}

func TestValidateUnrecognizedGraderName(t *testing.T) {
	dir := t.TempDir()
	datasetPath := writeTaskFile(t, dir, "tasks.jsonl", `{"id":"a","input":"hello"}`+"\n")
	suiteYAML := `
name: bad-suite
system: ` + fixturePath("consumer_uppercase.cmd") + `
dataset: ` + datasetPath + `
graders:
  - name: not_a_real_grader
`
	suitePath := writeTaskFile(t, dir, "suite.yaml", suiteYAML)

	res := runEvalctl(t, "validate", "--suite", suitePath)
	if res.ExitCode == 0 {
		t.Fatal("expected non-zero exit code for unrecognized grader name")
	}
}

func TestValidateLiteralCredential(t *testing.T) {
	dir := t.TempDir()
	datasetPath := writeTaskFile(t, dir, "tasks.jsonl", `{"id":"a","input":"hello"}`+"\n")
	suiteYAML := `
name: bad-suite
system: ` + fixturePath("consumer_uppercase.cmd") + `
dataset: ` + datasetPath + `
graders:
  - name: llm_judge
    params:
      rubric: "test"
      judge_endpoint: "http://localhost:8080"
      api_key: "sk-literal-secret"
`
	suitePath := writeTaskFile(t, dir, "suite.yaml", suiteYAML)

	res := runEvalctl(t, "validate", "--suite", suitePath)
	if res.ExitCode == 0 {
		t.Fatal("expected non-zero exit code for literal credential in config")
	}
}
