package analyze

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ErrNotFound reports that a referenced id (annotation or axial code)
// does not exist in the store.
type ErrNotFound struct {
	Kind string
	ID   string
}

func (e *ErrNotFound) Error() string {
	return fmt.Sprintf("%s %q not found", e.Kind, e.ID)
}

// ErrInvalidInput reports a request-level validation failure (e.g. an
// empty label or name).
type ErrInvalidInput struct {
	Msg string
}

func (e *ErrInvalidInput) Error() string { return e.Msg }

// ErrDuplicateLabel reports FR-004: a trace already carries an
// annotation whose normalized_label matches.
type ErrDuplicateLabel struct {
	ExistingID string
}

func (e *ErrDuplicateLabel) Error() string {
	return fmt.Sprintf("trace already has an annotation with this label (existing id %q)", e.ExistingID)
}

// asStoreError reports whether err's dynamic type is T, assigning it into
// target on success. A small stand-in for errors.As since these Store
// error types are always returned directly, never wrapped.
func asStoreError[T any](err error, target *T) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(T); ok {
		*target = e
		return true
	}
	return false
}

// storeVersion is the schema version this feature writes and expects
// (data-model.md AnnotationStore.version).
const storeVersion = 1

// OpenCodeAnnotation is one free-text label (plus optional note) attached
// to one AnalysisTrace (data-model.md OpenCodeAnnotation).
type OpenCodeAnnotation struct {
	ID              string `json:"id"`
	TraceID         string `json:"trace_id"`
	Label           string `json:"label"`
	NormalizedLabel string `json:"normalized_label"`
	Note            string `json:"note"`
	CreatedAt       string `json:"created_at"`
}

// AxialCode is a named, user-defined category grouping distinct
// open-code labels (data-model.md AxialCode).
type AxialCode struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	MemberLabels []string `json:"member_labels"`
}

// AnnotationStore is the JSON sidecar file holding all OpenCodeAnnotations
// and AxialCodes for one trace input file (data-model.md AnnotationStore).
type AnnotationStore struct {
	Version         int                  `json:"version"`
	OpenCodes       []OpenCodeAnnotation `json:"open_codes"`
	AxialCodes      []AxialCode          `json:"axial_codes"`
	NextOpenCodeID  int                  `json:"next_open_code_id"`
	NextAxialCodeID int                  `json:"next_axial_code_id"`
}

// Store wraps an AnnotationStore with the path it persists to and a mutex
// guarding every read-modify-write cycle (research.md §6).
type Store struct {
	mu    sync.Mutex
	path  string
	state AnnotationStore
}

// normalizeLabel implements FR-008's normalization: trim whitespace, then
// case-fold.
func normalizeLabel(label string) string {
	return strings.ToLower(strings.TrimSpace(label))
}

// Load reads the sidecar file at path if it exists, or starts a fresh
// empty store (data-model.md AnnotationStore Lifecycle). A malformed file
// or an unrecognized version fails fast rather than silently discarding
// existing annotations.
func Load(path string) (*Store, error) {
	s := &Store{
		path: path,
		state: AnnotationStore{
			Version:         storeVersion,
			OpenCodes:       []OpenCodeAnnotation{},
			AxialCodes:      []AxialCode{},
			NextOpenCodeID:  1,
			NextAxialCodeID: 1,
		},
	}

	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("open annotations %s: %w", path, err)
	}

	var loaded AnnotationStore
	if err := json.Unmarshal(b, &loaded); err != nil {
		return nil, fmt.Errorf("load annotations %s: malformed JSON: %w", path, err)
	}
	if loaded.Version != 0 && loaded.Version != storeVersion {
		return nil, fmt.Errorf("load annotations %s: unrecognized version %d (want %d)", path, loaded.Version, storeVersion)
	}
	if loaded.OpenCodes == nil {
		loaded.OpenCodes = []OpenCodeAnnotation{}
	}
	if loaded.AxialCodes == nil {
		loaded.AxialCodes = []AxialCode{}
	}
	if loaded.Version == 0 {
		loaded.Version = storeVersion
	}
	s.state = loaded
	return s, nil
}

// Save serializes the whole store and writes it to path via
// temp-file-then-rename (research.md §6) so a crash mid-write never
// leaves a torn file.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.save()
}

