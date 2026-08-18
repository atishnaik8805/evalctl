package builtin

import (
	"encoding/json"
	"testing"

	"evalctl/internal/protocol"
)

func task(id string, metadata string) protocol.Task {
	return protocol.Task{ID: id, Input: json.RawMessage(`"x"`), Metadata: json.RawMessage(metadata)}
}

func transcript(id string, output string) protocol.Transcript {
	return protocol.Transcript{TaskID: id, Output: json.RawMessage(output)}
}

func TestExactMatch(t *testing.T) {
	cases := []struct {
		name       string
		metadata   string
		output     string
		wantScore  float64
		wantPassed bool
	}{
		{"match", `{"expected":"HELLO"}`, `"HELLO"`, 1.0, true},
		{"mismatch", `{"expected":"HELLO"}`, `"GOODBYE"`, 0.0, false},
		{"match ignores object key order", `{"expected":{"a":1,"b":2}}`, `{"b":2,"a":1}`, 1.0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := ExactMatch(task("t1", c.metadata), transcript("t1", c.output))
			if out.Score != c.wantScore {
				t.Errorf("Score = %v, want %v", out.Score, c.wantScore)
			}
			if out.Passed == nil || *out.Passed != c.wantPassed {
				t.Errorf("Passed = %v, want %v", out.Passed, c.wantPassed)
			}
		})
	}
}

func TestExactMatchMissingExpected(t *testing.T) {
	out := ExactMatch(task("t1", `{}`), transcript("t1", `"HELLO"`))
	if out.Error == nil {
		t.Fatal("expected an error outcome when metadata.expected is missing")
	}
}

func TestSchemaValidity(t *testing.T) {
	tsk := task("t1", `{}`)
	params := map[string]any{"required_fields": []any{"title", "steps"}}

	passOut := SchemaValidity(tsk, transcript("t1", `{"title":"x","steps":[1,2]}`), params)
	if passOut.Score != 1.0 || passOut.Passed == nil || !*passOut.Passed {
		t.Errorf("expected pass, got %+v", passOut)
	}

	failOut := SchemaValidity(tsk, transcript("t1", `{"title":"x"}`), params)
	if failOut.Score != 0.0 || failOut.Passed == nil || *failOut.Passed {
		t.Errorf("expected fail, got %+v", failOut)
	}
}

func TestSchemaValidityNonObjectOutput(t *testing.T) {
	tsk := task("t1", `{}`)
	params := map[string]any{"required_fields": []any{"title"}}
	out := SchemaValidity(tsk, transcript("t1", `"just a string"`), params)
	if out.Error == nil {
		t.Fatal("expected an error outcome for non-object output")
	}
}

func TestEntityGrounding(t *testing.T) {
	tsk := task("t1", `{"known_entities":["Paris","Eiffel Tower"]}`)

	fullOut := EntityGrounding(tsk, transcript("t1", `"I visited Paris and saw the Eiffel Tower"`))
	if fullOut.Score != 1.0 || fullOut.Passed == nil || !*fullOut.Passed {
		t.Errorf("expected full match, got %+v", fullOut)
	}

	partialOut := EntityGrounding(tsk, transcript("t1", `"I visited Paris"`))
	if partialOut.Score != 0.5 || partialOut.Passed == nil || *partialOut.Passed {
		t.Errorf("expected partial match score 0.5 and fail, got %+v", partialOut)
	}
}

func TestEntityGroundingMissingKnownEntities(t *testing.T) {
	out := EntityGrounding(task("t1", `{}`), transcript("t1", `"anything"`))
	if out.Error == nil {
		t.Fatal("expected an error outcome when metadata.known_entities is missing")
	}
}
