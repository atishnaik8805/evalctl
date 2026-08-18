// Package report builds and formats SuiteRunReport and CalibrationReport
// (data-model.md), evalctl's two JSON output shapes.
package report

import (
	"encoding/json"
	"fmt"
	"io"

	"evalctl/internal/protocol"
)

// SuiteRunReport is the output of `evalctl run` (data-model.md, FR-013).
type SuiteRunReport struct {
	SuiteName    string             `json:"suite_name"`
	GraderScores map[string]float64 `json:"grader_scores"`
	OverallScore float64            `json:"overall_score"`
	Outcomes     []protocol.Outcome `json:"outcomes"`
	Passed       bool               `json:"passed"`
}

// BuildSuiteRunReport aggregates per-grader mean scores and the overall
// score, and applies the fail_below gate (nil means the run always passes).
func BuildSuiteRunReport(suiteName string, outcomes []protocol.Outcome, failBelow *float64) SuiteRunReport {
	sums := make(map[string]float64)
	counts := make(map[string]int)
	for _, o := range outcomes {
		sums[o.GraderName] += o.Score
		counts[o.GraderName]++
	}

	graderScores := make(map[string]float64, len(sums))
	var overallSum float64
	for name, sum := range sums {
		mean := sum / float64(counts[name])
		graderScores[name] = mean
		overallSum += mean
	}
	overall := 0.0
	if len(graderScores) > 0 {
		overall = overallSum / float64(len(graderScores))
	}

	passed := true
	if failBelow != nil && overall < *failBelow {
		passed = false
	}

	return SuiteRunReport{
		SuiteName:    suiteName,
		GraderScores: graderScores,
		OverallScore: overall,
		Outcomes:     outcomes,
		Passed:       passed,
	}
}

// WriteJSON writes r as indented JSON to w.
func WriteJSON(w io.Writer, r any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// PrintSummary writes a human-readable per-grader summary to w
// (contracts/cli-commands.md: "human-readable" stdout summary).
func (r SuiteRunReport) PrintSummary(w io.Writer) {
	fmt.Fprintf(w, "Suite: %s\n", r.SuiteName)
	for name, score := range r.GraderScores {
		fmt.Fprintf(w, "  %s: %.4f\n", name, score)
	}
	fmt.Fprintf(w, "Overall score: %.4f\n", r.OverallScore)
	status := "PASS"
	if !r.Passed {
		status = "FAIL"
	}
	fmt.Fprintf(w, "Result: %s\n", status)
}

// CalibrationReport is the output of `evalctl calibrate` (data-model.md, FR-012).
type CalibrationReport struct {
	MeanAbsoluteDiff  float64 `json:"mean_absolute_diff"`
	PassFailAgreement float64 `json:"pass_fail_agreement"`
	NCompared         int     `json:"n_compared"`
}
