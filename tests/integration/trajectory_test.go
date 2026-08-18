package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunTrajectoryConsumerWithTrajectoryGrader(t *testing.T) {
	dir := t.TempDir()
	datasetPath := writeTaskFile(t, dir, "tasks.jsonl", `{"id":"a","input":"hello"}`+"\n")
	suiteYAML := `
name: trajectory-suite
system: ` + fixturePath("consumer_trajectory.cmd") + `
dataset: ` + datasetPath + `
graders:
  - name: ` + fixturePath("grader_trajectory.cmd") + `
    params: {}
`
	suitePath := writeTaskFile(t, dir, "suite.yaml", suiteYAML)
	outputPath := filepath.Join(dir, "results.json")

	res := runEvalctl(t, "run", "--suite", suitePath, "--output", outputPath)
	if res.ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0. stdout=%s stderr=%s", res.ExitCode, res.Stdout, res.Stderr)
	}

	b, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var rep struct {
		Outcomes []struct {
			TaskID   string          `json:"task_id"`
			Score    float64         `json:"score"`
			Passed   *bool           `json:"passed"`
			Evidence json.RawMessage `json:"evidence"`
		} `json:"outcomes"`
	}
	if err := json.Unmarshal(b, &rep); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if len(rep.Outcomes) != 1 {
		t.Fatalf("got %d outcomes, want 1", len(rep.Outcomes))
	}
	o := rep.Outcomes[0]
	if o.Score != 1.0 || o.Passed == nil || !*o.Passed {
		t.Errorf("got score=%v passed=%v, want 1.0/true", o.Score, o.Passed)
	}
	var evidence map[string]any
	if err := json.Unmarshal(o.Evidence, &evidence); err != nil {
		t.Fatalf("unmarshal evidence: %v", err)
	}
	if evidence["step_count"].(float64) != 2 {
		t.Errorf("evidence.step_count = %v, want 2 (proves outcome references step data)", evidence["step_count"])
	}
}

func TestRunTrajectoryGraderOnNoStepsTranscript(t *testing.T) {
	dir := t.TempDir()
	datasetPath := writeTaskFile(t, dir, "tasks.jsonl", `{"id":"a","input":"hello"}`+"\n")
	suiteYAML := `
name: no-steps-suite
system: ` + fixturePath("consumer_uppercase.cmd") + `
dataset: ` + datasetPath + `
graders:
  - name: ` + fixturePath("grader_trajectory.cmd") + `
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
			Error  *string `json:"error"`
		} `json:"outcomes"`
	}
	if err := json.Unmarshal(b, &rep); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if len(rep.Outcomes) != 1 {
		t.Fatalf("got %d outcomes, want 1", len(rep.Outcomes))
	}
	if rep.Outcomes[0].Error == nil || *rep.Outcomes[0].Error != "no trajectory data" {
		t.Errorf("outcome.error = %v, want \"no trajectory data\"", rep.Outcomes[0].Error)
	}
}
