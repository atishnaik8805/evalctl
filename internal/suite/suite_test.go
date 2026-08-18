package suite

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return p
}

func validSuiteDir(t *testing.T) (dir string, suitePath string) {
	t.Helper()
	dir = t.TempDir()
	writeFile(t, dir, "system.py", "#!/usr/bin/env python3\n")
	writeFile(t, dir, "tasks.jsonl", `{"id":"a","input":"hi"}`+"\n")
	suitePath = writeFile(t, dir, "suite.yaml", `
name: my-suite
system: ./system.py
dataset: tasks.jsonl
fail_below: 0.8
graders:
  - name: exact_match
    params: {}
`)
	return dir, suitePath
}

func TestLoadAndValidateValidSuite(t *testing.T) {
	_, suitePath := validSuiteDir(t)
	s, err := Load(suitePath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if s.Concurrency != DefaultConcurrency {
		t.Errorf("Concurrency = %d, want default %d", s.Concurrency, DefaultConcurrency)
	}
	if s.TimeoutSeconds != DefaultTimeoutSeconds {
		t.Errorf("TimeoutSeconds = %d, want default %d", s.TimeoutSeconds, DefaultTimeoutSeconds)
	}
}

func TestValidateMissingRequiredField(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "suite.yaml", `
system: ./system.py
dataset: tasks.jsonl
graders:
  - name: exact_match
`)
	s, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
}

func TestValidateMissingSystemPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "tasks.jsonl", `{"id":"a","input":"hi"}`+"\n")
	p := writeFile(t, dir, "suite.yaml", `
name: my-suite
system: ./does-not-exist.py
dataset: tasks.jsonl
graders:
  - name: exact_match
`)
	s, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for missing system path, got nil")
	}
}

func TestValidateUnrecognizedGrader(t *testing.T) {
	dir, _ := validSuiteDir(t)
	p := writeFile(t, dir, "suite2.yaml", `
name: my-suite
system: ./system.py
dataset: tasks.jsonl
graders:
  - name: not_a_real_grader
`)
	s, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for unrecognized grader, got nil")
	}
}

func TestValidateLiteralCredentialRejected(t *testing.T) {
	dir, _ := validSuiteDir(t)
	p := writeFile(t, dir, "suite3.yaml", `
name: my-suite
system: ./system.py
dataset: tasks.jsonl
graders:
  - name: llm_judge
    params:
      rubric: "test"
      judge_endpoint: "http://localhost:8080"
      api_key: "sk-literal-secret"
`)
	s, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for literal credential, got nil")
	}
}

func TestValidateEnvIndirectionAllowed(t *testing.T) {
	dir, _ := validSuiteDir(t)
	p := writeFile(t, dir, "suite4.yaml", `
name: my-suite
system: ./system.py
dataset: tasks.jsonl
graders:
  - name: llm_judge
    params:
      rubric: "test"
      judge_endpoint: "http://localhost:8080"
      api_key_env: "JUDGE_API_KEY"
`)
	s, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}
