// Package analyze implements evalctl analyze: a local web UI for
// open-coding and axial-coding raw AI traces (spec.md).
package analyze

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"evalctl/internal/protocol"
	"evalctl/internal/report"
)

// AnalysisTrace is the unit this feature annotates (data-model.md
// AnalysisTrace). Read-only — sourced only from the --traces input file.
type AnalysisTrace struct {
	ID     string          `json:"id"`
	Input  json.RawMessage `json:"input"`
	Output json.RawMessage `json:"output"`
	Steps  []protocol.Step `json:"steps,omitempty"`
}

// LoadTraces reads path and dispatches by content shape (research.md §1)
// to either the SuiteRunReport-sourced path or the standalone JSONL
// raw-trace path (contracts/raw-trace-format.md), returning the whole
// file's traces or a single descriptive error (no partial load).
func LoadTraces(path string) ([]AnalysisTrace, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open traces %s: %w", path, err)
	}

	var probe map[string]json.RawMessage
	if err := json.Unmarshal(b, &probe); err == nil {
		if _, ok := probe["outcomes"]; ok {
			traces, err := loadFromReport(b)
			if err != nil {
				return nil, fmt.Errorf("load traces %s: %w", path, err)
			}
			return traces, nil
		}
	}

	traces, err := loadJSONL(b)
	if err != nil {
		return nil, fmt.Errorf("load traces %s: %w", path, err)
	}
	return traces, nil
}

// loadFromReport extracts AnalysisTraces from a (post-FR-001)
// SuiteRunReport, failing fast on a pre-FR-001 report (research.md §8).
func loadFromReport(b []byte) ([]AnalysisTrace, error) {
	var rep report.SuiteRunReport
	if err := json.Unmarshal(b, &rep); err != nil {
		return nil, fmt.Errorf("malformed SuiteRunReport: %w", err)
	}
	if len(rep.Outcomes) > 0 && len(rep.Traces) == 0 {
		return nil, fmt.Errorf("report predates persisted transcripts (FR-001) — re-run `evalctl run --output <path>` to regenerate it")
	}

	traces := make([]AnalysisTrace, 0, len(rep.Traces))
	for i, t := range rep.Traces {
		if t.TaskID == "" {
			return nil, fmt.Errorf("record %d: missing required field: task_id", i+1)
		}
		if len(t.Input) == 0 {
			return nil, fmt.Errorf("record %d: missing required field: input", i+1)
		}
		if len(t.Output) == 0 {
			return nil, fmt.Errorf("record %d: missing required field: output", i+1)
		}
		traces = append(traces, AnalysisTrace{ID: t.TaskID, Input: t.Input, Output: t.Output, Steps: t.Steps})
	}
	if err := checkDuplicateIDs(traces); err != nil {
		return nil, err
	}
	return traces, nil
}

type jsonlRow struct {
	ID     string          `json:"id"`
	Input  json.RawMessage `json:"input"`
	Output json.RawMessage `json:"output"`
	Steps  []protocol.Step `json:"steps"`
}

// loadJSONL parses a standalone raw-trace file per
// contracts/raw-trace-format.md: one JSON object per non-blank line.
func loadJSONL(b []byte) ([]AnalysisTrace, error) {
	var traces []AnalysisTrace
	scanner := bufio.NewScanner(bytes.NewReader(b))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var row jsonlRow
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("line %d: malformed JSON: %w", lineNum, err)
		}
		if row.ID == "" {
			return nil, fmt.Errorf("line %d: missing required field: id", lineNum)
		}
		if len(row.Input) == 0 {
			return nil, fmt.Errorf("line %d: missing required field: input", lineNum)
		}
		if len(row.Output) == 0 {
			return nil, fmt.Errorf("line %d: missing required field: output", lineNum)
		}
		traces = append(traces, AnalysisTrace{ID: row.ID, Input: row.Input, Output: row.Output, Steps: row.Steps})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if err := checkDuplicateIDs(traces); err != nil {
		return nil, err
	}
	return traces, nil
}

// checkDuplicateIDs enforces FR-014: a whole-file rejection naming the
// duplicate id, independent of source shape.
func checkDuplicateIDs(traces []AnalysisTrace) error {
	seen := make(map[string]bool, len(traces))
	for _, t := range traces {
		if seen[t.ID] {
			return fmt.Errorf("duplicate trace id %q", t.ID)
		}
		seen[t.ID] = true
	}
	return nil
}
