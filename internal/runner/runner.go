// Package runner invokes external subprocesses (evaluated systems and,
// via internal/grader/external, custom graders) over the stdio protocol
// (contracts/stdio-protocol.md), with bounded concurrency and timeouts.
package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Result is the outcome of one subprocess call: either a parsed response
// line, or a classified failure reason.
type Result struct {
	Line []byte
	// FailureReason is one of "consumer_error"/"grader_error" (the caller
	// picks which label applies), "timeout", or "" on success.
	FailureReason string
	Detail        string
}

// Call runs one subprocess: writes reqLine to stdin, closes stdin, reads a
// single line from stdout, and enforces timeout. It never returns a Go
// error for subprocess-side failures (non-zero exit, malformed output,
// timeout) — those are reported via Result.FailureReason so callers can
// record a failing Outcome per FR-003/FR-018 instead of aborting.
func Call(ctx context.Context, executable string, reqLine []byte, timeout time.Duration) Result {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, executable)
	cmd.Stdin = bytes.NewReader(append(reqLine, '\n'))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		return Result{FailureReason: "timeout", Detail: fmt.Sprintf("no output before %s elapsed", timeout)}
	}
	if err != nil {
		return Result{FailureReason: "process_error", Detail: fmt.Sprintf("exit error: %v; stderr: %s", err, strings.TrimSpace(stderr.String()))}
	}

	line, parseErr := firstJSONLine(stdout.Bytes())
	if parseErr != nil {
		return Result{FailureReason: "process_error", Detail: fmt.Sprintf("malformed stdout: %v", parseErr)}
	}
	return Result{Line: line}
}

// firstJSONLine extracts and validates the single JSON line expected on
// stdout (contracts/stdio-protocol.md: "exactly one line of JSON").
func firstJSONLine(out []byte) ([]byte, error) {
	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var probe json.RawMessage
		if err := json.Unmarshal(line, &probe); err != nil {
			return nil, fmt.Errorf("not valid JSON: %w", err)
		}
		return append([]byte(nil), line...), nil
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("no output produced")
}

// Pool bounds how many Run funcs execute concurrently (FR-017).
type Pool struct {
	sem chan struct{}
}

// NewPool creates a worker pool bounded to n concurrent slots.
func NewPool(n int) *Pool {
	if n <= 0 {
		n = 1
	}
	return &Pool{sem: make(chan struct{}, n)}
}

// Go runs fn, blocking until a pool slot is free. Callers coordinate
// completion themselves (e.g. via sync.WaitGroup).
func (p *Pool) Go(fn func()) {
	p.sem <- struct{}{}
	go func() {
		defer func() { <-p.sem }()
		fn()
	}()
}
