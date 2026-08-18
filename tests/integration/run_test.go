package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeTaskFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

func exactMatchDataset(expectedC string) string {
	return `{"id":"a","input":"hello","metadata":{"expected":"HELLO"}}
{"id":"b","input":"world","metadata":{"expected":"WORLD"}}
{"id":"c","input":"foo","metadata":{"expected":"` + expectedC + `"}}
`
}

func exactMatchSuiteYAML(system, dataset string, failBelow string) string {
	return `
name: story1-suite
system: ` + system + `
dataset: ` + dataset + `
fail_below: ` + failBelow + `
graders:
  - name: exact_match
    params: {}
`
}

func TestRunPassesAboveThreshold(t *testing.T) {
	dir := t.TempDir()
	datasetPath := writeTaskFile(t, dir, "tasks.jsonl", exactMatchDataset("FOO"))
	suitePath := writeTaskFile(t, dir, "suite.yaml", exactMatchSuiteYAML(fixturePath("consumer_uppercase.cmd"), datasetPath, "0.8"))
	outputPath := filepath.Join(dir, "results.json")

	res := runEvalctl(t, "run", "--suite", suitePath, "--output", outputPath)
	if res.ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0. stdout=%s stderr=%s", res.ExitCode, res.Stdout, res.Stderr)
	}

	b, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var rep map[string]any
	if err := json.Unmarshal(b, &rep); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if rep["overall_score"].(float64) != 1.0 {
		t.Errorf("overall_score = %v, want 1.0", rep["overall_score"])
	}
	if rep["passed"] != true {
		t.Errorf("passed = %v, want true", rep["passed"])
	}
}

func TestRunFailsBelowThreshold(t *testing.T) {
	dir := t.TempDir()
	// "c" expects WRONG but consumer produces FOO uppercased, so 2/3 exact
	// matches = 0.667, below the 0.8 fail_below gate.
	datasetPath := writeTaskFile(t, dir, "tasks.jsonl", exactMatchDataset("WRONG"))
	suitePath := writeTaskFile(t, dir, "suite.yaml", exactMatchSuiteYAML(fixturePath("consumer_uppercase.cmd"), datasetPath, "0.8"))

	res := runEvalctl(t, "run", "--suite", suitePath)
	if res.ExitCode == 0 {
		t.Fatalf("expected non-zero exit code when overall_score < fail_below. stdout=%s", res.Stdout)
	}
}

func TestRunHandlesConsumerFailure(t *testing.T) {
	dir := t.TempDir()
	dataset := `{"id":"a","input":"hello","metadata":{"expected":"HELLO"}}
{"id":"fail-me","input":"world","metadata":{"expected":"WORLD"}}
`
	datasetPath := writeTaskFile(t, dir, "tasks.jsonl", dataset)
	suiteYAML := `
name: fail-handling-suite
system: ` + fixturePath("consumer_fails_on_id.cmd") + `
dataset: ` + datasetPath + `
graders:
  - name: exact_match
    params: {}
`
	suitePath := writeTaskFile(t, dir, "suite.yaml", suiteYAML)
	outputPath := filepath.Join(dir, "results.json")

	res := runEvalctl(t, "run", "--suite", suitePath, "--output", outputPath)
	if res.ExitCode != 0 {
		t.Fatalf("expected the run to complete (fail_below unset) despite one consumer failure, got exit %d. stdout=%s stderr=%s", res.ExitCode, res.Stdout, res.Stderr)
	}

	b, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var rep struct {
		Outcomes []struct {
			TaskID string  `json:"task_id"`
			Error  *string `json:"error"`
		} `json:"outcomes"`
	}
	if err := json.Unmarshal(b, &rep); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}

	found := false
	for _, o := range rep.Outcomes {
		if o.TaskID == "fail-me" {
			found = true
			if o.Error == nil || *o.Error != "consumer_error" {
				t.Errorf("fail-me outcome.error = %v, want \"consumer_error\"", o.Error)
			}
		}
	}
	if !found {
		t.Fatal("expected an outcome for task \"fail-me\"")
	}
}
