package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunWithCustomGraderOnly(t *testing.T) {
	dir := t.TempDir()
	dataset := `{"id":"a","input":"hello","metadata":{"expected":"HELLO"}}
{"id":"b","input":"world","metadata":{"expected":"WRONG"}}
`
	datasetPath := writeTaskFile(t, dir, "tasks.jsonl", dataset)
	suiteYAML := `
name: custom-grader-suite
system: ` + fixturePath("consumer_uppercase.cmd") + `
dataset: ` + datasetPath + `
graders:
  - name: ` + fixturePath("grader_custom_exact.cmd") + `
    params: {}
`
	suitePath := writeTaskFile(t, dir, "suite.yaml", suiteYAML)
	outputPath := filepath.Join(dir, "results.json")

	res := runEvalctl(t, "run", "--suite", suitePath, "--output", outputPath)
	if res.ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0 (fail_below unset). stdout=%s stderr=%s", res.ExitCode, res.Stdout, res.Stderr)
	}

	b, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var rep struct {
		Outcomes []struct {
			TaskID string  `json:"task_id"`
			Score  float64 `json:"score"`
			Passed *bool   `json:"passed"`
		} `json:"outcomes"`
	}
	if err := json.Unmarshal(b, &rep); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if len(rep.Outcomes) != 2 {
		t.Fatalf("got %d outcomes, want 2", len(rep.Outcomes))
	}
	for _, o := range rep.Outcomes {
		switch o.TaskID {
		case "a":
			if o.Score != 1.0 || o.Passed == nil || !*o.Passed {
				t.Errorf("task a: got score=%v passed=%v, want 1.0/true", o.Score, o.Passed)
			}
		case "b":
			if o.Score != 0.0 || o.Passed == nil || *o.Passed {
				t.Errorf("task b: got score=%v passed=%v, want 0.0/false", o.Score, o.Passed)
			}
		}
	}
}
