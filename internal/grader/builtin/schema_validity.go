package builtin

import (
	"encoding/json"

	"evalctl/internal/protocol"
)

// SchemaValidity scores 1.0 if transcript.output is a JSON object containing
// every field named in params.required_fields, else 0.0, listing the
// missing fields as evidence.
func SchemaValidity(task protocol.Task, transcript protocol.Transcript, params map[string]any) protocol.Outcome {
	const name = "schema_validity"

	requiredFields := stringSlice(params["required_fields"])

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(transcript.Output, &obj); err != nil {
		o := protocol.FailingOutcome(task.ID, name, "grader_error")
		msg := "transcript.output is not a JSON object"
		o.Reasoning = &msg
		return o
	}

	var missing []string
	for _, f := range requiredFields {
		if _, ok := obj[f]; !ok {
			missing = append(missing, f)
		}
	}

	passed := len(missing) == 0
	score := 0.0
	if passed {
		score = 1.0
	}
	evidence, _ := json.Marshal(map[string]any{"missing_fields": missing})
	return protocol.Outcome{
		TaskID:     task.ID,
		GraderName: name,
		Score:      score,
		Passed:     &passed,
		Evidence:   evidence,
	}
}

func stringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
