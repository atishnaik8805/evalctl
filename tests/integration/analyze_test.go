package integration

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// startAnalyze launches the real evalctl binary as `analyze` in the
// background on a free loopback port and waits until it answers
// GET /api/traces, per plan.md's Testing section (real HTTP server, real
// sidecar file on a temp directory).
func startAnalyze(t *testing.T, extraArgs ...string) (baseURL string, cleanup func()) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	args := append([]string{"analyze", "--addr", addr}, extraArgs...)
	cmd := exec.Command(evalctlBin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start evalctl analyze: %v", err)
	}

	baseURL = "http://" + addr
	deadline := time.Now().Add(10 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/api/traces")
		if err == nil {
			resp.Body.Close()
			lastErr = nil
			break
		}
		lastErr = err
		if cmd.ProcessState != nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if lastErr != nil {
		cmd.Process.Kill()
		cmd.Wait()
		t.Fatalf("evalctl analyze never became ready: %v\nstdout=%s\nstderr=%s", lastErr, stdout.String(), stderr.String())
	}

	cleanup = func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
	return baseURL, cleanup
}

func getJSON(t *testing.T, url string, out any) *http.Response {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode %s: %v", url, err)
		}
	}
	return resp
}

// Scenario 1: load a (post-FR-001) SuiteRunReport.
func TestAnalyzeLoadsSuiteRunReport(t *testing.T) {
	dir := t.TempDir()
	report := `{
  "suite_name": "story1-suite",
  "grader_scores": {"exact_match": 1},
  "overall_score": 1,
  "passed": true,
  "outcomes": [{"task_id": "a", "grader_name": "exact_match", "score": 1}],
  "traces": [{"task_id": "a", "input": "hello", "output": "HELLO"}]
}`
	reportPath := writeTaskFile(t, dir, "results.json", report)

	baseURL, cleanup := startAnalyze(t, "--traces", reportPath)
	defer cleanup()

	var list []map[string]any
	if resp := getJSON(t, baseURL+"/api/traces", &list); resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/traces status = %d", resp.StatusCode)
	}
	if len(list) != 1 || list[0]["id"] != "a" {
		t.Fatalf("trace list = %+v, want one entry with id \"a\"", list)
	}

	var detail map[string]any
	if resp := getJSON(t, baseURL+"/api/traces/a", &detail); resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/traces/a status = %d", resp.StatusCode)
	}
	if detail["input"] != "hello" || detail["output"] != "HELLO" {
		t.Fatalf("detail = %+v", detail)
	}
}

// Scenario 2: load a standalone raw-trace JSONL file, including a
// steps-bearing trace.
func TestAnalyzeLoadsStandaloneJSONL(t *testing.T) {
	dir := t.TempDir()
	content := `{"id": "t1", "input": "What's 2+2?", "output": "4"}
{"id": "t2", "input": "Summarize X", "output": "X is...", "steps": [{"index": 0, "state": {}, "action": {"tool": "search"}, "result": {"hits": 3}}]}
`
	tracesPath := writeTaskFile(t, dir, "traces.jsonl", content)

	baseURL, cleanup := startAnalyze(t, "--traces", tracesPath)
	defer cleanup()

	var list []map[string]any
	getJSON(t, baseURL+"/api/traces", &list)
	if len(list) != 2 {
		t.Fatalf("trace list = %+v, want 2 entries", list)
	}

	var detail struct {
		ID    string `json:"id"`
		Steps []struct {
			Index int `json:"index"`
		} `json:"steps"`
	}
	if resp := getJSON(t, baseURL+"/api/traces/t2", &detail); resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/traces/t2 status = %d", resp.StatusCode)
	}
	if len(detail.Steps) != 1 || detail.Steps[0].Index != 0 {
		t.Fatalf("detail.Steps = %+v, want one step at index 0", detail.Steps)
	}
}

func postJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

