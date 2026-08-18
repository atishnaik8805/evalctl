// Package integration exercises the real evalctl binary against fixture
// consumer/grader executables in tests/fixtures, per plan.md's Testing
// section (the stdio protocol boundary must be proven with a real
// subprocess call, not mocked).
package integration

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var evalctlBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "evalctl-bin")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	bin := filepath.Join(dir, "evalctl.exe")
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		panic(err)
	}
	build := exec.Command("go", "build", "-o", bin, "./cmd/evalctl")
	build.Dir = repoRoot
	build.Env = os.Environ()
	if out, err := build.CombinedOutput(); err != nil {
		panic("build evalctl: " + err.Error() + ": " + string(out))
	}
	evalctlBin = bin

	os.Exit(m.Run())
}

type cliResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

func runEvalctl(t *testing.T, args ...string) cliResult {
	t.Helper()
	cmd := exec.Command(evalctlBin, args...)
	cmd.Env = os.Environ()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("run evalctl: %v", err)
		}
	}
	return cliResult{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: exitCode}
}

// fixturePath resolves a path under tests/fixtures relative to the
// integration test package (which runs with its own directory as cwd).
func fixturePath(name string) string {
	abs, err := filepath.Abs(filepath.Join("..", "fixtures", name))
	if err != nil {
		panic(err)
	}
	return abs
}
