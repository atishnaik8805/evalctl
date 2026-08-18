// Package calibrate compares human- and model-produced Outcomes for shared
// task ids (FR-012).
package calibrate

import (
	"math"

	"evalctl/internal/protocol"
)

// Result holds the comparison figures for CalibrationReport.
type Result struct {
	MeanAbsoluteDiff  float64
	PassFailAgreement float64
	NCompared         int
}

// Compare computes mean_absolute_diff, pass_fail_agreement, and n_compared
// over the task ids shared between human and model outcome sets.
// n_compared is 0 (not an error) when the sets don't overlap.
func Compare(human, model []protocol.Outcome) Result {
	humanByID := make(map[string]protocol.Outcome, len(human))
	for _, o := range human {
		humanByID[o.TaskID] = o
	}

	var sumAbsDiff float64
	var agreementCount int
	var n int

	for _, m := range model {
		h, ok := humanByID[m.TaskID]
		if !ok {
			continue
		}
		n++
		sumAbsDiff += math.Abs(h.Score - m.Score)
		if boolPassed(h) == boolPassed(m) {
			agreementCount++
		}
	}

	if n == 0 {
		return Result{NCompared: 0}
	}
	return Result{
		MeanAbsoluteDiff:  sumAbsDiff / float64(n),
		PassFailAgreement: float64(agreementCount) / float64(n),
		NCompared:         n,
	}
}

// boolPassed treats a nil Passed as false, so score-only outcomes compare consistently.
func boolPassed(o protocol.Outcome) bool {
	return o.Passed != nil && *o.Passed
}
