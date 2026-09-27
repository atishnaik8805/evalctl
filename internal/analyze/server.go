package analyze

import (
	"encoding/json"
	"fmt"
	"net/http"

	"evalctl/internal/analyze/web"
	"evalctl/internal/protocol"
)

// Server wires the HTTP API in contracts/http-api.md onto a loaded trace
// set and its AnnotationStore, plus the embedded static frontend at "/".
type Server struct {
	traces       []AnalysisTrace
	traceIndex   map[string]int
	store        *Store
	mux          *http.ServeMux
	llmEndpoint  string
	llmAPIKeyEnv string
}

// NewServer builds a Server ready to serve. traces is the read-only,
// load-time trace set (never mutated through the HTTP API). llmEndpoint
// and llmAPIKeyEnv configure the optional "suggest with LLM" feature
// (FR-011/FR-012); an empty llmEndpoint disables it — see T032/T033.
func NewServer(traces []AnalysisTrace, store *Store, llmEndpoint, llmAPIKeyEnv string) *Server {
	idx := make(map[string]int, len(traces))
	for i, t := range traces {
		idx[t.ID] = i
	}
	s := &Server{
		traces:       traces,
		traceIndex:   idx,
		store:        store,
		mux:          http.NewServeMux(),
		llmEndpoint:  llmEndpoint,
		llmAPIKeyEnv: llmAPIKeyEnv,
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/traces", s.handleListTraces)
	s.mux.HandleFunc("GET /api/traces/{id}", s.handleGetTrace)
	s.mux.HandleFunc("GET /api/orphaned-annotations", s.handleOrphanedAnnotations)
	s.mux.HandleFunc("POST /api/traces/{id}/annotations", s.handleCreateAnnotation)
	s.mux.HandleFunc("PUT /api/annotations/{id}", s.handleUpdateAnnotation)
	s.mux.HandleFunc("DELETE /api/annotations/{id}", s.handleDeleteAnnotation)
	s.mux.HandleFunc("GET /api/open-codes", s.handleOpenCodes)
	s.mux.HandleFunc("GET /api/axial-codes", s.handleListAxialCodes)
	s.mux.HandleFunc("POST /api/axial-codes", s.handleCreateAxialCode)
	s.mux.HandleFunc("PUT /api/axial-codes/{id}", s.handleUpdateAxialCode)
	s.mux.HandleFunc("POST /api/axial-codes/{id}/members", s.handleAssignMember)
	s.mux.HandleFunc("DELETE /api/axial-codes/{id}/members/{normalized_label}", s.handleUnassignMember)
	s.mux.HandleFunc("DELETE /api/axial-codes/{id}", s.handleDeleteAxialCode)
	s.mux.HandleFunc("GET /api/config", s.handleConfig)
	if s.llmEndpoint != "" {
		s.mux.HandleFunc("POST /api/axial-codes/suggest", s.handleSuggest)
	}

	s.mux.Handle("/", http.FileServer(http.FS(web.FS)))
}

// writeStoreError maps a Store mutation error to the HTTP status
// contracts/http-api.md specifies for it.
func writeStoreError(w http.ResponseWriter, err error) {
	var notFound *ErrNotFound
	var invalid *ErrInvalidInput
	var dup *ErrDuplicateLabel
	switch {
	case asStoreError(err, &notFound):
		writeError(w, http.StatusNotFound, notFound.Error())
	case asStoreError(err, &invalid):
		writeError(w, http.StatusBadRequest, invalid.Error())
	case asStoreError(err, &dup):
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":                  dup.Error(),
			"existing_annotation_id": dup.ExistingID,
		})
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

type traceSummary struct {
	ID      string `json:"id"`
	Missing bool   `json:"missing"`
}

// handleListTraces implements GET /api/traces: an id-ordered summary
// list, deliberately excluding input/output to keep initial page load
// flat regardless of trace count (plan.md Performance Goals).
func (s *Server) handleListTraces(w http.ResponseWriter, r *http.Request) {
	out := make([]traceSummary, 0, len(s.traces))
	for _, t := range s.traces {
		out = append(out, traceSummary{ID: t.ID, Missing: false})
	}
	writeJSON(w, http.StatusOK, out)
}

type annotationView struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Note      string `json:"note"`
	CreatedAt string `json:"created_at"`
}

