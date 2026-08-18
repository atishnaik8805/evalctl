package main

import "fmt"

// listGradersCommand prints the built-in graders and their accepted params
// keys (FR-015, contracts/cli-commands.md).
func listGradersCommand(args []string) error {
	graders := []struct {
		Name   string
		Params string
	}{
		{"exact_match", "(none)"},
		{"schema_validity", "required_fields"},
		{"entity_grounding", "(none; reads task.metadata.known_entities)"},
		{"llm_judge", "rubric, judge_endpoint, api_key_env"},
	}
	for _, g := range graders {
		fmt.Printf("%s\n  params: %s\n", g.Name, g.Params)
	}
	return nil
}
