package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"evalctl/internal/calibrate"
	"evalctl/internal/protocol"
	"evalctl/internal/report"
)

// calibrateCommand loads human/model Outcome[] JSON files and prints their
// comparison as a CalibrationReport (FR-012, contracts/cli-commands.md).
// Exits non-zero only on malformed input files; a zero task-id overlap is a
// valid result, not an error.
func calibrateCommand(args []string) error {
	fs := flag.NewFlagSet("calibrate", flag.ExitOnError)
	humanPath := fs.String("human", "", "path to human Outcome[] JSON file")
	modelPath := fs.String("model", "", "path to model Outcome[] JSON file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *humanPath == "" || *modelPath == "" {
		return fmt.Errorf("calibrate: --human and --model are both required")
	}

	human, err := loadOutcomes(*humanPath)
	if err != nil {
		return fmt.Errorf("calibrate: --human: %w", err)
	}
	model, err := loadOutcomes(*modelPath)
	if err != nil {
		return fmt.Errorf("calibrate: --model: %w", err)
	}

	result := calibrate.Compare(human, model)
	rep := report.CalibrationReport{
		MeanAbsoluteDiff:  result.MeanAbsoluteDiff,
		PassFailAgreement: result.PassFailAgreement,
		NCompared:         result.NCompared,
	}
	return report.WriteJSON(os.Stdout, rep)
}

func loadOutcomes(path string) ([]protocol.Outcome, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var outcomes []protocol.Outcome
	if err := json.Unmarshal(b, &outcomes); err != nil {
		return nil, fmt.Errorf("malformed Outcome[] JSON: %w", err)
	}
	return outcomes, nil
}
