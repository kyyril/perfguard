// Package eval loads evaluation scenarios, runs trials, and records raw results.
// It defines ports for workspace preparation and check execution; implementations
// live in the app package so that eval does not depend on adapters.
package eval

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/kyyril/perfguard/internal/model"
)

// ErrScenario marks errors caused by an invalid or missing scenario definition.
// The CLI maps it to the configuration exit code.
var ErrScenario = errors.New("invalid eval scenario")

// Scenario represents an evaluation test case with base code, a change patch,
// and ground truth expectations.
type Scenario struct {
	ID          string          `yaml:"id"`
	Description string          `yaml:"description"`
	Trials      int             `yaml:"trials"`
	Expect      ExpectedOutcome `yaml:"expect"`

	// Directory is the absolute path to the scenario directory. It is not
	// part of scenario.yaml.
	Directory string `yaml:"-"`
}

// ExpectedOutcome defines the ground truth for a scenario.
type ExpectedOutcome struct {
	Verdict   model.Verdict    `yaml:"verdict"`
	Metrics   []MetricExpect   `yaml:"metrics,omitempty"`
	Diagnosis *DiagnosisExpect `yaml:"diagnosis,omitempty"`
}

// MetricExpect defines the expected status for a specific (benchmark, metric) pair.
type MetricExpect struct {
	Benchmark string       `yaml:"benchmark"`
	Metric    string       `yaml:"metric"`
	Status    model.Status `yaml:"status"`
}

// DiagnosisExpect defines the expected diagnosis results.
type DiagnosisExpect struct {
	Profile  string `yaml:"profile"`
	TopK     int    `yaml:"top_k"`
	Function string `yaml:"function"`
}

// LoadScenario reads and validates scenario.yaml from the given directory.
func LoadScenario(dir string) (Scenario, error) {
	scenarioFile := filepath.Join(dir, "scenario.yaml")
	f, err := os.Open(scenarioFile)
	if err != nil {
		return Scenario{}, fmt.Errorf("%w: open scenario file: %w", ErrScenario, err)
	}
	defer func() {
		// Read-only file; a close error carries no actionable information.
		_ = f.Close()
	}()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)

	var s Scenario
	if err := dec.Decode(&s); err != nil {
		return Scenario{}, fmt.Errorf("%w: parse scenario yaml: %w", ErrScenario, err)
	}

	s.Directory = dir

	if err := validateScenario(&s); err != nil {
		return Scenario{}, fmt.Errorf("%w: scenario %s: %v", ErrScenario, s.ID, err)
	}

	return s, nil
}

func validateScenario(s *Scenario) error {
	if s.ID == "" {
		return errors.New("scenario.id is required")
	}
	if s.Trials <= 0 {
		s.Trials = 1
	}

	baseDir := filepath.Join(s.Directory, "base")
	if stat, err := os.Stat(baseDir); err != nil || !stat.IsDir() {
		return fmt.Errorf("scenario %s: base/ directory not found", s.ID)
	}

	patchFile := filepath.Join(s.Directory, "change.patch")
	if _, err := os.Stat(patchFile); err != nil {
		return fmt.Errorf("scenario %s: change.patch not found", s.ID)
	}

	switch s.Expect.Verdict {
	case model.VerdictPass, model.VerdictWarn, model.VerdictFail, model.VerdictInconclusive:
	default:
		return fmt.Errorf("scenario %s: expect.verdict %q must be one of pass, warn, fail, inconclusive",
			s.ID, s.Expect.Verdict)
	}

	for i, m := range s.Expect.Metrics {
		if m.Benchmark == "" {
			return fmt.Errorf("scenario %s: expect.metrics[%d].benchmark is required", s.ID, i)
		}
		if m.Metric == "" {
			return fmt.Errorf("scenario %s: expect.metrics[%d].metric is required", s.ID, i)
		}
		switch m.Status {
		case model.StatusUnchanged, model.StatusImproved, model.StatusWarn, model.StatusFail,
			model.StatusInconclusive, model.StatusNew, model.StatusRemoved:
		default:
			return fmt.Errorf("scenario %s: expect.metrics[%d].status %q is invalid", s.ID, i, m.Status)
		}
	}

	if d := s.Expect.Diagnosis; d != nil {
		switch d.Profile {
		case "cpu", "allocs", "mutex", "block":
		default:
			return fmt.Errorf("scenario %s: expect.diagnosis.profile %q must be cpu, allocs, mutex, or block",
				s.ID, d.Profile)
		}
		if d.TopK < 1 {
			return fmt.Errorf("scenario %s: expect.diagnosis.top_k must be at least 1", s.ID)
		}
		if d.Function == "" {
			return fmt.Errorf("scenario %s: expect.diagnosis.function is required", s.ID)
		}
	}

	return nil
}

// ListScenarios discovers all scenarios under <evalRoot>/scenarios.
func ListScenarios(evalRoot string) ([]Scenario, error) {
	scenariosDir := filepath.Join(evalRoot, "scenarios")
	entries, err := os.ReadDir(scenariosDir)
	if err != nil {
		return nil, fmt.Errorf("%w: read scenarios directory: %w", ErrScenario, err)
	}

	var scenarios []Scenario
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		scenarioDir := filepath.Join(scenariosDir, entry.Name())
		s, err := LoadScenario(scenarioDir)
		if err != nil {
			return nil, fmt.Errorf("load scenario %s: %w", entry.Name(), err)
		}
		scenarios = append(scenarios, s)
	}

	return scenarios, nil
}
