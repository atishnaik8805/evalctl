package analyze

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreLoadNonexistentPathIsFreshEmptyStore(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "does-not-exist.annotations.json")

	s, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.state.Version != storeVersion {
		t.Errorf("Version = %d, want %d", s.state.Version, storeVersion)
	}
	if len(s.state.OpenCodes) != 0 || len(s.state.AxialCodes) != 0 {
		t.Errorf("expected empty store, got %+v", s.state)
	}
	if s.state.NextOpenCodeID != 1 || s.state.NextAxialCodeID != 1 {
		t.Errorf("counters = %d/%d, want 1/1", s.state.NextOpenCodeID, s.state.NextAxialCodeID)
	}
}

func TestStoreLoadExistingFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "traces.annotations.json")
	content := `{
  "version": 1,
  "open_codes": [
    {"id": "oc-1", "trace_id": "t1", "label": "Hallucinated citation", "normalized_label": "hallucinated citation", "note": "cited a paper that doesn't exist", "created_at": "2026-09-20T10:00:00Z"},
    {"id": "oc-2", "trace_id": "t2", "label": "hallucinated citation ", "normalized_label": "hallucinated citation", "note": "", "created_at": "2026-09-20T10:05:00Z"}
  ],
  "axial_codes": [
    {"id": "ax-1", "name": "Factual errors", "description": "", "member_labels": ["hallucinated citation"]}
  ],
  "next_open_code_id": 3,
  "next_axial_code_id": 2
}`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(s.state.OpenCodes) != 2 {
		t.Fatalf("len(OpenCodes) = %d, want 2", len(s.state.OpenCodes))
	}
	if s.state.OpenCodes[0].ID != "oc-1" || s.state.OpenCodes[0].Label != "Hallucinated citation" {
		t.Errorf("OpenCodes[0] = %+v", s.state.OpenCodes[0])
	}
	if len(s.state.AxialCodes) != 1 || s.state.AxialCodes[0].Name != "Factual errors" {
		t.Errorf("AxialCodes = %+v", s.state.AxialCodes)
	}
	if s.state.NextOpenCodeID != 3 || s.state.NextAxialCodeID != 2 {
		t.Errorf("counters = %d/%d, want 3/2", s.state.NextOpenCodeID, s.state.NextAxialCodeID)
	}
}

