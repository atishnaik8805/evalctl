package analyze

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

// suggestTimeout bounds the LLM-suggest HTTP call (research.md §7).
const suggestTimeout = 30 * time.Second

// LabelSample is one currently-uncategorized normalized label plus a
// small sample of its annotation notes, sent to the suggest endpoint.
type LabelSample struct {
	Label       string
	SampleNotes []string
}

// SuggestedGrouping is one proposed axial-code grouping returned by the
// suggest endpoint (contracts/http-api.md POST /api/axial-codes/suggest).
type SuggestedGrouping struct {
	Name   string   `json:"name"`
	Labels []string `json:"labels"`
}

type suggestLabelRequest struct {
	Label       string   `json:"label"`
	SampleNotes []string `json:"sample_notes"`
}

type suggestRequest struct {
	ProtocolVersion string                `json:"protocol_version"`
	Labels          []suggestLabelRequest `json:"labels"`
}

type suggestResponse struct {
	Groupings []SuggestedGrouping `json:"groupings"`
}

// Suggest POSTs labels to endpoint (research.md §7, structurally parallel
// to internal/grader/judge) and returns the proposed groupings. Any
// transport error, non-2xx, or malformed response is returned as a
// descriptive error — never a panic, never a partial write anywhere
// (the caller decides whether/how to persist the result).
func Suggest(ctx context.Context, endpoint, apiKeyEnv string, labels []LabelSample) ([]SuggestedGrouping, error) {
	reqLabels := make([]suggestLabelRequest, 0, len(labels))
	for _, l := range labels {
		reqLabels = append(reqLabels, suggestLabelRequest{Label: l.Label, SampleNotes: l.SampleNotes})
	}
	reqBody := suggestRequest{ProtocolVersion: protocol.ProtocolVersion, Labels: reqLabels}
	b, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("suggest: encode request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, suggestTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("suggest: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKeyEnv != "" {
		if key := os.Getenv(apiKeyEnv); key != "" {
			httpReq.Header.Set("Authorization", "Bearer "+key)
		}
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("suggest: no response before timeout elapsed")
		}
		return nil, fmt.Errorf("suggest: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("suggest: non-2xx status: %d", resp.StatusCode)
	}

	var respBody suggestResponse
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, fmt.Errorf("suggest: malformed response body: %w", err)
	}
	return respBody.Groupings, nil
}