type traceDetail struct {
	ID          string           `json:"id"`
	Input       json.RawMessage  `json:"input"`
	Output      json.RawMessage  `json:"output"`
	Steps       []protocol.Step  `json:"steps,omitempty"`
	Annotations []annotationView `json:"annotations"`
}

// handleGetTrace implements GET /api/traces/{id}: detail plus embedded
// annotations, 404 if id was never in the loaded trace set.
func (s *Server) handleGetTrace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	idx, ok := s.traceIndex[id]
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("trace %q not found", id))
		return
	}
	t := s.traces[idx]

	anns := s.store.AnnotationsForTrace(id)
	views := make([]annotationView, 0, len(anns))
	for _, a := range anns {
		views = append(views, annotationView{ID: a.ID, Label: a.Label, Note: a.Note, CreatedAt: a.CreatedAt})
	}

	writeJSON(w, http.StatusOK, traceDetail{
		ID:          t.ID,
		Input:       t.Input,
		Output:      t.Output,
		Steps:       t.Steps,
		Annotations: views,
	})
}

type orphanedAnnotationView struct {
	ID        string `json:"id"`
	TraceID   string `json:"trace_id"`
	Label     string `json:"label"`
	Note      string `json:"note"`
	CreatedAt string `json:"created_at"`
}

// handleOrphanedAnnotations implements GET /api/orphaned-annotations.
func (s *Server) handleOrphanedAnnotations(w http.ResponseWriter, r *http.Request) {
	orphans := s.store.OrphanedAnnotations(s.traceIndex)
	views := make([]orphanedAnnotationView, 0, len(orphans))
	for _, a := range orphans {
		views = append(views, orphanedAnnotationView{ID: a.ID, TraceID: a.TraceID, Label: a.Label, Note: a.Note, CreatedAt: a.CreatedAt})
	}
	writeJSON(w, http.StatusOK, views)
}

type annotationRequest struct {
	Label string `json:"label"`
	Note  string `json:"note"`
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	return dec.Decode(v)
}

// handleCreateAnnotation implements POST /api/traces/{id}/annotations.
func (s *Server) handleCreateAnnotation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.traceIndex[id]; !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("trace %q not found", id))
		return
	}

	var req annotationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "malformed request body: "+err.Error())
		return
	}

	ann, err := s.store.CreateAnnotation(id, req.Label, req.Note)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, annotationView{ID: ann.ID, Label: ann.Label, Note: ann.Note, CreatedAt: ann.CreatedAt})
}

// handleUpdateAnnotation implements PUT /api/annotations/{id}.
func (s *Server) handleUpdateAnnotation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req annotationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "malformed request body: "+err.Error())
		return
	}

	ann, err := s.store.UpdateAnnotation(id, req.Label, req.Note)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, annotationView{ID: ann.ID, Label: ann.Label, Note: ann.Note, CreatedAt: ann.CreatedAt})
}

// handleDeleteAnnotation implements DELETE /api/annotations/{id}.
func (s *Server) handleDeleteAnnotation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteAnnotation(id); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type openCodeView struct {
	NormalizedLabel string  `json:"normalized_label"`
	DisplayLabel    string  `json:"display_label"`
	Count           int     `json:"count"`
	AxialCodeID     *string `json:"axial_code_id"`
}

// handleOpenCodes implements GET /api/open-codes (FR-008).
func (s *Server) handleOpenCodes(w http.ResponseWriter, r *http.Request) {
	summary := s.store.OpenCodesSummary()
	views := make([]openCodeView, 0, len(summary))
	for _, sm := range summary {
		v := openCodeView{NormalizedLabel: sm.NormalizedLabel, DisplayLabel: sm.DisplayLabel, Count: sm.Count}
		if sm.AxialCodeID != "" {
			id := sm.AxialCodeID
			v.AxialCodeID = &id
		}
		views = append(views, v)
	}
	writeJSON(w, http.StatusOK, views)
}

