package builtin

import (
	"encoding/json"
	"strings"

	"evalctl/internal/protocol"
)

// EntityGrounding scores the fraction of task.metadata.known_entities that
// appear as substrings of transcript.output (stringified), passing only
// when every known entity is present.
func EntityGrounding(task protocol.Task, transcript protocol.Transcript) protocol.Outcome {
	const name = "entity_grounding"

	var meta struct {
		KnownEntities []string `json:"known_entities"`
	}
	if len(task.Metadata) > 0 {
		_ = json.Unmarshal(task.Metadata, &meta)
	}
	if len(meta.KnownEntities) == 0 {
		o := protocol.FailingOutcome(task.ID, name, "grader_error")
		msg := "task.metadata.known_entities is required for entity_grounding"
		o.Reasoning = &msg
		return o
	}

	outputText := stringify(transcript.Output)

	var found, missing []string
	for _, e := range meta.KnownEntities {
		if strings.Contains(outputText, e) {
			found = append(found, e)
		} else {
			missing = append(missing, e)
		}
	}

	score := float64(len(found)) / float64(len(meta.KnownEntities))
	passed := len(missing) == 0
	evidence, _ := json.Marshal(map[string]any{"matched_entities": found, "missing_entities": missing})
	return protocol.Outcome{
		TaskID:     task.ID,
		GraderName: name,
		Score:      score,
		Passed:     &passed,
		Evidence:   evidence,
	}
}

func stringify(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}
