// Package suite parses and validates suite configuration files
// (contracts/suite-config-schema.md).
package suite

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultConcurrency    = 5
	DefaultTimeoutSeconds = 30
)

// BuiltinGraders lists the recognized built-in grader names (FR-015).
var BuiltinGraders = map[string]bool{
	"exact_match":      true,
	"schema_validity":  true,
	"entity_grounding": true,
	"llm_judge":        true,
}

// literalSecretKeys are params keys that must never carry a literal value;
// they must instead be referenced indirectly via an *_env sibling key (FR-019).
var literalSecretKeys = []string{"api_key", "token", "secret", "password"}

// GraderConfig is one grader entry inside a Suite (data-model.md GraderConfig).
type GraderConfig struct {
	Name   string         `yaml:"name" json:"name"`
	Params map[string]any `yaml:"params" json:"params"`
}

// Suite is the declarative definition of one evaluation run (data-model.md Suite).
type Suite struct {
	Name           string         `yaml:"name" json:"name"`
	System         string         `yaml:"system" json:"system"`
	Dataset        string         `yaml:"dataset" json:"dataset"`
	Graders        []GraderConfig `yaml:"graders" json:"graders"`
	FailBelow      *float64       `yaml:"fail_below" json:"fail_below"`
	Concurrency    int            `yaml:"concurrency" json:"concurrency"`
	TimeoutSeconds int            `yaml:"timeout_seconds" json:"timeout_seconds"`

	// baseDir is the directory the suite file lives in, used to resolve
	// relative system/dataset/custom-grader paths.
	baseDir string
}

// Load reads a suite config file (YAML or JSON, by extension) and applies defaults.
func Load(path string) (*Suite, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read suite file %s: %w", path, err)
	}

	var s Suite
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(b, &s); err != nil {
			return nil, fmt.Errorf("parse suite YAML %s: %w", path, err)
		}
	case ".json":
		if err := json.Unmarshal(b, &s); err != nil {
			return nil, fmt.Errorf("parse suite JSON %s: %w", path, err)
		}
	default:
		return nil, fmt.Errorf("unsupported suite file extension %q (want .yaml, .yml, or .json)", ext)
	}

	s.baseDir = filepath.Dir(path)
	if s.Concurrency == 0 {
		s.Concurrency = DefaultConcurrency
	}
	if s.TimeoutSeconds == 0 {
		s.TimeoutSeconds = DefaultTimeoutSeconds
	}
	return &s, nil
}

// ResolvePath resolves a path field (system, dataset, custom grader) relative
// to the suite file's own directory when it is not already absolute.
func (s *Suite) ResolvePath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(s.baseDir, p)
}

// Validate runs the schema checks from contracts/suite-config-schema.md,
// without spawning any process (FR-014).
func (s *Suite) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("suite: missing required field \"name\"")
	}
	if s.System == "" {
		return fmt.Errorf("suite: missing required field \"system\"")
	}
	if s.Dataset == "" {
		return fmt.Errorf("suite: missing required field \"dataset\"")
	}
	if len(s.Graders) == 0 {
		return fmt.Errorf("suite: at least one grader is required")
	}
	if s.FailBelow != nil && (*s.FailBelow < 0.0 || *s.FailBelow > 1.0) {
		return fmt.Errorf("suite: fail_below must be within [0.0, 1.0]")
	}

	systemPath := s.ResolvePath(s.System)
	if _, err := os.Stat(systemPath); err != nil {
		return fmt.Errorf("suite: system executable not found: %s", systemPath)
	}

	datasetPath := s.ResolvePath(s.Dataset)
	if _, err := os.Stat(datasetPath); err != nil {
		return fmt.Errorf("suite: dataset file not found: %s", datasetPath)
	}

	for i, g := range s.Graders {
		if err := s.validateGrader(g); err != nil {
			return fmt.Errorf("suite: graders[%d]: %w", i, err)
		}
	}
	return nil
}

func (s *Suite) validateGrader(g GraderConfig) error {
	if g.Name == "" {
		return fmt.Errorf("missing required field \"name\"")
	}
	if !BuiltinGraders[g.Name] {
		graderPath := s.ResolvePath(g.Name)
		if _, err := os.Stat(graderPath); err != nil {
			return fmt.Errorf("grader %q is neither a recognized built-in nor an existing executable path", g.Name)
		}
	}
	if err := checkNoLiteralSecrets(g.Params); err != nil {
		return fmt.Errorf("grader %q: %w", g.Name, err)
	}
	return nil
}

// checkNoLiteralSecrets rejects a literal string value under any
// secret-shaped key (FR-019); such values must instead be supplied via an
// *_env-suffixed key naming an environment variable.
func checkNoLiteralSecrets(params map[string]any) error {
	for k, v := range params {
		lowerKey := strings.ToLower(k)
		for _, secretKey := range literalSecretKeys {
			if lowerKey == secretKey {
				if s, ok := v.(string); ok && s != "" {
					return fmt.Errorf("params key %q contains a literal credential value; use %q with an environment variable name instead", k, secretKey+"_env")
				}
			}
		}
	}
	return nil
}
