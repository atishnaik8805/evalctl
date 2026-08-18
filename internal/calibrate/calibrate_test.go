package calibrate

import (
	"math"
	"testing"

	"evalctl/internal/protocol"
)

func mkOutcome(taskID string, score float64, passed bool) protocol.Outcome {
	return protocol.Outcome{TaskID: taskID, Score: score, Passed: &passed}
}

func TestCompareKnownAgreementFigures(t *testing.T) {
	human := []protocol.Outcome{
		mkOutcome("t1", 1.0, true),
		mkOutcome("t2", 0.5, false),
		mkOutcome("t3", 0.0, false),
	}
	model := []protocol.Outcome{
		mkOutcome("t1", 0.8, true),  // diff 0.2, agree
		mkOutcome("t2", 0.9, true),  // diff 0.4, disagree
		mkOutcome("t3", 0.0, false), // diff 0.0, agree
	}

	r := Compare(human, model)
	if r.NCompared != 3 {
		t.Fatalf("NCompared = %d, want 3", r.NCompared)
	}
	wantMAD := (0.2 + 0.4 + 0.0) / 3
	if math.Abs(r.MeanAbsoluteDiff-wantMAD) > 1e-9 {
		t.Errorf("MeanAbsoluteDiff = %v, want %v", r.MeanAbsoluteDiff, wantMAD)
	}
	wantAgreement := 2.0 / 3.0
	if math.Abs(r.PassFailAgreement-wantAgreement) > 1e-9 {
		t.Errorf("PassFailAgreement = %v, want %v", r.PassFailAgreement, wantAgreement)
	}
}

func TestCompareZeroOverlap(t *testing.T) {
	human := []protocol.Outcome{mkOutcome("t1", 1.0, true)}
	model := []protocol.Outcome{mkOutcome("t2", 1.0, true)}

	r := Compare(human, model)
	if r.NCompared != 0 {
		t.Fatalf("NCompared = %d, want 0", r.NCompared)
	}
	if r.MeanAbsoluteDiff != 0 || r.PassFailAgreement != 0 {
		t.Errorf("expected zero figures on zero overlap, got %+v", r)
	}
}
