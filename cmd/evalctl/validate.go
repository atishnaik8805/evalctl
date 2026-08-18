package main

import (
	"flag"
	"fmt"

	"evalctl/internal/dataset"
	"evalctl/internal/suite"
)

// validateCommand runs the checks in contracts/suite-config-schema.md
// without spawning system or any grader (FR-014).
func validateCommand(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	suitePath := fs.String("suite", "", "path to suite config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *suitePath == "" {
		return fmt.Errorf("validate: --suite is required")
	}

	s, err := suite.Load(*suitePath)
	if err != nil {
		return err
	}
	if err := s.Validate(); err != nil {
		return err
	}
	// Suite.Validate() checks the dataset file exists; also load it to
	// enforce data-model.md Task validation (duplicate ids, missing fields).
	if _, err := dataset.Load(s.ResolvePath(s.Dataset)); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	fmt.Println("OK: suite config and dataset are well-formed")
	return nil
}
