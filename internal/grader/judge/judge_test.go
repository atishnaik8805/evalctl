package judge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"evalctl/internal/protocol"
)

func testTask() protocol.Task {
	return protocol.Task{ID: "t1", Input: json.RawMessage(`"hi"`), Metadata: json.RawMessage(`{}`)}
}

func testTranscript() protocol.Transcript {
	return protocol.Transcript{TaskID: "t1", Output: json.RawMessage(`"the output"`)}
}

func TestGradeSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("server: decode request: %v", err)
		}
		if _, ok := body["rubric"]; !ok {
			t.Error("server: request missing rubric")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"score": 0.85, "reasoning": "good"})
	}))
	defer srv.Close()

	params := Params{Rubric: "test rubric", JudgeEndpoint: srv.URL}
	out := Grade(context.Background(), testTask(), testTranscript(), params, 5*time.Second)

	if out.Error != nil {
		t.Fatalf("unexpected error: %v", *out.Error)
	}
	if out.Score != 0.85 {
		t.Errorf("Score = %v, want 0.85", out.Score)
	}
	if out.Reasoning == nil || *out.Reasoning != "good" {
		t.Errorf("Reasoning = %v, want \"good\"", out.Reasoning)
	}
}

func TestGradeAuthHeader(t *testing.T) {
	os.Setenv("TEST_JUDGE_KEY", "secret123")
	defer os.Unsetenv("TEST_JUDGE_KEY")

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		json.NewEncoder(w).Encode(map[string]any{"score": 1.0, "reasoning": ""})
	}))
	defer srv.Close()

	params := Params{Rubric: "r", JudgeEndpoint: srv.URL, APIKeyEnv: "TEST_JUDGE_KEY"}
	Grade(context.Background(), testTask(), testTranscript(), params, 5*time.Second)

	if gotAuth != "Bearer secret123" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer secret123")
	}
}

func TestGradeNon2xxStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	params := Params{Rubric: "r", JudgeEndpoint: srv.URL}
	out := Grade(context.Background(), testTask(), testTranscript(), params, 5*time.Second)
	if out.Error == nil || *out.Error != protocol.ErrJudgeEndpointErr {
		t.Fatalf("Error = %v, want %q", out.Error, protocol.ErrJudgeEndpointErr)
	}
}

func TestGradeMalformedResponseBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	params := Params{Rubric: "r", JudgeEndpoint: srv.URL}
	out := Grade(context.Background(), testTask(), testTranscript(), params, 5*time.Second)
	if out.Error == nil || *out.Error != protocol.ErrJudgeEndpointErr {
		t.Fatalf("Error = %v, want %q", out.Error, protocol.ErrJudgeEndpointErr)
	}
}

func TestGradeScoreOutOfRange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"score": 1.5, "reasoning": "x"})
	}))
	defer srv.Close()

	params := Params{Rubric: "r", JudgeEndpoint: srv.URL}
	out := Grade(context.Background(), testTask(), testTranscript(), params, 5*time.Second)
	if out.Error == nil {
		t.Fatal("expected error for out-of-range score")
	}
}

func TestGradeMissingScore(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"reasoning": "x"})
	}))
	defer srv.Close()

	params := Params{Rubric: "r", JudgeEndpoint: srv.URL}
	out := Grade(context.Background(), testTask(), testTranscript(), params, 5*time.Second)
	if out.Error == nil {
		t.Fatal("expected error for missing score")
	}
}

func TestGradeTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		json.NewEncoder(w).Encode(map[string]any{"score": 1.0, "reasoning": ""})
	}))
	defer srv.Close()

	params := Params{Rubric: "r", JudgeEndpoint: srv.URL}
	out := Grade(context.Background(), testTask(), testTranscript(), params, 50*time.Millisecond)
	if out.Error == nil || *out.Error != protocol.ErrTimeout {
		t.Fatalf("Error = %v, want %q", out.Error, protocol.ErrTimeout)
	}
}

func TestParseParamsRequiresEndpoint(t *testing.T) {
	if _, err := ParseParams(map[string]any{"rubric": "x"}); err == nil {
		t.Fatal("expected error when judge_endpoint is missing")
	}
}

func TestParseParamsOK(t *testing.T) {
	p, err := ParseParams(map[string]any{"rubric": "x", "judge_endpoint": "http://localhost", "api_key_env": "KEY"})
	if err != nil {
		t.Fatalf("ParseParams: %v", err)
	}
	if p.Rubric != "x" || p.JudgeEndpoint != "http://localhost" || p.APIKeyEnv != "KEY" {
		t.Errorf("got %+v", p)
	}
}
