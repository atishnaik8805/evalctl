package analyze

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTraceFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "traces.json")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write traces file: %v", err)
	}
	return p
}

func TestLoadTracesFromSuiteRunReport(t *testing.T) {
	p := writeTraceFile(t, `{
  "suite_name": "s",
  "grader_scores": {},
  "overall_score": 1,
  "passed": true,
  "outcomes": [{"task_id": "t1", "grader_name": "g", "score": 1}],
  "traces": [
    {"task_id": "t1", "input": "in1", "output": "out1"},
    {"task_id": "t2", "input": "in2", "output": "out2", "steps": [{"index": 0, "state": {}, "action": {}, "result": {}}]}
  ]
}`)

	traces, err := LoadTraces(p)
	if err != nil {
		t.Fatalf("LoadTraces: %v", err)
	}
	if len(traces) != 2 {
		t.Fatalf("len(traces) = %d, want 2", len(traces))
	}
	if traces[0].ID != "t1" || string(traces[0].Input) != `"in1"` {
		t.Errorf("traces[0] = %+v", traces[0])
	}
	if traces[1].ID != "t2" || len(traces[1].Steps) != 1 {
		t.Errorf("traces[1] = %+v", traces[1])
	}
}

func TestLoadTracesFromJSONL(t *testing.T) {
	p := writeTraceFile(t, `{"id": "t1", "input": "What's 2+2?", "output": "4"}

{"id": "t2", "input": "Summarize X", "output": "X is...", "steps": [{"index": 0, "state": {}, "action": {"tool": "search"}, "result": {"hits": 3}}]}
`)

	traces, err := LoadTraces(p)
	if err != nil {
		t.Fatalf("LoadTraces: %v", err)
	}
	if len(traces) != 2 {
		t.Fatalf("len(traces) = %d, want 2 (blank line skipped)", len(traces))
	}
	if traces[0].ID != "t1" {
		t.Errorf("traces[0].ID = %q, want t1", traces[0].ID)
	}
	if traces[1].ID != "t2" || len(traces[1].Steps) != 1 {
		t.Errorf("traces[1] = %+v", traces[1])
	}
}

func TestLoadTracesSingleTraceObjectShapeSniffing(t *testing.T) {
	// A standalone raw-trace file containing exactly one trace is also
	// valid as a single JSON object — must not be mistaken for a
	// SuiteRunReport (research.md §1).
	p := writeTraceFile(t, `{"id": "t1", "input": "a", "output": "b"}`)

	traces, err := LoadTraces(p)
	if err != nil {
		t.Fatalf("LoadTraces: %v", err)
	}
	if len(traces) != 1 || traces[0].ID != "t1" {
		t.Fatalf("traces = %+v, want single trace t1", traces)
	}
}

func TestLoadTracesJSONLMissingFields(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"missing id", `{"input": "a", "output": "b"}`, "missing required field: id"},
		{"missing input", `{"id": "t1", "output": "b"}`, "missing required field: input"},
		{"missing output", `{"id": "t1", "input": "a"}`, "missing required field: output"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := writeTraceFile(t, c.content+"\n{\"id\":\"t2\",\"input\":\"a\",\"output\":\"b\"}\n")
			_, err := LoadTraces(p)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want containing %q", err, c.want)
			}
		})
	}
}

func TestLoadTracesJSONLDuplicateID(t *testing.T) {
	p := writeTraceFile(t, `{"id":"t1","input":"a","output":"b"}
{"id":"t1","input":"c","output":"d"}
`)
	_, err := LoadTraces(p)
	if err == nil || !strings.Contains(err.Error(), `duplicate trace id "t1"`) {
		t.Fatalf("err = %v, want duplicate trace id t1", err)
	}
}

func TestLoadTracesReportDuplicateID(t *testing.T) {
	p := writeTraceFile(t, `{
  "suite_name": "s", "grader_scores": {}, "overall_score": 1, "passed": true,
  "outcomes": [{"task_id": "t1", "grader_name": "g", "score": 1}],
  "traces": [
    {"task_id": "t1", "input": "a", "output": "b"},
    {"task_id": "t1", "input": "c", "output": "d"}
  ]
}`)
	_, err := LoadTraces(p)
	if err == nil || !strings.Contains(err.Error(), `duplicate trace id "t1"`) {
		t.Fatalf("err = %v, want duplicate trace id t1", err)
	}
}

func TestLoadTracesPreFR001ReportRejected(t *testing.T) {
	p := writeTraceFile(t, `{"suite_name":"x","grader_scores":{},"overall_score":0,"outcomes":[{"task_id":"t1","grader_name":"g","score":1}],"passed":true}`)
	_, err := LoadTraces(p)
	if err == nil || !strings.Contains(err.Error(), "predates persisted transcripts") {
		t.Fatalf("err = %v, want pre-FR-001 rejection", err)
	}
}
