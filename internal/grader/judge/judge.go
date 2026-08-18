// Package judge implements the llm_judge built-in grader: an HTTP client
// speaking the minimal contract in contracts/judge-http-contract.md.
package judge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"evalctl/internal/protocol"
)

const GraderName = "llm_judge"

type request struct {
	ProtocolVersion string          `json:"protocol_version"`
	Rubric          string          `json:"rubric"`
	Task            protocol.Task   `json:"task"`
	Output          json.RawMessage `json:"output"`
}

type response struct {
	Score     *float64 `json:"score"`
	Reasoning *string  `json:"reasoning"`
}

// Params is the parsed params block for an llm_judge GraderConfig entry
// (contracts/suite-config-schema.md).
type Params struct {
	Rubric        string `json:"rubric"`
	JudgeEndpoint string `json:"judge_endpoint"`
	APIKeyEnv     string `json:"api_key_env"`
}

// ParseParams decodes a raw params map into Params.
func ParseParams(raw map[string]any) (Params, error) {
	b, err := json.Marshal(raw)
	if err != nil {
		return Params{}, err
	}
	var p Params
	if err := json.Unmarshal(b, &p); err != nil {
		return Params{}, err
	}
	if p.JudgeEndpoint == "" {
		return Params{}, fmt.Errorf("llm_judge: params.judge_endpoint is required")
	}
	return p, nil
}

// Grade calls the judge_endpoint per contracts/judge-http-contract.md and
// converts the response into an Outcome, or a failing Outcome per FR-018
// on any transport/response error.
func Grade(ctx context.Context, task protocol.Task, transcript protocol.Transcript, params Params, timeout time.Duration) protocol.Outcome {
	fail := func(reason, detail string) protocol.Outcome {
		o := protocol.FailingOutcome(task.ID, GraderName, reason)
		if detail != "" {
			o.Reasoning = &detail
		}
		return o
	}

	reqBody := request{
		ProtocolVersion: protocol.ProtocolVersion,
		Rubric:          params.Rubric,
		Task:            task,
		Output:          transcript.Output,
	}
	b, err := json.Marshal(reqBody)
	if err != nil {
		return fail(protocol.ErrJudgeEndpointErr, "failed to encode request: "+err.Error())
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, params.JudgeEndpoint, bytes.NewReader(b))
	if err != nil {
		return fail(protocol.ErrJudgeEndpointErr, "failed to build request: "+err.Error())
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if params.APIKeyEnv != "" {
		if key := os.Getenv(params.APIKeyEnv); key != "" {
			httpReq.Header.Set("Authorization", "Bearer "+key)
		}
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fail(protocol.ErrTimeout, "no response before timeout elapsed")
		}
		return fail(protocol.ErrJudgeEndpointErr, "request failed: "+err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fail(protocol.ErrJudgeEndpointErr, fmt.Sprintf("non-2xx status: %d", resp.StatusCode))
	}

	var respBody response
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return fail(protocol.ErrJudgeEndpointErr, "malformed response body: "+err.Error())
	}
	if respBody.Score == nil || *respBody.Score < 0.0 || *respBody.Score > 1.0 {
		return fail(protocol.ErrJudgeEndpointErr, "response.score missing or out of [0.0, 1.0] range")
	}
	if respBody.Reasoning == nil {
		return fail(protocol.ErrJudgeEndpointErr, "response.reasoning missing")
	}

	return protocol.Outcome{
		TaskID:     task.ID,
		GraderName: GraderName,
		Score:      *respBody.Score,
		Reasoning:  respBody.Reasoning,
	}
}
