package report

import (
	"evalctl/internal/protocol"
	"testing"
)

func outcome(grader string, score float64) protocol.Outcome {
	return protocol.Outcome{TaskID: "t1", GraderName: grader, Score: score}
}

func TestBuildSuiteRunReportAggregation(t *testing.T) {
	outcomes := []protocol.Outcome{
		outcome("exact_match", 1.0),
		outcome("exact_match", 0.0),
		outcome("schema_validity", 0.5),
	}
	r := BuildSuiteRunReport("suite1", outcomes, nil)

	if r.GraderScores["exact_match"] != 0.5 {
		t.Errorf("exact_match mean = %v, want 0.5", r.GraderScores["exact_match"])
	}
	if r.GraderScores["schema_validity"] != 0.5 {
		t.Errorf("schema_validity mean = %v, want 0.5", r.GraderScores["schema_validity"])
	}
	if r.OverallScore != 0.5 {
		t.Errorf("OverallScore = %v, want 0.5", r.OverallScore)
	}
	if !r.Passed {
		t.Error("expected Passed = true when fail_below is unset")
	}
}

func TestBuildSuiteRunReportFailBelowGate(t *testing.T) {
	outcomes := []protocol.Outcome{outcome("exact_match", 0.5)}

	below := 0.8
	r := BuildSuiteRunReport("suite1", outcomes, &below)
	if r.Passed {
		t.Error("expected Passed = false when overall_score < fail_below")
	}

	above := 0.3
	r2 := BuildSuiteRunReport("suite1", outcomes, &above)
	if !r2.Passed {
		t.Error("expected Passed = true when overall_score >= fail_below")
	}
}
