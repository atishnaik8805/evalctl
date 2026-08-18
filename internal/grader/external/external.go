// Package external invokes custom grader executables over the stdio
// protocol's grader role (contracts/stdio-protocol.md), sharing the same
// subprocess mechanism used for consumer invocation (internal/runner).
package external

import (
	"context"
	"encoding/json"
	"time"

	"evalctl/internal/protocol"
	"evalctl/internal/runner"
)

// Grade invokes a custom grader executable at execPath with the Task and
// Transcript to grade, and returns its Outcome. Only score/passed/
// reasoning/evidence/error are trusted from the grader's response;
// task_id/grader_name are forced to the values the framework already
// knows. Non-zero exit, malformed stdout, or timeout produce a failing
// Outcome (FR-018) rather than a Go error.
func Grade(ctx context.Context, label, execPath string, task protocol.Task, transcript protocol.Transcript, timeout time.Duration) protocol.Outcome {
	reqBody := protocol.GraderRequest{ProtocolVersion: protocol.ProtocolVersion, Task: task, Transcript: transcript}
	reqLine, _ := json.Marshal(reqBody)

	res := runner.Call(ctx, execPath, reqLine, timeout)
	if res.FailureReason == "timeout" {
		return protocol.FailingOutcome(task.ID, label, protocol.ErrTimeout)
	}
	if res.FailureReason != "" {
		return protocol.FailingOutcome(task.ID, label, protocol.ErrGraderError)
	}

	var out protocol.Outcome
	if err := json.Unmarshal(res.Line, &out); err != nil {
		return protocol.FailingOutcome(task.ID, label, protocol.ErrGraderError)
	}
	out.TaskID = task.ID
	out.GraderName = label
	return out
}