// Scenario 3: open-code annotate, write-through, and survive an unclean
// kill + restart against the same files.
func TestAnalyzeAnnotateWriteThroughAndSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	tracesPath := writeTaskFile(t, dir, "traces.jsonl", `{"id": "t1", "input": "a", "output": "b"}
{"id": "t2", "input": "c", "output": "d"}
`)
	annotationsPath := tracesPath + ".annotations.json"

	baseURL, cleanup := startAnalyze(t, "--traces", tracesPath)

	if resp := postJSON(t, baseURL+"/api/traces/t1/annotations", map[string]string{"label": "hallucinated citation", "note": "n1"}); resp.StatusCode != http.StatusCreated {
		t.Fatalf("create annotation on t1: status %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}
	if resp := postJSON(t, baseURL+"/api/traces/t2/annotations", map[string]string{"label": "wrong tone", "note": ""}); resp.StatusCode != http.StatusCreated {
		t.Fatalf("create annotation on t2: status %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}

	// Write-through: the sidecar file already reflects both annotations
	// before we even stop the server (FR-006).
	b, err := os.ReadFile(annotationsPath)
	if err != nil {
		t.Fatalf("read sidecar file: %v", err)
	}
	if !strings.Contains(string(b), "hallucinated citation") || !strings.Contains(string(b), "wrong tone") {
		t.Fatalf("sidecar file missing an annotation: %s", string(b))
	}

	// Simulate an unclean shutdown.
	cleanup()

	// Restart against the same files.
	baseURL2, cleanup2 := startAnalyze(t, "--traces", tracesPath)
	defer cleanup2()

	var t1detail struct {
		Annotations []map[string]any `json:"annotations"`
	}
	getJSON(t, baseURL2+"/api/traces/t1", &t1detail)
	if len(t1detail.Annotations) != 1 || t1detail.Annotations[0]["label"] != "hallucinated citation" {
		t.Fatalf("t1 annotations after restart = %+v", t1detail.Annotations)
	}

	var openCodes []map[string]any
	getJSON(t, baseURL2+"/api/open-codes", &openCodes)
	if len(openCodes) != 2 {
		t.Fatalf("open-codes after restart = %+v, want 2 entries", openCodes)
	}
	for _, oc := range openCodes {
		if oc["count"].(float64) != 1 {
			t.Errorf("open code %+v has count %v, want 1", oc, oc["count"])
		}
	}
}

func deleteURL(t *testing.T, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		t.Fatalf("build DELETE %s: %v", url, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", url, err)
	}
	return resp
}

// Scenario 4: group open codes into axial codes, restart, reassign, then
// delete an axial code and confirm its label reverts to uncategorized
// (not deleted) while the underlying annotation survives.
func TestAnalyzeAxialCodeGroupingAndRestart(t *testing.T) {
	dir := t.TempDir()
	tracesPath := writeTaskFile(t, dir, "traces.jsonl", `{"id": "t1", "input": "a", "output": "b"}
{"id": "t2", "input": "c", "output": "d"}
`)

	baseURL, cleanup := startAnalyze(t, "--traces", tracesPath)

	postJSON(t, baseURL+"/api/traces/t1/annotations", map[string]string{"label": "hallucinated citation"}).Body.Close()
	postJSON(t, baseURL+"/api/traces/t2/annotations", map[string]string{"label": "wrong tone"}).Body.Close()

	// Step 1: create an empty axial code.
	var factual map[string]any
	if resp := postJSON(t, baseURL+"/api/axial-codes", map[string]any{"name": "Factual errors"}); resp.StatusCode != http.StatusCreated {
		t.Fatalf("create axial code: status %d", resp.StatusCode)
	} else {
		json.NewDecoder(resp.Body).Decode(&factual)
		resp.Body.Close()
	}
	if factual["trace_count"].(float64) != 0 || len(factual["member_labels"].([]any)) != 0 {
		t.Fatalf("factual = %+v, want zero trace_count and zero members", factual)
	}

	// Step 2: assign "hallucinated citation" to it; "wrong tone" stays uncategorized.
	factualID := factual["id"].(string)
	if resp := postJSON(t, baseURL+"/api/axial-codes/"+factualID+"/members", map[string]string{"normalized_label": "hallucinated citation"}); resp.StatusCode != http.StatusOK {
		t.Fatalf("assign member: status %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}

	var openCodes []map[string]any
	getJSON(t, baseURL+"/api/open-codes", &openCodes)
	for _, oc := range openCodes {
		if oc["normalized_label"] == "wrong tone" && oc["axial_code_id"] != nil {
			t.Errorf("wrong tone axial_code_id = %v, want nil (uncategorized)", oc["axial_code_id"])
		}
	}

	// Step 3: restart against the same files; axial code and membership survive.
	cleanup()
	baseURL2, cleanup2 := startAnalyze(t, "--traces", tracesPath)
	defer cleanup2()

	var axialCodes []map[string]any
	getJSON(t, baseURL2+"/api/axial-codes", &axialCodes)
	if len(axialCodes) != 1 || axialCodes[0]["id"] != factualID {
		t.Fatalf("axial codes after restart = %+v", axialCodes)
	}
	members := axialCodes[0]["member_labels"].([]any)
	if len(members) != 1 || members[0] != "hallucinated citation" {
		t.Fatalf("members after restart = %+v", members)
	}

	// Step 4: reassign "wrong tone" into a new axial code, then delete it.
	var toneIssues map[string]any
	if resp := postJSON(t, baseURL2+"/api/axial-codes", map[string]any{"name": "Tone issues", "member_labels": []string{"wrong tone"}}); resp.StatusCode != http.StatusCreated {
		t.Fatalf("create Tone issues: status %d", resp.StatusCode)
	} else {
		json.NewDecoder(resp.Body).Decode(&toneIssues)
		resp.Body.Close()
	}
	toneIssuesID := toneIssues["id"].(string)

	if resp := deleteURL(t, baseURL2+"/api/axial-codes/"+toneIssuesID); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete Tone issues: status %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}

	var openCodesAfter []map[string]any
	getJSON(t, baseURL2+"/api/open-codes", &openCodesAfter)
	found := false
	for _, oc := range openCodesAfter {
		if oc["normalized_label"] == "wrong tone" {
			found = true
			if oc["axial_code_id"] != nil {
				t.Errorf("wrong tone axial_code_id = %v, want nil after deleting its axial code", oc["axial_code_id"])
			}
		}
	}
	if !found {
		t.Fatalf("expected \"wrong tone\" to still appear in open codes, got %+v", openCodesAfter)
	}

	var t2detail struct {
		Annotations []map[string]any `json:"annotations"`
	}
	getJSON(t, baseURL2+"/api/traces/t2", &t2detail)
	if len(t2detail.Annotations) != 1 || t2detail.Annotations[0]["label"] != "wrong tone" {
		t.Fatalf("t2 annotations = %+v, want the wrong tone annotation still present", t2detail.Annotations)
	}
}

// Scenario 5: LLM-suggest happy path (draft never auto-persisted, edited
// name wins on apply) and failure path (clear 502, server still
// responsive, sidecar unchanged), then confirm manual grouping still
// works with --llm-endpoint unset.
func TestAnalyzeLLMSuggestHappyAndFailurePaths(t *testing.T) {
	dir := t.TempDir()
	tracesPath := writeTaskFile(t, dir, "traces.jsonl", `{"id": "t1", "input": "a", "output": "b"}
`)
	annotationsPath := tracesPath + ".annotations.json"

	mockUp := true
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"groupings": [{"name": "Test group", "labels": ["wrong tone"]}]}`))
	})
	fixture := httptest.NewServer(mux)
	defer fixture.Close()

	baseURL, cleanup := startAnalyze(t, "--traces", tracesPath, "--llm-endpoint", fixture.URL)
	defer func() {
		if mockUp {
			cleanup()
		}
	}()

	postJSON(t, baseURL+"/api/traces/t1/annotations", map[string]string{"label": "wrong tone"}).Body.Close()

	var config map[string]bool
	getJSON(t, baseURL+"/api/config", &config)
	if !config["llm_suggest_available"] {
		t.Fatalf("config = %+v, want llm_suggest_available true", config)
	}

	// Suggest: draft returned, store untouched.
	var draft struct {
		Groupings []map[string]any `json:"groupings"`
	}
	if resp := postJSON(t, baseURL+"/api/axial-codes/suggest", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("suggest: status %d", resp.StatusCode)
	} else {
		json.NewDecoder(resp.Body).Decode(&draft)
		resp.Body.Close()
	}
	if len(draft.Groupings) != 1 || draft.Groupings[0]["name"] != "Test group" {
		t.Fatalf("draft = %+v", draft)
	}
	var axialCodesBefore []map[string]any
	getJSON(t, baseURL+"/api/axial-codes", &axialCodesBefore)
	if len(axialCodesBefore) != 0 {
		t.Fatalf("axial codes before apply = %+v, want none (suggestion not auto-persisted)", axialCodesBefore)
	}

	// Apply an *edited* name.
	if resp := postJSON(t, baseURL+"/api/axial-codes", map[string]any{"name": "Edited group name", "member_labels": []string{"wrong tone"}}); resp.StatusCode != http.StatusCreated {
		t.Fatalf("apply edited suggestion: status %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}
	var axialCodesAfter []map[string]any
	getJSON(t, baseURL+"/api/axial-codes", &axialCodesAfter)
	if len(axialCodesAfter) != 1 || axialCodesAfter[0]["name"] != "Edited group name" {
		t.Fatalf("axial codes after apply = %+v, want the edited name persisted", axialCodesAfter)
	}

	sidecarBeforeFailure, err := os.ReadFile(annotationsPath)
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}

	// Stop the fixture server: suggest now fails clearly, server stays up,
	// sidecar file is unchanged.
	fixture.Close()
	mockUp = false

	rr := postJSON(t, baseURL+"/api/axial-codes/suggest", nil)
	if rr.StatusCode != http.StatusBadGateway {
		t.Fatalf("suggest after fixture down: status %d, want 502", rr.StatusCode)
	}
	rr.Body.Close()

	var stillUp []map[string]any
	if resp := getJSON(t, baseURL+"/api/traces", &stillUp); resp.StatusCode != http.StatusOK {
		t.Fatalf("server not responsive after suggest failure: status %d", resp.StatusCode)
	}

	sidecarAfterFailure, err := os.ReadFile(annotationsPath)
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	if string(sidecarBeforeFailure) != string(sidecarAfterFailure) {
		t.Fatalf("sidecar file changed after a failed suggest call")
	}

	cleanup()

	// Restart without --llm-endpoint: config reports false, manual
	// grouping (Scenario 4's flow) still works.
	baseURL2, cleanup2 := startAnalyze(t, "--traces", tracesPath)
	defer cleanup2()

	var config2 map[string]bool
	getJSON(t, baseURL2+"/api/config", &config2)
	if config2["llm_suggest_available"] {
		t.Fatalf("config2 = %+v, want llm_suggest_available false", config2)
	}

	if resp := postJSON(t, baseURL2+"/api/axial-codes", map[string]any{"name": "Manual group"}); resp.StatusCode != http.StatusCreated {
		t.Fatalf("manual grouping without --llm-endpoint: status %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}
}

// Scenario 6 (part 1): duplicate trace ids reject the whole file, no
// server started.
func TestAnalyzeDuplicateIDRejected(t *testing.T) {
	dir := t.TempDir()
	content := `{"id":"t1","input":"a","output":"b"}
{"id":"t1","input":"c","output":"d"}
`
	tracesPath := writeTaskFile(t, dir, "dup.jsonl", content)

	res := runEvalctl(t, "analyze", "--traces", tracesPath, "--addr", "127.0.0.1:0")
	if res.ExitCode == 0 {
		t.Fatalf("expected non-zero exit code, stdout=%s stderr=%s", res.Stdout, res.Stderr)
	}
	if !strings.Contains(res.Stderr, "t1") {
		t.Errorf("stderr = %q, want it to name the duplicate id \"t1\"", res.Stderr)
	}
}

// Scenario 6 (part 2): a pre-FR-001 report (outcomes present, no traces
// field) fails fast with an actionable error, not a blank UI.
func TestAnalyzePreFR001ReportRejected(t *testing.T) {
	dir := t.TempDir()
	content := `{"suite_name":"x","grader_scores":{},"overall_score":0,"outcomes":[{"task_id":"t1","grader_name":"g","score":1}],"passed":true}`
	reportPath := writeTaskFile(t, dir, "old.json", content)

	res := runEvalctl(t, "analyze", "--traces", reportPath, "--addr", "127.0.0.1:0")
	if res.ExitCode == 0 {
		t.Fatalf("expected non-zero exit code, stdout=%s stderr=%s", res.Stdout, res.Stderr)
	}
	if !strings.Contains(res.Stderr, "predates persisted transcripts") {
		t.Errorf("stderr = %q, want it to name the pre-FR-001 condition", res.Stderr)
	}
}
