package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"

	"evalctl/internal/analyze"
)

// analyzeCommand starts the evalctl analyze web UI (research.md §9).
// It fails fast — no retry on another port — if --addr is already in use.
func analyzeCommand(args []string) error {
	fs := flag.NewFlagSet("analyze", flag.ExitOnError)
	tracesPath := fs.String("traces", "", "path to a SuiteRunReport or standalone raw-trace JSONL file")
	annotationsPath := fs.String("annotations", "", "path to the annotations sidecar file (default: <traces>.annotations.json)")
	addr := fs.String("addr", "localhost:8420", "address to listen on")
	llmEndpoint := fs.String("llm-endpoint", "", "optional HTTP endpoint for LLM-suggested axial-code groupings")
	llmAPIKeyEnv := fs.String("llm-api-key-env", "", "environment variable holding the bearer token for --llm-endpoint")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *tracesPath == "" {
		return fmt.Errorf("analyze: --traces is required")
	}
	if *annotationsPath == "" {
		*annotationsPath = *tracesPath + ".annotations.json"
	}

	traces, err := analyze.LoadTraces(*tracesPath)
	if err != nil {
		return fmt.Errorf("analyze: %w", err)
	}
	store, err := analyze.Load(*annotationsPath)
	if err != nil {
		return fmt.Errorf("analyze: %w", err)
	}

	srv := analyze.NewServer(traces, store, *llmEndpoint, *llmAPIKeyEnv)

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return fmt.Errorf("analyze: listen on %s: %w", *addr, err)
	}
	fmt.Printf("evalctl analyze: serving %d trace(s) at http://%s (annotations: %s)\n", len(traces), *addr, *annotationsPath)
	return http.Serve(ln, srv)
}
