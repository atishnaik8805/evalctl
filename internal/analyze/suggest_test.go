package analyze

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSuggestHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req suggestRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(req.Labels) != 1 || req.Labels[0].Label != "wrong tone" {
			t.Fatalf("request.Labels = %+v", req.Labels)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(suggestResponse{
			Groupings: []SuggestedGrouping{{Name: "Tone issues", Labels: []string{"wrong tone"}}},
		})
	}))
	defer srv.Close()

	groupings, err := Suggest(context.Background(), srv.URL, "", []LabelSample{{Label: "wrong tone"}})
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(groupings) != 1 || groupings[0].Name != "Tone issues" {
		t.Fatalf("groupings = %+v", groupings)
	}
}

func TestSuggestNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := Suggest(context.Background(), srv.URL, "", nil)
	if err == nil {
		t.Fatal("expected an error on a non-2xx response")
	}
}

func TestSuggestMalformedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	_, err := Suggest(context.Background(), srv.URL, "", nil)
	if err == nil {
		t.Fatal("expected an error on a malformed response body")
	}
}

func TestSuggestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := Suggest(ctx, srv.URL, "", nil)
	if err == nil {
		t.Fatal("expected an error on timeout")
	}
}

func TestSuggestSendsAuthorizationHeader(t *testing.T) {
	t.Setenv("SUGGEST_TEST_KEY", "secret-token")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret-token" {
			t.Errorf("Authorization header = %q, want \"Bearer secret-token\"", got)
		}
		json.NewEncoder(w).Encode(suggestResponse{})
	}))
	defer srv.Close()

	if _, err := Suggest(context.Background(), srv.URL, "SUGGEST_TEST_KEY", nil); err != nil {
		t.Fatalf("Suggest: %v", err)
	}
}
