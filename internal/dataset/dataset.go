// Package dataset loads JSONL/CSV/JSON dataset files into []protocol.Task,
// enforcing whole-dataset hard validation per FR-007.
package dataset

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"evalctl/internal/protocol"
)

// Load reads a dataset file, dispatching on its extension (.jsonl, .csv, .json).
func Load(path string) ([]protocol.Task, error) {
	ext := strings.ToLower(filepath.Ext(path))
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open dataset %s: %w", path, err)
	}
	defer f.Close()

	var tasks []protocol.Task
	switch ext {
	case ".jsonl":
		tasks, err = loadJSONL(f)
	case ".csv":
		tasks, err = loadCSV(f)
	case ".json":
		tasks, err = loadJSON(f)
	default:
		return nil, fmt.Errorf("unsupported dataset extension %q (want .jsonl, .csv, or .json)", ext)
	}
	if err != nil {
		return nil, fmt.Errorf("load dataset %s: %w", path, err)
	}
	if err := validate(tasks); err != nil {
		return nil, fmt.Errorf("load dataset %s: %w", path, err)
	}
	return tasks, nil
}

type jsonlRow struct {
	ID       string          `json:"id"`
	Input    json.RawMessage `json:"input"`
	Metadata json.RawMessage `json:"metadata"`
}

func loadJSONL(r io.Reader) ([]protocol.Task, error) {
	var tasks []protocol.Task
	scanner := bufio.NewScanner(r)
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
		tasks = append(tasks, toTask(row))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return tasks, nil
}

func loadJSON(r io.Reader) ([]protocol.Task, error) {
	var rows []jsonlRow
	dec := json.NewDecoder(r)
	if err := dec.Decode(&rows); err != nil {
		return nil, fmt.Errorf("malformed JSON array: %w", err)
	}
	tasks := make([]protocol.Task, 0, len(rows))
	for _, row := range rows {
		tasks = append(tasks, toTask(row))
	}
	return tasks, nil
}

func loadCSV(r io.Reader) ([]protocol.Task, error) {
	cr := csv.NewReader(r)
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	colIdx := make(map[string]int, len(header))
	for i, h := range header {
		colIdx[strings.TrimSpace(h)] = i
	}
	idCol, hasID := colIdx["id"]
	inputCol, hasInput := colIdx["input"]
	metaCol, hasMeta := colIdx["metadata"]
	if !hasID || !hasInput {
		return nil, fmt.Errorf("CSV header must contain \"id\" and \"input\" columns")
	}

	var tasks []protocol.Task
	rowNum := 1
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", rowNum, err)
		}
		rowNum++
		row := jsonlRow{
			ID:    rec[idCol],
			Input: json.RawMessage(mustJSONString(rec[inputCol])),
		}
		if hasMeta && metaCol < len(rec) && strings.TrimSpace(rec[metaCol]) != "" {
			row.Metadata = json.RawMessage(rec[metaCol])
		}
		tasks = append(tasks, toTask(row))
	}
	return tasks, nil
}

// mustJSONString wraps a raw CSV cell as a JSON string value, unless it is
// already valid JSON (so numeric/object/array cells pass through as-is).
func mustJSONString(s string) []byte {
	var probe json.RawMessage
	if err := json.Unmarshal([]byte(s), &probe); err == nil {
		return []byte(s)
	}
	b, _ := json.Marshal(s)
	return b
}

func toTask(row jsonlRow) protocol.Task {
	t := protocol.Task{ID: row.ID, Input: row.Input, Metadata: row.Metadata}
	if len(t.Metadata) == 0 {
		t.Metadata = json.RawMessage(`{}`)
	}
	return t
}

// validate enforces FR-007: missing id/input or duplicate ids reject the
// whole dataset, with no partial load.
func validate(tasks []protocol.Task) error {
	if len(tasks) == 0 {
		return fmt.Errorf("dataset is empty")
	}
	seen := make(map[string]bool, len(tasks))
	for i, t := range tasks {
		if err := t.Validate(); err != nil {
			return fmt.Errorf("record %d: %w", i+1, err)
		}
		if seen[t.ID] {
			return fmt.Errorf("duplicate task id %q", t.ID)
		}
		seen[t.ID] = true
	}
	return nil
}