// save is the lock-free core of Save, used by mutation methods that
// already hold s.mu (FR-006: every mutation ends with a write-through).
func (s *Store) save() error {
	b, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal annotations: %w", err)
	}

	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, ".annotations-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp annotations file: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp annotations file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp annotations file: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename annotations file: %w", err)
	}
	return nil
}

// AnnotationsForTrace returns traceID's OpenCodeAnnotations, in the
// store's insertion order.
func (s *Store) AnnotationsForTrace(traceID string) []OpenCodeAnnotation {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []OpenCodeAnnotation
	for _, a := range s.state.OpenCodes {
		if a.TraceID == traceID {
			out = append(out, a)
		}
	}
	return out
}

// OrphanedAnnotations returns every OpenCodeAnnotation whose trace_id is
// not a key of knownTraceIDs (spec.md Edge Cases: sidecar references a
// trace id no longer present in the loaded trace set).
func (s *Store) OrphanedAnnotations(knownTraceIDs map[string]int) []OpenCodeAnnotation {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []OpenCodeAnnotation
	for _, a := range s.state.OpenCodes {
		if _, ok := knownTraceIDs[a.TraceID]; !ok {
			out = append(out, a)
		}
	}
	return out
}

// CreateAnnotation attaches a new open-code label (plus optional note) to
// traceID, rejecting an empty label and a duplicate normalized label
// already present on that trace (FR-004).
func (s *Store) CreateAnnotation(traceID, label, note string) (OpenCodeAnnotation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalized := normalizeLabel(label)
	if normalized == "" {
		return OpenCodeAnnotation{}, &ErrInvalidInput{Msg: "label must not be empty"}
	}
	for _, a := range s.state.OpenCodes {
		if a.TraceID == traceID && a.NormalizedLabel == normalized {
			return OpenCodeAnnotation{}, &ErrDuplicateLabel{ExistingID: a.ID}
		}
	}

	ann := OpenCodeAnnotation{
		ID:              fmt.Sprintf("oc-%d", s.state.NextOpenCodeID),
		TraceID:         traceID,
		Label:           label,
		NormalizedLabel: normalized,
		Note:            note,
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
	}
	s.state.NextOpenCodeID++
	s.state.OpenCodes = append(s.state.OpenCodes, ann)

	if err := s.save(); err != nil {
		return OpenCodeAnnotation{}, err
	}
	return ann, nil
}

// UpdateAnnotation replaces id's label and note, rejecting an empty label
// and a duplicate normalized label on the same trace's *other*
// annotations (FR-004).
func (s *Store) UpdateAnnotation(id, label, note string) (OpenCodeAnnotation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := -1
	for i, a := range s.state.OpenCodes {
		if a.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return OpenCodeAnnotation{}, &ErrNotFound{Kind: "annotation", ID: id}
	}

	normalized := normalizeLabel(label)
	if normalized == "" {
		return OpenCodeAnnotation{}, &ErrInvalidInput{Msg: "label must not be empty"}
	}
	traceID := s.state.OpenCodes[idx].TraceID
	for i, a := range s.state.OpenCodes {
		if i != idx && a.TraceID == traceID && a.NormalizedLabel == normalized {
			return OpenCodeAnnotation{}, &ErrDuplicateLabel{ExistingID: a.ID}
		}
	}

	s.state.OpenCodes[idx].Label = label
	s.state.OpenCodes[idx].NormalizedLabel = normalized
	s.state.OpenCodes[idx].Note = note

	if err := s.save(); err != nil {
		return OpenCodeAnnotation{}, err
	}
	return s.state.OpenCodes[idx], nil
}

// DeleteAnnotation removes id, never cascading to axial-code membership
// (contracts/http-api.md).
func (s *Store) DeleteAnnotation(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := -1
	for i, a := range s.state.OpenCodes {
		if a.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return &ErrNotFound{Kind: "annotation", ID: id}
	}
	s.state.OpenCodes = append(s.state.OpenCodes[:idx], s.state.OpenCodes[idx+1:]...)

	return s.save()
}

// OpenCodeSummary is one row of the aggregated "open codes" view
// (FR-008, data-model.md OpenCodeAnnotation "Derived").
type OpenCodeSummary struct {
	NormalizedLabel string
	DisplayLabel    string
	Count           int
	AxialCodeID     string // "" when uncategorized (FR-010)
}

