package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// helperSrc is a tiny stdio-protocol-shaped program used to exercise Call
// against a real subprocess: it reads one line from stdin and, based on
// that line's content, echoes it back, exits non-zero, prints garbage, or
// hangs. This lets tests drive Call's actual stdin-write/stdout-read path
// without depending on any external interpreter being installed.
const helperSrc = `package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

func main() {
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	line = strings.TrimSpace(line)
	switch line {
	case "fail":
		os.Exit(1)
	case "malformed":
		fmt.Println("not json")
	case "hang":
		time.Sleep(5 * time.Second)
	default:
		fmt.Println(line)
	}
}
`

var helperBinPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "runner-helper")
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

func TestCallSuccessEchoesStdin(t *testing.T) {
	res := Call(context.Background(), helperBinPath, []byte(`{"ok":true}`), 5*time.Second)
	if res.FailureReason != "" {
		t.Fatalf("unexpected failure: %s (%s)", res.FailureReason, res.Detail)
	}
	if string(res.Line) != `{"ok":true}` {
		t.Errorf("Line = %s, want {\"ok\":true}", res.Line)
	}
}

func TestCallNonZeroExit(t *testing.T) {
	res := Call(context.Background(), helperBinPath, []byte("fail"), 5*time.Second)
	if res.FailureReason == "" {
		t.Fatal("expected a failure reason for non-zero exit")
	}
}

func TestCallMalformedOutput(t *testing.T) {
	res := Call(context.Background(), helperBinPath, []byte("malformed"), 5*time.Second)
	if res.FailureReason == "" {
		t.Fatal("expected a failure reason for malformed output")
	}
}

func TestCallTimeout(t *testing.T) {
	res := Call(context.Background(), helperBinPath, []byte("hang"), 200*time.Millisecond)
	if res.FailureReason != "timeout" {
		t.Fatalf("FailureReason = %q, want timeout", res.FailureReason)
	}
}

func TestPoolBoundsConcurrency(t *testing.T) {
	const limit = 3
	pool := NewPool(limit)
	var inFlight int32
	var maxSeen int32
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		pool.Go(func() {
			defer wg.Done()
			n := atomic.AddInt32(&inFlight, 1)
			for {
				m := atomic.LoadInt32(&maxSeen)
				if n <= m || atomic.CompareAndSwapInt32(&maxSeen, m, n) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			atomic.AddInt32(&inFlight, -1)
		})
	}
	wg.Wait()

	if maxSeen > limit {
		t.Errorf("max concurrent = %d, want <= %d", maxSeen, limit)
	}
}
