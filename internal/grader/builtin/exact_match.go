// Package builtin implements the framework's built-in graders (FR-015):
// exact_match, schema_validity, entity_grounding.
package builtin

import (
	"encoding/json"

	"evalctl/internal/protocol"
)

// ExactMatch scores 1.0 if transcript.output equals task.metadata.expected
// (compared as decoded JSON values), else 0.0.
func ExactMatch(task protocol.Task, transcript protocol.Transcript) protocol.Outcome {
	const name = "exact_match"

	var meta struct {
		Expected json.RawMessage `json:"expected"`
	}
	if len(task.Metadata) > 0 {
		_ = json.Unmarshal(task.Metadata, &meta)
	}
	if len(meta.Expected) == 0 {
		reason := "grader_error"
		o := protocol.FailingOutcome(task.ID, name, reason)
		msg := "task.metadata.expected is required for exact_match"
		o.Reasoning = &msg
		return o
	}

	matched := jsonEqual(meta.Expected, transcript.Output)
	score := 0.0
	if matched {
		score = 1.0
	}
	passed := matched
	return protocol.Outcome{
		TaskID:     task.ID,
		GraderName: name,
		Score:      score,
		Passed:     &passed,
	}
}

// jsonEqual compares two json.RawMessage values by their decoded form, so
// formatting differences (whitespace, key order) don't cause false negatives.
func jsonEqual(a, b json.RawMessage) bool {
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		return false
	}
	ab, _ := json.Marshal(av)
	bb, _ := json.Marshal(bv)
	return string(ab) == string(bb)
}
