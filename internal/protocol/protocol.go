// Package protocol defines the wire types shared by evalctl, evaluated
// systems, and custom graders (contracts/stdio-protocol.md).
package protocol

import "encoding/json"

const ProtocolVersion = "1.0"

// Task is a single unit of evaluation input (data-model.md Task).
type Task struct {
	ID       string          `json:"id"`
	Input    json.RawMessage `json:"input"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// Validate checks the required fields per data-model.md Task validation.
func (t Task) Validate() error {
	if t.ID == "" {
		return errMissingField("id")
	}
	if len(t.Input) == 0 {
		return errMissingField("input")
	}
	return nil
}

// Step is one state/action/result unit within a trajectory (data-model.md Step).
type Step struct {
	Index  int             `json:"index"`
	State  json.RawMessage `json:"state"`
	Action json.RawMessage `json:"action"`
	Result json.RawMessage `json:"result"`
}

// Transcript is the evaluated system's response to one Task (data-model.md Transcript).
type Transcript struct {
	TaskID string          `json:"task_id"`
	Output json.RawMessage `json:"output"`
	Steps  []Step          `json:"steps,omitempty"`
	Raw    json.RawMessage `json:"raw,omitempty"`
}

// HasSteps reports whether this transcript carries trajectory data.
func (t Transcript) HasSteps() bool {
	return len(t.Steps) > 0
}

// Outcome is the result of grading one Task with one grader (data-model.md Outcome).
type Outcome struct {
	TaskID     string          `json:"task_id"`
	GraderName string          `json:"grader_name"`
	Score      float64         `json:"score"`
	Passed     *bool           `json:"passed,omitempty"`
	Reasoning  *string         `json:"reasoning,omitempty"`
	Evidence   json.RawMessage `json:"evidence,omitempty"`
	Error      *string         `json:"error,omitempty"`
}

// Known error reasons (data-model.md Outcome.error, contracts/stdio-protocol.md).
const (
	ErrConsumerError    = "consumer_error"
	ErrGraderError      = "grader_error"
	ErrTimeout          = "timeout"
	ErrNoTrajectoryData = "no trajectory data"
	ErrJudgeEndpointErr = "judge_endpoint_error"
)

// FailingOutcome builds a synthetic failing Outcome for a task-grader pair,
// per the failure-handling rules in contracts/stdio-protocol.md.
func FailingOutcome(taskID, graderName, reason string) Outcome {
	f := false
	return Outcome{
		TaskID:     taskID,
		GraderName: graderName,
		Score:      0.0,
		Passed:     &f,
		Error:      &reason,
	}
}

// ConsumerRequest is the envelope sent to an evaluated system's stdin
// (contracts/stdio-protocol.md Request envelope).
type ConsumerRequest struct {
	ProtocolVersion string `json:"protocol_version"`
	Task            Task   `json:"task"`
}

// GraderRequest is the envelope sent to a custom grader's stdin
// (contracts/stdio-protocol.md Request envelope, custom-grader role).
type GraderRequest struct {
	ProtocolVersion string     `json:"protocol_version"`
	Task            Task       `json:"task"`
	Transcript      Transcript `json:"transcript"`
}

type fieldError struct {
	field string
}

func (e *fieldError) Error() string {
	return "missing required field: " + e.field
}

func errMissingField(field string) error {
	return &fieldError{field: field}
}
