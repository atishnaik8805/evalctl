package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGatingIsDeterministicAcrossRepeatedRuns(t *testing.T) {
	dir := t.TempDir()
	datasetPath := writeTaskFile(t, dir, "tasks.jsonl", exactMatchDataset("FOO"))
	suitePath := writeTaskFile(t, dir, "suite.yaml", exactMatchSuiteYAML(fixturePath("consumer_uppercase.cmd"), datasetPath, "0.8"))
	outputPath := filepath.Join(dir, "results.json")

	var exitCodes []int
	var passedValues []bool
	for i := 0; i < 3; i++ {
		res := runEvalctl(t, "run", "--suite", suitePath, "--output", outputPath)
		exitCodes = append(exitCodes, res.ExitCode)

		var rep struct {
			Passed bool `json:"passed"`
		}
		b, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatalf("read output: %v", err)
		}
		if err := json.Unmarshal(b, &rep); err != nil {
			t.Fatalf("unmarshal report: %v", err)
		}
		passedValues = append(passedValues, rep.Passed)
	}

	for i := 1; i < len(exitCodes); i++ {
		if exitCodes[i] != exitCodes[0] {
			t.Errorf("run %d exit code = %d, want %d (deterministic gating)", i, exitCodes[i], exitCodes[0])
		}
		if passedValues[i] != passedValues[0] {
			t.Errorf("run %d passed = %v, want %v (deterministic gating)", i, passedValues[i], passedValues[0])
		}
	}
	if exitCodes[0] != 0 || !passedValues[0] {
		t.Fatalf("expected all repeated runs to pass (score above threshold), got exitCodes=%v passedValues=%v", exitCodes, passedValues)
	}
}

func TestGatingFlipsWithFailBelowThreshold(t *testing.T) {
	dir := t.TempDir()
	datasetPath := writeTaskFile(t, dir, "tasks.jsonl", exactMatchDataset("FOO"))

	belowSuitePath := writeTaskFile(t, dir, "suite_below.yaml", exactMatchSuiteYAML(fixturePath("consumer_uppercase.cmd"), datasetPath, "0.5"))
	res := runEvalctl(t, "run", "--suite", belowSuitePath)
	if res.ExitCode != 0 {
		t.Fatalf("expected pass when fail_below (0.5) is below overall_score (1.0), got exit %d", res.ExitCode)
	}

	aboveSuitePath := writeTaskFile(t, dir, "suite_above.yaml", exactMatchSuiteYAML(fixturePath("consumer_uppercase.cmd"), datasetPath, "1.01"))
	res2 := runEvalctl(t, "run", "--suite", aboveSuitePath)
	if res2.ExitCode == 0 {
		t.Fatalf("expected failure when fail_below (1.01) is above overall_score (1.0), got exit 0")
	}
}
