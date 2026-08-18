package protocol

import (
	"encoding/json"
	"testing"
)

func TestTaskMarshalRoundTrip(t *testing.T) {
	in := Task{ID: "t1", Input: json.RawMessage(`"hello"`), Metadata: json.RawMessage(`{"expected":"HELLO"}`)}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out Task
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.ID != in.ID {
		t.Errorf("ID = %q, want %q", out.ID, in.ID)
	}
	if string(out.Input) != string(in.Input) {
		t.Errorf("Input = %s, want %s", out.Input, in.Input)
	}
}

func TestTaskValidate(t *testing.T) {
	cases := []struct {
		name    string
		task    Task
		wantErr bool
	}{
		{"valid", Task{ID: "a", Input: json.RawMessage(`1`)}, false},
		{"missing id", Task{Input: json.RawMessage(`1`)}, true},
		{"missing input", Task{ID: "a"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.task.Validate()
			if (err != nil) != c.wantErr {
				t.Errorf("Validate() err = %v, wantErr %v", err, c.wantErr)
			}
		})
	}
}

func TestTranscriptHasSteps(t *testing.T) {
	withSteps := Transcript{TaskID: "t1", Steps: []Step{{Index: 0}}}
	withoutSteps := Transcript{TaskID: "t1"}
	if !withSteps.HasSteps() {
		t.Error("expected HasSteps() true")
	}
	if withoutSteps.HasSteps() {
		t.Error("expected HasSteps() false")
	}
}

func TestOutcomeMarshalRoundTrip(t *testing.T) {
	passed := true
	reasoning := "looks good"
	in := Outcome{TaskID: "t1", GraderName: "exact_match", Score: 1.0, Passed: &passed, Reasoning: &reasoning}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out Outcome
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Score != in.Score || *out.Passed != *in.Passed || *out.Reasoning != *in.Reasoning {
		t.Errorf("round trip mismatch: got %+v", out)
	}
}

func TestFailingOutcome(t *testing.T) {
	o := FailingOutcome("t1", "exact_match", ErrConsumerError)
	if o.Score != 0.0 {
		t.Errorf("Score = %v, want 0.0", o.Score)
	}
	if o.Passed == nil || *o.Passed != false {
		t.Errorf("Passed = %v, want false", o.Passed)
	}
	if o.Error == nil || *o.Error != ErrConsumerError {
		t.Errorf("Error = %v, want %q", o.Error, ErrConsumerError)
	}
}

func TestConsumerRequestEnvelope(t *testing.T) {
	req := ConsumerRequest{ProtocolVersion: ProtocolVersion, Task: Task{ID: "t1", Input: json.RawMessage(`"x"`)}}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := m["protocol_version"]; !ok {
		t.Error("missing protocol_version field")
	}
	if _, ok := m["task"]; !ok {
		t.Error("missing task field")
	}
}
