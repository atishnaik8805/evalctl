package integration

import (
	"encoding/json"
	"math"
	"testing"
)

func TestCalibrateMatchesHandCalculatedFigures(t *testing.T) {
	res := runEvalctl(t, "calibrate",
		"--human", fixturePath("calibration/human.json"),
		"--model", fixturePath("calibration/model.json"))
	if res.ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0. stdout=%s stderr=%s", res.ExitCode, res.Stdout, res.Stderr)
	}

	var rep struct {
		MeanAbsoluteDiff  float64 `json:"mean_absolute_diff"`
		PassFailAgreement float64 `json:"pass_fail_agreement"`
		NCompared         int     `json:"n_compared"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &rep); err != nil {
		t.Fatalf("unmarshal report: %v (stdout=%s)", err, res.Stdout)
	}

	if rep.NCompared != 5 {
		t.Errorf("NCompared = %d, want 5", rep.NCompared)
	}
	wantMAD := 0.18
	if math.Abs(rep.MeanAbsoluteDiff-wantMAD) > 1e-9 {
		t.Errorf("MeanAbsoluteDiff = %v, want %v", rep.MeanAbsoluteDiff, wantMAD)
	}
	wantAgreement := 0.6
	if math.Abs(rep.PassFailAgreement-wantAgreement) > 1e-9 {
		t.Errorf("PassFailAgreement = %v, want %v", rep.PassFailAgreement, wantAgreement)
	}
}

func TestCalibrateZeroOverlapIsNotAnError(t *testing.T) {
	res := runEvalctl(t, "calibrate",
		"--human", fixturePath("calibration/human.json"),
		"--model", fixturePath("calibration/model_zero_overlap.json"))
	if res.ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0 (zero overlap is a valid result, not an error). stdout=%s stderr=%s", res.ExitCode, res.Stdout, res.Stderr)
	}

	var rep struct {
		NCompared int `json:"n_compared"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &rep); err != nil {
		t.Fatalf("unmarshal report: %v (stdout=%s)", err, res.Stdout)
	}
	if rep.NCompared != 0 {
		t.Errorf("NCompared = %d, want 0", rep.NCompared)
	}
}