type axialCodeView struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	MemberLabels []string `json:"member_labels"`
	TraceCount   int      `json:"trace_count"`
}

func toAxialCodeView(sm AxialCodeSummary) axialCodeView {
	return axialCodeView{
		ID:           sm.ID,
		Name:         sm.Name,
		Description:  sm.Description,
		MemberLabels: sm.MemberLabels,
		TraceCount:   sm.TraceCount,
	}
}

// axialCodeViewByID re-derives id's full view (including trace_count)
// after a mutation, since Store's mutation methods return the bare
// AxialCode without the server-computed count.
func (s *Server) axialCodeViewByID(id string) (axialCodeView, bool) {
	for _, ax := range s.store.AxialCodesSummary() {
		if ax.ID == id {
			return toAxialCodeView(ax), true
		}
	}
	return axialCodeView{}, false
}

// handleListAxialCodes implements GET /api/axial-codes.
func (s *Server) handleListAxialCodes(w http.ResponseWriter, r *http.Request) {
	summary := s.store.AxialCodesSummary()
	views := make([]axialCodeView, 0, len(summary))
	for _, ax := range summary {
		views = append(views, toAxialCodeView(ax))
	}
	writeJSON(w, http.StatusOK, views)
}

type axialCodeRequest struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	MemberLabels []string `json:"member_labels"`
}

// handleCreateAxialCode implements POST /api/axial-codes.
func (s *Server) handleCreateAxialCode(w http.ResponseWriter, r *http.Request) {
	var req axialCodeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "malformed request body: "+err.Error())
		return
	}
	ax, err := s.store.CreateAxialCode(req.Name, req.Description, req.MemberLabels)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	view, _ := s.axialCodeViewByID(ax.ID)
	writeJSON(w, http.StatusCreated, view)
}

// handleUpdateAxialCode implements PUT /api/axial-codes/{id}: rename and
// redescribe only, membership changes go through the endpoints below.
func (s *Server) handleUpdateAxialCode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req axialCodeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "malformed request body: "+err.Error())
		return
	}
	ax, err := s.store.RenameAxialCode(id, req.Name, req.Description)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	view, _ := s.axialCodeViewByID(ax.ID)
	writeJSON(w, http.StatusOK, view)
}

type memberRequest struct {
	NormalizedLabel string `json:"normalized_label"`
}

// handleAssignMember implements POST /api/axial-codes/{id}/members.
func (s *Server) handleAssignMember(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req memberRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "malformed request body: "+err.Error())
		return
	}
	ax, err := s.store.AssignMember(id, req.NormalizedLabel)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	view, _ := s.axialCodeViewByID(ax.ID)
	writeJSON(w, http.StatusOK, view)
}

// handleUnassignMember implements
// DELETE /api/axial-codes/{id}/members/{normalized_label}.
func (s *Server) handleUnassignMember(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	label := r.PathValue("normalized_label")
	ax, err := s.store.UnassignMember(id, label)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	view, _ := s.axialCodeViewByID(ax.ID)
	writeJSON(w, http.StatusOK, view)
}

// handleDeleteAxialCode implements DELETE /api/axial-codes/{id}.
func (s *Server) handleDeleteAxialCode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteAxialCode(id); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleConfig implements GET /api/config (FR-012): lets the frontend
// show/hide/disable "suggest with LLM" without guessing from a failed call.
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"llm_suggest_available": s.llmEndpoint != ""})
}

// handleSuggest implements POST /api/axial-codes/suggest. Only
// registered on the mux when --llm-endpoint was configured (FR-012); a
// request otherwise gets the mux's normal 404. Never writes to the
// sidecar file — the caller applies a suggestion via the normal
// create/assign endpoints (FR-011).
func (s *Server) handleSuggest(w http.ResponseWriter, r *http.Request) {
	labels := s.store.UncategorizedLabelSamples()
	groupings, err := Suggest(r.Context(), s.llmEndpoint, s.llmAPIKeyEnv, labels)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"groupings": groupings})
}