func TestStoreLoadMalformedJSONRejected(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.annotations.json")
	if err := os.WriteFile(p, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := Load(p)
	if err == nil || !strings.Contains(err.Error(), "malformed JSON") {
		t.Fatalf("err = %v, want malformed JSON rejection", err)
	}
}

func TestStoreLoadUnrecognizedVersionRejected(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "future.annotations.json")
	if err := os.WriteFile(p, []byte(`{"version": 99, "open_codes": [], "axial_codes": [], "next_open_code_id": 1, "next_axial_code_id": 1}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := Load(p)
	if err == nil || !strings.Contains(err.Error(), "unrecognized version") {
		t.Fatalf("err = %v, want unrecognized version rejection", err)
	}
}

func TestStoreSaveIsAtomic(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "traces.annotations.json")

	s, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	s.state.OpenCodes = append(s.state.OpenCodes, OpenCodeAnnotation{
		ID: "oc-1", TraceID: "t1", Label: "x", NormalizedLabel: "x", CreatedAt: "2026-09-20T10:00:00Z",
	})
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// The real path exists with the saved content, and no stray temp file
	// is left behind (temp-file-then-rename, research.md §6).
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	var reloaded AnnotationStore
	if err := json.Unmarshal(b, &reloaded); err != nil {
		t.Fatalf("unmarshal saved file: %v", err)
	}
	if len(reloaded.OpenCodes) != 1 || reloaded.OpenCodes[0].ID != "oc-1" {
		t.Fatalf("reloaded = %+v", reloaded)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("stray temp file left behind: %s", e.Name())
		}
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Load(filepath.Join(t.TempDir(), "annotations.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return s
}

func TestCreateAnnotation(t *testing.T) {
	s := newTestStore(t)
	ann, err := s.CreateAnnotation("t1", "  Hallucinated Citation  ", "a note")
	if err != nil {
		t.Fatalf("CreateAnnotation: %v", err)
	}
	if ann.ID != "oc-1" {
		t.Errorf("ID = %q, want oc-1", ann.ID)
	}
	if ann.NormalizedLabel != "hallucinated citation" {
		t.Errorf("NormalizedLabel = %q", ann.NormalizedLabel)
	}
	if ann.Label != "  Hallucinated Citation  " {
		t.Errorf("Label = %q, want the as-typed value preserved", ann.Label)
	}

	ann2, err := s.CreateAnnotation("t2", "wrong tone", "")
	if err != nil {
		t.Fatalf("CreateAnnotation: %v", err)
	}
	if ann2.ID != "oc-2" {
		t.Errorf("ID = %q, want oc-2 (sequential)", ann2.ID)
	}
}

func TestCreateAnnotationEmptyLabelRejected(t *testing.T) {
	s := newTestStore(t)
	_, err := s.CreateAnnotation("t1", "   ", "")
	var invalid *ErrInvalidInput
	if !asStoreError(err, &invalid) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestCreateAnnotationDuplicateOnSameTraceRejected(t *testing.T) {
	s := newTestStore(t)
	first, err := s.CreateAnnotation("t1", "hallucinated citation", "")
	if err != nil {
		t.Fatalf("CreateAnnotation: %v", err)
	}
	_, err = s.CreateAnnotation("t1", "Hallucinated Citation ", "different note")
	var dup *ErrDuplicateLabel
	if !asStoreError(err, &dup) {
		t.Fatalf("err = %v, want ErrDuplicateLabel", err)
	}
	if dup.ExistingID != first.ID {
		t.Errorf("ExistingID = %q, want %q", dup.ExistingID, first.ID)
	}

	// Same label on a different trace is fine.
	if _, err := s.CreateAnnotation("t2", "hallucinated citation", ""); err != nil {
		t.Errorf("CreateAnnotation on a different trace: %v", err)
	}
}

func TestUpdateAnnotation(t *testing.T) {
	s := newTestStore(t)
	ann, _ := s.CreateAnnotation("t1", "wrong tone", "orig note")

	updated, err := s.UpdateAnnotation(ann.ID, "Wrong Tone", "new note")
	if err != nil {
		t.Fatalf("UpdateAnnotation: %v", err)
	}
	if updated.Label != "Wrong Tone" || updated.Note != "new note" {
		t.Errorf("updated = %+v", updated)
	}
	if updated.NormalizedLabel != "wrong tone" {
		t.Errorf("NormalizedLabel = %q", updated.NormalizedLabel)
	}
}

func TestUpdateAnnotationNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.UpdateAnnotation("oc-999", "x", "")
	var nf *ErrNotFound
	if !asStoreError(err, &nf) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateAnnotationIntoDuplicateRejected(t *testing.T) {
	s := newTestStore(t)
	first, _ := s.CreateAnnotation("t1", "hallucinated citation", "")
	second, _ := s.CreateAnnotation("t1", "wrong tone", "")

	_, err := s.UpdateAnnotation(second.ID, "Hallucinated Citation", "")
	var dup *ErrDuplicateLabel
	if !asStoreError(err, &dup) {
		t.Fatalf("err = %v, want ErrDuplicateLabel", err)
	}
	if dup.ExistingID != first.ID {
		t.Errorf("ExistingID = %q, want %q", dup.ExistingID, first.ID)
	}

	// Editing to its own current label is always fine (excludes itself).
	if _, err := s.UpdateAnnotation(second.ID, "wrong tone", "still fine"); err != nil {
		t.Errorf("UpdateAnnotation to unchanged label: %v", err)
	}
}

func TestDeleteAnnotation(t *testing.T) {
	s := newTestStore(t)
	ann, _ := s.CreateAnnotation("t1", "x", "")

	if err := s.DeleteAnnotation(ann.ID); err != nil {
		t.Fatalf("DeleteAnnotation: %v", err)
	}
	if len(s.AnnotationsForTrace("t1")) != 0 {
		t.Errorf("expected no annotations left on t1")
	}

	err := s.DeleteAnnotation(ann.ID)
	var nf *ErrNotFound
	if !asStoreError(err, &nf) {
		t.Fatalf("err = %v, want ErrNotFound on double-delete", err)
	}
}

func TestOpenCodesSummary(t *testing.T) {
	s := newTestStore(t)
	s.CreateAnnotation("t1", "Hallucinated citation", "")
	s.CreateAnnotation("t2", "hallucinated citation ", "")
	s.CreateAnnotation("t2", "wrong tone", "")

	summary := s.OpenCodesSummary()
	if len(summary) != 2 {
		t.Fatalf("len(summary) = %d, want 2", len(summary))
	}

	var hc, wt *OpenCodeSummary
	for i := range summary {
		switch summary[i].NormalizedLabel {
		case "hallucinated citation":
			hc = &summary[i]
		case "wrong tone":
			wt = &summary[i]
		}
	}
	if hc == nil || hc.DisplayLabel != "Hallucinated citation" || hc.Count != 2 {
		t.Errorf("hallucinated citation summary = %+v", hc)
	}
	if wt == nil || wt.DisplayLabel != "wrong tone" || wt.Count != 1 {
		t.Errorf("wrong tone summary = %+v", wt)
	}
}

func TestOpenCodesSummaryAxialCodeID(t *testing.T) {
	s := newTestStore(t)
	s.CreateAnnotation("t1", "hallucinated citation", "")
	s.state.AxialCodes = append(s.state.AxialCodes, AxialCode{
		ID: "ax-1", Name: "Factual errors", MemberLabels: []string{"hallucinated citation"},
	})

	summary := s.OpenCodesSummary()
	if len(summary) != 1 || summary[0].AxialCodeID != "ax-1" {
		t.Fatalf("summary = %+v, want AxialCodeID ax-1", summary)
	}
}

func TestCreateAxialCode(t *testing.T) {
	s := newTestStore(t)
	ax, err := s.CreateAxialCode("Factual errors", "desc", nil)
	if err != nil {
		t.Fatalf("CreateAxialCode: %v", err)
	}
	if ax.ID != "ax-1" || len(ax.MemberLabels) != 0 {
		t.Fatalf("ax = %+v, want ax-1 with zero members (FR-009 create-without-assignment)", ax)
	}

	ax2, err := s.CreateAxialCode("Tone issues", "", []string{"Wrong Tone", "wrong tone"})
	if err != nil {
		t.Fatalf("CreateAxialCode with members: %v", err)
	}
	if ax2.ID != "ax-2" || len(ax2.MemberLabels) != 1 || ax2.MemberLabels[0] != "wrong tone" {
		t.Fatalf("ax2 = %+v, want one deduped normalized member", ax2)
	}
}

func TestCreateAxialCodeEmptyNameRejected(t *testing.T) {
	s := newTestStore(t)
	_, err := s.CreateAxialCode("  ", "", nil)
	var invalid *ErrInvalidInput
	if !asStoreError(err, &invalid) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestAssignMemberSingleMembershipReassignment(t *testing.T) {
	s := newTestStore(t)
	ax1, _ := s.CreateAxialCode("Group 1", "", []string{"wrong tone"})
	ax2, _ := s.CreateAxialCode("Group 2", "", nil)

	updated2, err := s.AssignMember(ax2.ID, "wrong tone")
	if err != nil {
		t.Fatalf("AssignMember: %v", err)
	}
	if len(updated2.MemberLabels) != 1 || updated2.MemberLabels[0] != "wrong tone" {
		t.Fatalf("ax2 members = %+v", updated2.MemberLabels)
	}

	summary := s.AxialCodesSummary()
	for _, ax := range summary {
		if ax.ID == ax1.ID && len(ax.MemberLabels) != 0 {
			t.Errorf("ax1 still has members after reassignment: %+v", ax.MemberLabels)
		}
	}
}

func TestAssignMemberNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.AssignMember("ax-999", "wrong tone")
	var nf *ErrNotFound
	if !asStoreError(err, &nf) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUnassignMemberRevertsToUncategorized(t *testing.T) {
	s := newTestStore(t)
	ax, _ := s.CreateAxialCode("Group", "", []string{"wrong tone"})

	updated, err := s.UnassignMember(ax.ID, "wrong tone")
	if err != nil {
		t.Fatalf("UnassignMember: %v", err)
	}
	if len(updated.MemberLabels) != 0 {
		t.Fatalf("members = %+v, want empty after unassign", updated.MemberLabels)
	}
}

func TestUnassignMemberNotAMemberRejected(t *testing.T) {
	s := newTestStore(t)
	ax, _ := s.CreateAxialCode("Group", "", nil)
	_, err := s.UnassignMember(ax.ID, "wrong tone")
	var nf *ErrNotFound
	if !asStoreError(err, &nf) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestDeleteAxialCodeDoesNotCascadeToAnnotations(t *testing.T) {
	s := newTestStore(t)
	s.CreateAnnotation("t1", "wrong tone", "note")
	ax, _ := s.CreateAxialCode("Group", "", []string{"wrong tone"})

	if err := s.DeleteAxialCode(ax.ID); err != nil {
		t.Fatalf("DeleteAxialCode: %v", err)
	}

	// Not auto-deleted-and-gone: the underlying annotation survives, and
	// the label reverts to uncategorized rather than vanishing.
	if len(s.AnnotationsForTrace("t1")) != 1 {
		t.Fatalf("expected the OpenCodeAnnotation on t1 to survive axial code deletion")
	}
	summary := s.OpenCodesSummary()
	if len(summary) != 1 || summary[0].AxialCodeID != "" {
		t.Fatalf("summary = %+v, want the label uncategorized after axial code delete", summary)
	}

	err := s.DeleteAxialCode(ax.ID)
	var nf *ErrNotFound
	if !asStoreError(err, &nf) {
		t.Fatalf("err = %v, want ErrNotFound on double-delete", err)
	}
}

func TestAxialCodesSummaryTraceCountUnion(t *testing.T) {
	s := newTestStore(t)
	// t1 carries both member labels of the same axial code — must count
	// once (union, not sum).
	s.CreateAnnotation("t1", "hallucinated citation", "")
	s.CreateAnnotation("t1", "wrong citation format", "")
	s.CreateAnnotation("t2", "hallucinated citation", "")
	ax, _ := s.CreateAxialCode("Factual errors", "", []string{"hallucinated citation", "wrong citation format"})

	summary := s.AxialCodesSummary()
	var got *AxialCodeSummary
	for i := range summary {
		if summary[i].ID == ax.ID {
			got = &summary[i]
		}
	}
	if got == nil || got.TraceCount != 2 {
		t.Fatalf("summary = %+v, want TraceCount 2 (t1 counted once, t2 once)", got)
	}
}

func TestAxialCodeEmptyAfterLastMemberRemovedNotAutoDeleted(t *testing.T) {
	s := newTestStore(t)
	ax, _ := s.CreateAxialCode("Group", "", []string{"wrong tone"})
	s.UnassignMember(ax.ID, "wrong tone")

	summary := s.AxialCodesSummary()
	found := false
	for _, a := range summary {
		if a.ID == ax.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("axial code %q was auto-deleted after its last member was removed", ax.ID)
	}
}

func TestNormalizeLabel(t *testing.T) {
	cases := map[string]string{
		"  Hallucinated Citation  ": "hallucinated citation",
		"wrong tone":                "wrong tone",
		"WRONG TONE":                "wrong tone",
	}
	for in, want := range cases {
		if got := normalizeLabel(in); got != want {
			t.Errorf("normalizeLabel(%q) = %q, want %q", in, got, want)
		}
	}
}