// OpenCodesSummary groups every OpenCodeAnnotation by normalized_label:
// DisplayLabel is the earliest-created annotation's as-typed label, and
// Count is the number of distinct trace ids carrying that label (FR-008).
func (s *Store) OpenCodesSummary() []OpenCodeSummary {
	s.mu.Lock()
	defer s.mu.Unlock()

	order := []string{}
	display := map[string]string{}
	traceIDs := map[string]map[string]bool{}
	for _, a := range s.state.OpenCodes {
		if _, ok := display[a.NormalizedLabel]; !ok {
			order = append(order, a.NormalizedLabel)
			display[a.NormalizedLabel] = a.Label
			traceIDs[a.NormalizedLabel] = map[string]bool{}
		}
		traceIDs[a.NormalizedLabel][a.TraceID] = true
	}

	axialOf := map[string]string{}
	for _, ax := range s.state.AxialCodes {
		for _, label := range ax.MemberLabels {
			axialOf[label] = ax.ID
		}
	}

	out := make([]OpenCodeSummary, 0, len(order))
	for _, label := range order {
		out = append(out, OpenCodeSummary{
			NormalizedLabel: label,
			DisplayLabel:    display[label],
			Count:           len(traceIDs[label]),
			AxialCodeID:     axialOf[label],
		})
	}
	return out
}

// CreateAxialCode creates a new AxialCode (FR-009: member_labels is
// optional, defaulting to no members). Any requested member label
// currently belonging to a different axial code is moved (single
// membership) as part of this same call.
func (s *Store) CreateAxialCode(name, description string, memberLabels []string) (AxialCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	name = strings.TrimSpace(name)
	if name == "" {
		return AxialCode{}, &ErrInvalidInput{Msg: "name must not be empty"}
	}

	members := dedupeNormalized(memberLabels)
	ax := AxialCode{
		ID:           fmt.Sprintf("ax-%d", s.state.NextAxialCodeID),
		Name:         name,
		Description:  description,
		MemberLabels: []string{},
	}
	s.state.NextAxialCodeID++
	s.state.AxialCodes = append(s.state.AxialCodes, ax)
	idx := len(s.state.AxialCodes) - 1

	for _, label := range members {
		s.reassignMemberLocked(idx, label)
	}

	if err := s.save(); err != nil {
		return AxialCode{}, err
	}
	return s.state.AxialCodes[idx], nil
}

// RenameAxialCode replaces id's name and description only; membership is
// changed via AssignMember/UnassignMember.
func (s *Store) RenameAxialCode(id, name, description string) (AxialCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.axialCodeIndexLocked(id)
	if idx == -1 {
		return AxialCode{}, &ErrNotFound{Kind: "axial code", ID: id}
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return AxialCode{}, &ErrInvalidInput{Msg: "name must not be empty"}
	}

	s.state.AxialCodes[idx].Name = name
	s.state.AxialCodes[idx].Description = description

	if err := s.save(); err != nil {
		return AxialCode{}, err
	}
	return s.state.AxialCodes[idx], nil
}

// AssignMember assigns normalizedLabel to axialCodeID, atomically removing
// it from whichever axial code it currently belongs to (single
// membership, per clarification).
func (s *Store) AssignMember(axialCodeID, normalizedLabel string) (AxialCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.axialCodeIndexLocked(axialCodeID)
	if idx == -1 {
		return AxialCode{}, &ErrNotFound{Kind: "axial code", ID: axialCodeID}
	}
	s.reassignMemberLocked(idx, normalizeLabel(normalizedLabel))

	if err := s.save(); err != nil {
		return AxialCode{}, err
	}
	return s.state.AxialCodes[idx], nil
}

// UnassignMember removes normalizedLabel from axialCodeID's members; the
// label reverts to uncategorized (contracts/http-api.md).
func (s *Store) UnassignMember(axialCodeID, normalizedLabel string) (AxialCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.axialCodeIndexLocked(axialCodeID)
	if idx == -1 {
		return AxialCode{}, &ErrNotFound{Kind: "axial code", ID: axialCodeID}
	}
	normalized := normalizeLabel(normalizedLabel)
	members := s.state.AxialCodes[idx].MemberLabels
	pos := -1
	for i, l := range members {
		if l == normalized {
			pos = i
			break
		}
	}
	if pos == -1 {
		return AxialCode{}, &ErrNotFound{Kind: "axial code member", ID: normalizedLabel}
	}
	s.state.AxialCodes[idx].MemberLabels = append(members[:pos], members[pos+1:]...)

	if err := s.save(); err != nil {
		return AxialCode{}, err
	}
	return s.state.AxialCodes[idx], nil
}

