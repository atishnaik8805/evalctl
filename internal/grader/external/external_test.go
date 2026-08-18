package external

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"evalctl/internal/protocol"
)

// helperSrc is a tiny stdio-protocol grader used to exercise Grade against
// a real subprocess: it reads one line from stdin (a GraderRequest) and,
// based on the task id, echoes a success Outcome, exits non-zero, prints
// garbage, or hangs.
const helperSrc = `package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

func main() {
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	var req map[string]any
	json.Unmarshal([]byte(line), &req)
	task := req["task"].(map[string]any)
	switch task["id"] {
	case "fail":
		os.Exit(1)
	case "malformed":
		fmt.Println("not json")
	case "hang":
		time.Sleep(5 * time.Second)
	default:
		out := map[string]any{"task_id": "wrong-id", "grader_name": "wrong-name", "score": 1.0, "passed": true}
		b, _ := json.Marshal(out)
		fmt.Println(string(b))
	}
}
`

var helperBinPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "external-helper")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	srcPath := filepath.Join(dir, "helper.go")
	if err := os.WriteFile(srcPath, []byte(helperSrc), 0o644); err != nil {
		panic(err)
	}
	binPath := filepath.Join(dir, "helper")
	if os.PathListSeparator == ';' {
		binPath += ".exe"
	}
	build := exec.Command("go", "build", "-o", binPath, srcPath)
	build.Env = os.Environ()
	if out, err := build.CombinedOutput(); err != nil {
		panic("build helper: " + err.Error() + ": " + string(out))
	}
	helperBinPath = binPath

	os.Exit(m.Run())
}

func testTask(id string) protocol.Task {
	return protocol.Task{ID: id, Input: json.RawMessage(`"x"`), Metadata: json.RawMessage(`{}`)}
}

func TestGradeSuccessForcesTaskAndGraderIdentity(t *testing.T) {
	out := Grade(context.Background(), "my-grader", helperBinPath, testTask("ok"), protocol.Transcript{TaskID: "ok"}, 5*time.Second)
	if out.Error != nil {
		t.Fatalf("unexpected error: %v", *out.Error)
	}
	if out.TaskID != "ok" {
		t.Errorf("TaskID = %q, want %q (forced, not the grader's own claim)", out.TaskID, "ok")
	}
	if out.GraderName != "my-grader" {
		t.Errorf("GraderName = %q, want %q (forced, not the grader's own claim)", out.GraderName, "my-grader")
	}
	if out.Score != 1.0 {
		t.Errorf("Score = %v, want 1.0", out.Score)
	}
}

func TestGradeNonZeroExit(t *testing.T) {
	out := Grade(context.Background(), "my-grader", helperBinPath, testTask("fail"), protocol.Transcript{TaskID: "fail"}, 5*time.Second)
	if out.Error == nil || *out.Error != protocol.ErrGraderError {
		t.Fatalf("Error = %v, want %q", out.Error, protocol.ErrGraderError)
	}
}

func TestGradeMalformedOutput(t *testing.T) {
	out := Grade(context.Background(), "my-grader", helperBinPath, testTask("malformed"), protocol.Transcript{TaskID: "malformed"}, 5*time.Second)
	if out.Error == nil || *out.Error != protocol.ErrGraderError {
		t.Fatalf("Error = %v, want %q", out.Error, protocol.ErrGraderError)
	}
}

func TestGradeTimeout(t *testing.T) {
	out := Grade(context.Background(), "my-grader", helperBinPath, testTask("hang"), protocol.Transcript{TaskID: "hang"}, 200*time.Millisecond)
	if out.Error == nil || *out.Error != protocol.ErrTimeout {
		t.Fatalf("Error = %v, want %q", out.Error, protocol.ErrTimeout)
	}
}
