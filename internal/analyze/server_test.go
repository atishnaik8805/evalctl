package analyze

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func newTestServer(t *testing.T, traces []AnalysisTrace) (*Server, *Store) {
	t.Helper()
	dir := t.TempDir()
	store, err := Load(filepath.Join(dir, "annotations.json"))
	if err != nil {
		t.Fatalf("Load store: %v", err)
	}
	return NewServer(traces, store, "", ""), store
}

func sampleTraces() []AnalysisTrace {
	return []AnalysisTrace{
		{ID: "t1", Input: json.RawMessage(`"in1"`), Output: json.RawMessage(`"out1"`)},
		{ID: "t2", Input: json.RawMessage(`"in2"`), Output: json.RawMessage(`"out2"`)},
	}
}

func TestHandleListTraces(t *testing.T) {
	s, _ := newTestServer(t, sampleTraces())
	req := httptest.NewRequest(http.MethodGet, "/api/traces", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var got []traceSummary
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 2 || got[0].ID != "t1" || got[1].ID != "t2" {
		t.Fatalf("got %+v", got)
	}
	if got[0].Missing {
		t.Errorf("Missing = true, want false")
	}
}

func TestHandleGetTraceSuccess(t *testing.T) {
	s, _ := newTestServer(t, sampleTraces())
	req := httptest.NewRequest(http.MethodGet, "/api/traces/t1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rr.Code, rr.Body.String())
	}
	var got traceDetail
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ID != "t1" || string(got.Input) != `"in1"` {
		t.Fatalf("got %+v", got)
	}
	if len(got.Annotations) != 0 {
		t.Errorf("Annotations = %+v, want empty", got.Annotations)
	}
}

func TestHandleGetTraceNotFound(t *testing.T) {
	s, _ := newTestServer(t, sampleTraces())
	req := httptest.NewRequest(http.MethodGet, "/api/traces/nope", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestHandleOrphanedAnnotations(t *testing.T) {
	s, store := newTestServer(t, sampleTraces())
	store.state.OpenCodes = append(store.state.OpenCodes, OpenCodeAnnotation{
		ID: "oc-9", TraceID: "t-deleted", Label: "x", NormalizedLabel: "x", CreatedAt: "2026-09-20T10:00:00Z",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/orphaned-annotations", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var got []orphanedAnnotationView
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 1 || got[0].TraceID != "t-deleted" {
		t.Fatalf("got %+v", got)
	}
}

func doJSON(t *testing.T, s *Server, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reqBody = bytes.NewReader(b)
	} else {
		reqBody = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reqBody)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	return rr
}

func TestHandleCreateAnnotation(t *testing.T) {
	s, _ := newTestServer(t, sampleTraces())

	rr := doJSON(t, s, http.MethodPost, "/api/traces/t1/annotations", annotationRequest{Label: "Hallucinated citation", Note: "n"})
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body=%s", rr.Code, rr.Body.String())
	}
	var got annotationView
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ID != "oc-1" || got.Label != "Hallucinated citation" || got.Note != "n" {
		t.Fatalf("got %+v", got)
	}
}

func TestHandleCreateAnnotationTraceNotFound(t *testing.T) {
	s, _ := newTestServer(t, sampleTraces())
	rr := doJSON(t, s, http.MethodPost, "/api/traces/nope/annotations", annotationRequest{Label: "x"})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestHandleCreateAnnotationEmptyLabel(t *testing.T) {
	s, _ := newTestServer(t, sampleTraces())
	rr := doJSON(t, s, http.MethodPost, "/api/traces/t1/annotations", annotationRequest{Label: "  "})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestHandleCreateAnnotationDuplicate(t *testing.T) {
	s, _ := newTestServer(t, sampleTraces())
	doJSON(t, s, http.MethodPost, "/api/traces/t1/annotations", annotationRequest{Label: "x"})
	rr := doJSON(t, s, http.MethodPost, "/api/traces/t1/annotations", annotationRequest{Label: "X"})
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body=%s", rr.Code, rr.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["existing_annotation_id"] != "oc-1" {
		t.Fatalf("body = %+v, want existing_annotation_id oc-1", body)
	}
}

func TestHandleUpdateAndDeleteAnnotation(t *testing.T) {
	s, _ := newTestServer(t, sampleTraces())
	created := doJSON(t, s, http.MethodPost, "/api/traces/t1/annotations", annotationRequest{Label: "x"})
	var ann annotationView
	json.Unmarshal(created.Body.Bytes(), &ann)

	rr := doJSON(t, s, http.MethodPut, "/api/annotations/"+ann.ID, annotationRequest{Label: "y", Note: "note"})
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200, body=%s", rr.Code, rr.Body.String())
	}

	rrNotFound := doJSON(t, s, http.MethodPut, "/api/annotations/oc-999", annotationRequest{Label: "y"})
	if rrNotFound.Code != http.StatusNotFound {
		t.Fatalf("PUT status = %d, want 404", rrNotFound.Code)
	}

	rrDel := doJSON(t, s, http.MethodDelete, "/api/annotations/"+ann.ID, nil)
	if rrDel.Code != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, want 204", rrDel.Code)
	}
	rrDel404 := doJSON(t, s, http.MethodDelete, "/api/annotations/"+ann.ID, nil)
	if rrDel404.Code != http.StatusNotFound {
		t.Fatalf("DELETE status = %d, want 404 on second delete", rrDel404.Code)
	}
}

func TestHandleOpenCodes(t *testing.T) {
	s, _ := newTestServer(t, sampleTraces())
	doJSON(t, s, http.MethodPost, "/api/traces/t1/annotations", annotationRequest{Label: "Hallucinated citation"})
	doJSON(t, s, http.MethodPost, "/api/traces/t2/annotations", annotationRequest{Label: "hallucinated citation "})

	rr := doJSON(t, s, http.MethodGet, "/api/open-codes", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var got []openCodeView
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 1 || got[0].Count != 2 || got[0].AxialCodeID != nil {
		t.Fatalf("got %+v", got)
	}
}

func TestHandleCreateAndListAxialCodes(t *testing.T) {
	s, _ := newTestServer(t, sampleTraces())

	rr := doJSON(t, s, http.MethodPost, "/api/axial-codes", axialCodeRequest{Name: "Factual errors"})
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body=%s", rr.Code, rr.Body.String())
	}
	var created axialCodeView
	json.Unmarshal(rr.Body.Bytes(), &created)
	if created.ID != "ax-1" || created.TraceCount != 0 {
		t.Fatalf("created = %+v", created)
	}

	rrList := doJSON(t, s, http.MethodGet, "/api/axial-codes", nil)
	var list []axialCodeView
	json.Unmarshal(rrList.Body.Bytes(), &list)
	if len(list) != 1 || list[0].ID != "ax-1" {
		t.Fatalf("list = %+v", list)
	}
}

func TestHandleCreateAxialCodeEmptyName(t *testing.T) {
	s, _ := newTestServer(t, sampleTraces())
	rr := doJSON(t, s, http.MethodPost, "/api/axial-codes", axialCodeRequest{Name: "  "})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestHandleUpdateAxialCodeNotFound(t *testing.T) {
	s, _ := newTestServer(t, sampleTraces())
	rr := doJSON(t, s, http.MethodPut, "/api/axial-codes/ax-999", axialCodeRequest{Name: "x"})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestHandleAssignAndUnassignMember(t *testing.T) {
	s, _ := newTestServer(t, sampleTraces())
	doJSON(t, s, http.MethodPost, "/api/traces/t1/annotations", annotationRequest{Label: "wrong tone"})
	created := doJSON(t, s, http.MethodPost, "/api/axial-codes", axialCodeRequest{Name: "Group"})
	var ax axialCodeView
	json.Unmarshal(created.Body.Bytes(), &ax)

	rr := doJSON(t, s, http.MethodPost, "/api/axial-codes/"+ax.ID+"/members", memberRequest{NormalizedLabel: "wrong tone"})
	if rr.Code != http.StatusOK {
		t.Fatalf("assign status = %d, want 200, body=%s", rr.Code, rr.Body.String())
	}
	var assigned axialCodeView
	json.Unmarshal(rr.Body.Bytes(), &assigned)
	if len(assigned.MemberLabels) != 1 || assigned.TraceCount != 1 {
		t.Fatalf("assigned = %+v", assigned)
	}

	rrUnassign := doJSON(t, s, http.MethodDelete, "/api/axial-codes/"+ax.ID+"/members/wrong%20tone", nil)
	if rrUnassign.Code != http.StatusOK {
		t.Fatalf("unassign status = %d, want 200, body=%s", rrUnassign.Code, rrUnassign.Body.String())
	}
	var unassigned axialCodeView
	json.Unmarshal(rrUnassign.Body.Bytes(), &unassigned)
	if len(unassigned.MemberLabels) != 0 {
		t.Fatalf("unassigned = %+v, want zero members", unassigned)
	}

	rrUnassignAgain := doJSON(t, s, http.MethodDelete, "/api/axial-codes/"+ax.ID+"/members/wrong%20tone", nil)
	if rrUnassignAgain.Code != http.StatusNotFound {
		t.Fatalf("unassign-again status = %d, want 404", rrUnassignAgain.Code)
	}
}

func TestHandleAssignMemberReassignsAcrossAxialCodes(t *testing.T) {
	s, _ := newTestServer(t, sampleTraces())
	doJSON(t, s, http.MethodPost, "/api/traces/t1/annotations", annotationRequest{Label: "wrong tone"})
	c1 := doJSON(t, s, http.MethodPost, "/api/axial-codes", axialCodeRequest{Name: "Group 1", MemberLabels: []string{"wrong tone"}})
	c2 := doJSON(t, s, http.MethodPost, "/api/axial-codes", axialCodeRequest{Name: "Group 2"})
	var ax1, ax2 axialCodeView
	json.Unmarshal(c1.Body.Bytes(), &ax1)
	json.Unmarshal(c2.Body.Bytes(), &ax2)

	doJSON(t, s, http.MethodPost, "/api/axial-codes/"+ax2.ID+"/members", memberRequest{NormalizedLabel: "wrong tone"})

	rrList := doJSON(t, s, http.MethodGet, "/api/axial-codes", nil)
	var list []axialCodeView
	json.Unmarshal(rrList.Body.Bytes(), &list)
	for _, ax := range list {
		if ax.ID == ax1.ID && len(ax.MemberLabels) != 0 {
			t.Errorf("ax1 still has members after reassignment: %+v", ax.MemberLabels)
		}
		if ax.ID == ax2.ID && (len(ax.MemberLabels) != 1 || ax.MemberLabels[0] != "wrong tone") {
			t.Errorf("ax2 members = %+v, want [wrong tone]", ax.MemberLabels)
		}
	}
}

func TestHandleDeleteAxialCode(t *testing.T) {
	s, _ := newTestServer(t, sampleTraces())
	created := doJSON(t, s, http.MethodPost, "/api/axial-codes", axialCodeRequest{Name: "Group"})
	var ax axialCodeView
	json.Unmarshal(created.Body.Bytes(), &ax)

	rr := doJSON(t, s, http.MethodDelete, "/api/axial-codes/"+ax.ID, nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
	rr404 := doJSON(t, s, http.MethodDelete, "/api/axial-codes/"+ax.ID, nil)
	if rr404.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 on second delete", rr404.Code)
	}
}

func TestHandleConfigWithoutLLMEndpoint(t *testing.T) {
	s, _ := newTestServer(t, sampleTraces())
	rr := doJSON(t, s, http.MethodGet, "/api/config", nil)
	var got map[string]bool
	json.Unmarshal(rr.Body.Bytes(), &got)
	if got["llm_suggest_available"] {
		t.Errorf("llm_suggest_available = true, want false with no --llm-endpoint configured")
	}
}

func TestHandleConfigWithLLMEndpoint(t *testing.T) {
	dir := t.TempDir()
	store, _ := Load(filepath.Join(dir, "annotations.json"))
	s := NewServer(sampleTraces(), store, "http://example.invalid", "")

	rr := doJSON(t, s, http.MethodGet, "/api/config", nil)
	var got map[string]bool
	json.Unmarshal(rr.Body.Bytes(), &got)
	if !got["llm_suggest_available"] {
		t.Errorf("llm_suggest_available = false, want true with --llm-endpoint configured")
	}
}

func TestHandleSuggestRouteAbsentWithoutLLMEndpoint(t *testing.T) {
	s, _ := newTestServer(t, sampleTraces())
	rr := doJSON(t, s, http.MethodPost, "/api/axial-codes/suggest", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want the mux's normal 404 when --llm-endpoint is unset", rr.Code)
	}
}

func TestHandleSuggestSuccess(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"groupings": []map[string]any{{"name": "Tone issues", "labels": []string{"wrong tone"}}},
		})
	}))
	defer mock.Close()

	dir := t.TempDir()
	store, _ := Load(filepath.Join(dir, "annotations.json"))
	s := NewServer(sampleTraces(), store, mock.URL, "")
	store.CreateAnnotation("t1", "wrong tone", "")

	rr := doJSON(t, s, http.MethodPost, "/api/axial-codes/suggest", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rr.Code, rr.Body.String())
	}
	var got struct {
		Groupings []SuggestedGrouping `json:"groupings"`
	}
	json.Unmarshal(rr.Body.Bytes(), &got)
	if len(got.Groupings) != 1 || got.Groupings[0].Name != "Tone issues" {
		t.Fatalf("got %+v", got)
	}

	// Never written to the sidecar file (FR-011): no axial codes exist.
	if len(store.AxialCodesSummary()) != 0 {
		t.Errorf("expected the suggestion to not be persisted, got %+v", store.AxialCodesSummary())
	}
}

func TestHandleSuggestUpstreamFailureReturns502(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mock.Close()

	dir := t.TempDir()
	store, _ := Load(filepath.Join(dir, "annotations.json"))
	s := NewServer(sampleTraces(), store, mock.URL, "")

	rr := doJSON(t, s, http.MethodPost, "/api/axial-codes/suggest", nil)
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rr.Code)
	}
}