// DeleteAxialCode removes id. Its former member labels become
// uncategorized; underlying OpenCodeAnnotations are never touched
// (FR-009).
func (s *Store) DeleteAxialCode(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.axialCodeIndexLocked(id)
	if idx == -1 {
		return &ErrNotFound{Kind: "axial code", ID: id}
	}
	s.state.AxialCodes = append(s.state.AxialCodes[:idx], s.state.AxialCodes[idx+1:]...)

	return s.save()
}

// axialCodeIndexLocked returns id's index in s.state.AxialCodes, or -1.
// Callers must hold s.mu.
func (s *Store) axialCodeIndexLocked(id string) int {
	for i, ax := range s.state.AxialCodes {
		if ax.ID == id {
			return i
		}
	}
	return -1
}

// reassignMemberLocked adds label to s.state.AxialCodes[idx]'s members,
// first removing it from whichever axial code (including idx itself, a
// no-op) currently holds it, enforcing single membership. Callers must
// hold s.mu.
func (s *Store) reassignMemberLocked(idx int, label string) {
	for i := range s.state.AxialCodes {
		members := s.state.AxialCodes[i].MemberLabels
		for j, l := range members {
			if l == label {
				s.state.AxialCodes[i].MemberLabels = append(members[:j], members[j+1:]...)
				break
			}
		}
	}
	s.state.AxialCodes[idx].MemberLabels = append(s.state.AxialCodes[idx].MemberLabels, label)
}

// dedupeNormalized normalizes and de-duplicates a requested member-label
// list, preserving first-seen order.
func dedupeNormalized(labels []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(labels))
	for _, l := range labels {
		n := normalizeLabel(l)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return out
}

// AxialCodeSummary is one row of GET /api/axial-codes
// (contracts/http-api.md): an AxialCode plus its server-computed
// TraceCount (data-model.md AxialCode "Derived").
type AxialCodeSummary struct {
	ID           string
	Name         string
	Description  string
	MemberLabels []string
	TraceCount   int
}

// UncategorizedLabelSamples returns every currently-uncategorized
// normalized label with a small sample of its annotation notes, for the
// LLM-suggest request body (research.md §7; only uncategorized labels
// are ever sent, per clarification).
func (s *Store) UncategorizedLabelSamples() []LabelSample {
	s.mu.Lock()
	defer s.mu.Unlock()

	categorized := map[string]bool{}
	for _, ax := range s.state.AxialCodes {
		for _, l := range ax.MemberLabels {
			categorized[l] = true
		}
	}

	const maxSampleNotes = 3
	order := []string{}
	notes := map[string][]string{}
	for _, a := range s.state.OpenCodes {
		if categorized[a.NormalizedLabel] {
			continue
		}
		if _, ok := notes[a.NormalizedLabel]; !ok {
			order = append(order, a.NormalizedLabel)
			notes[a.NormalizedLabel] = []string{}
		}
		if a.Note != "" && len(notes[a.NormalizedLabel]) < maxSampleNotes {
			notes[a.NormalizedLabel] = append(notes[a.NormalizedLabel], a.Note)
		}
	}

	out := make([]LabelSample, 0, len(order))
	for _, label := range order {
		out = append(out, LabelSample{Label: label, SampleNotes: notes[label]})
	}
	return out
}

// AxialCodesSummary returns every AxialCode with its distinct-trace
// union count across member labels (a trace with two member labels of
// the same axial code counts once).
func (s *Store) AxialCodesSummary() []AxialCodeSummary {
	s.mu.Lock()
	defer s.mu.Unlock()

	labelTraces := map[string]map[string]bool{}
	for _, a := range s.state.OpenCodes {
		if labelTraces[a.NormalizedLabel] == nil {
			labelTraces[a.NormalizedLabel] = map[string]bool{}
		}
		labelTraces[a.NormalizedLabel][a.TraceID] = true
	}

	out := make([]AxialCodeSummary, 0, len(s.state.AxialCodes))
	for _, ax := range s.state.AxialCodes {
		union := map[string]bool{}
		for _, label := range ax.MemberLabels {
			for tid := range labelTraces[label] {
				union[tid] = true
			}
		}
		out = append(out, AxialCodeSummary{
			ID:           ax.ID,
			Name:         ax.Name,
			Description:  ax.Description,
			MemberLabels: append([]string{}, ax.MemberLabels...),
			TraceCount:   len(union),
		})
	}
	return out
}
