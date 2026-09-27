package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"evalctl/internal/dataset"
	"evalctl/internal/grader/builtin"
	"evalctl/internal/grader/external"
	"evalctl/internal/grader/judge"
	"evalctl/internal/protocol"
	"evalctl/internal/report"
	"evalctl/internal/runner"
	"evalctl/internal/suite"
)

func runCommand(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	suitePath := fs.String("suite", "", "path to suite config file")
	outputPath := fs.String("output", "", "path to write full SuiteRunReport JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *suitePath == "" {
		return fmt.Errorf("run: --suite is required")
	}

	s, err := suite.Load(*suitePath)
	if err != nil {
		return err
	}
	tasks, err := dataset.Load(s.ResolvePath(s.Dataset))
	if err != nil {
		return err
	}

	systemPath := s.ResolvePath(s.System)
	timeout := time.Duration(s.TimeoutSeconds) * time.Second

	pool := runner.NewPool(s.Concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var allOutcomes []protocol.Outcome
	var allTraces []report.Trace

	for _, tsk := range tasks {
		tsk := tsk
		wg.Add(1)
		pool.Go(func() {
			defer wg.Done()
			outcomes, transcript, ok := runTask(context.Background(), s, systemPath, tsk, timeout)
			mu.Lock()
			allOutcomes = append(allOutcomes, outcomes...)
			if ok {
				allTraces = append(allTraces, report.Trace{
					TaskID: tsk.ID,
					Input:  tsk.Input,
					Output: transcript.Output,
					Steps:  transcript.Steps,
				})
			}
			mu.Unlock()
		})
	}
	wg.Wait()

	rep := report.BuildSuiteRunReport(s.Name, allOutcomes, allTraces, s.FailBelow)
	rep.PrintSummary(os.Stdout)

	if *outputPath != "" {
		f, err := os.Create(*outputPath)
		if err != nil {
			return fmt.Errorf("run: write output: %w", err)
		}
		defer f.Close()
		if err := report.WriteJSON(f, rep); err != nil {
			return fmt.Errorf("run: write output: %w", err)
		}
	}

	if !rep.Passed {
		os.Exit(1)
	}
	return nil
}

// runTask evaluates one task against the suite's system, then every
// configured grader. Consumer subprocess failures (FR-003) produce a
// failing Outcome per grader rather than aborting the run. The returned
// bool reports whether a transcript was actually produced (research.md
// §2) — false means the returned Transcript is a zero value and must not
// be turned into a report.Trace.
func runTask(ctx context.Context, s *suite.Suite, systemPath string, tsk protocol.Task, timeout time.Duration) ([]protocol.Outcome, protocol.Transcript, bool) {
	reqBody := protocol.ConsumerRequest{ProtocolVersion: protocol.ProtocolVersion, Task: tsk}
	reqLine, _ := json.Marshal(reqBody)

	res := runner.Call(ctx, systemPath, reqLine, timeout)
	if res.FailureReason != "" {
		reason := protocol.ErrConsumerError
		if res.FailureReason == "timeout" {
			reason = protocol.ErrTimeout
		}
		return failAllGraders(s.Graders, tsk.ID, reason), protocol.Transcript{}, false
	}

	var transcript protocol.Transcript
	if err := json.Unmarshal(res.Line, &transcript); err != nil {
		return failAllGraders(s.Graders, tsk.ID, protocol.ErrConsumerError), protocol.Transcript{}, false
	}

	outcomes := make([]protocol.Outcome, 0, len(s.Graders))
	for _, g := range s.Graders {
		outcomes = append(outcomes, runGrader(ctx, s, tsk, transcript, g, timeout))
	}
	return outcomes, transcript, true
}

func failAllGraders(graders []suite.GraderConfig, taskID, reason string) []protocol.Outcome {
	outcomes := make([]protocol.Outcome, 0, len(graders))
	for _, g := range graders {
		outcomes = append(outcomes, protocol.FailingOutcome(taskID, g.Name, reason))
	}
	return outcomes
}

func runGrader(ctx context.Context, s *suite.Suite, tsk protocol.Task, transcript protocol.Transcript, g suite.GraderConfig, timeout time.Duration) protocol.Outcome {
	switch g.Name {
	case "exact_match":
		return builtin.ExactMatch(tsk, transcript)
	case "schema_validity":
		return builtin.SchemaValidity(tsk, transcript, g.Params)
	case "entity_grounding":
		return builtin.EntityGrounding(tsk, transcript)
	case "llm_judge":
		params, err := judge.ParseParams(g.Params)
		if err != nil {
			o := protocol.FailingOutcome(tsk.ID, g.Name, protocol.ErrJudgeEndpointErr)
			msg := err.Error()
			o.Reasoning = &msg
			return o
		}
		return judge.Grade(ctx, tsk, transcript, params, timeout)
	default:
		execPath := s.ResolvePath(g.Name)
		return external.Grade(ctx, g.Name, execPath, tsk, transcript, timeout)
	}
}
