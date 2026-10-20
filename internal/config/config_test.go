package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyyril/perfguard/internal/config"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

func TestDefaultConfigValid(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config failed validation: %v", err)
	}

	if cfg.Version != 1 {
		t.Errorf("expected version 1, got %d", cfg.Version)
	}
	if cfg.Benchmarks.Count != 10 {
		t.Errorf("expected count 10, got %d", cfg.Benchmarks.Count)
	}
	if cfg.Thresholds.MinSamples != 6 {
		t.Errorf("expected min_samples 6, got %d", cfg.Thresholds.MinSamples)
	}
}

func TestLoadBytes_ValidCases(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		yaml string
	}{
		{
			name: "minimal valid",
			yaml: `
version: 1
benchmarks:
  count: 10
thresholds:
  min_samples: 6
`,
		},
		{
			name: "full config with custom metric and override",
			yaml: `
version: 1
benchmarks:
  packages: ["./pkg/..."]
  include: "^BenchmarkTest"
  exclude: ["BenchmarkSlow"]
  count: 15
  benchtime: "2s"
  cpu: 4
  timeout: "45m"
execution:
  mode: stored
  interleave: false
  warmup_runs: 2
  max_extra_runs: 5
  extra_round_step: 3
  pin_cpus: "0-3"
thresholds:
  alpha: 0.01
  min_samples: 8
  correction: holm
  zero_base: fail
  metrics:
    sec/op: { warn: 4, fail: 12 }
  custom:
    p95-ms: { warn: 5, fail: 15, direction: lower_is_better }
  overrides:
    - match: "^BenchmarkHot"
      metric: sec/op
      warn: 2
      fail: 6
noise:
  max_cv: 8
  on_noisy: warn
profiling:
  mode: always
  types: [cpu, allocs, mutex, block]
  runs: 5
  min_share: 2
  min_samples: 500
  top: 20
baseline:
  store: fs
  dir: .baselines
report:
  formats: [text, md, json, github]
github:
  comment: false
  step_summary: true
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := config.LoadBytes([]byte(tc.yaml))
			if err != nil {
				t.Fatalf("expected valid config, got error: %v", err)
			}
			if cfg == nil {
				t.Fatal("expected non-nil config")
			}
		})
	}
}

func TestLoadBytes_InvalidTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		yaml          string
		expectedField string
	}{
		{
			name: "wrong version",
			yaml: `
version: 2
`,
			expectedField: "version",
		},
		{
			name: "count less than min_samples",
			yaml: `
version: 1
benchmarks:
  count: 4
thresholds:
  min_samples: 6
`,
			expectedField: "benchmarks.count",
		},
		{
			name: "invalid regex in include",
			yaml: `
version: 1
benchmarks:
  include: "[invalid regex"
`,
			expectedField: "benchmarks.include",
		},
		{
			name: "invalid regex in exclude",
			yaml: `
version: 1
benchmarks:
  exclude: ["[bad"]
`,
			expectedField: "benchmarks.exclude[0]",
		},
		{
			name: "invalid timeout duration",
			yaml: `
version: 1
benchmarks:
  timeout: "not-a-duration"
`,
			expectedField: "benchmarks.timeout",
		},
		{
			name: "invalid execution mode",
			yaml: `
version: 1
execution:
  mode: invalid-mode
`,
			expectedField: "execution.mode",
		},
		{
			name: "extra_round_step zero",
			yaml: `
version: 1
execution:
  extra_round_step: 0
`,
			expectedField: "execution.extra_round_step",
		},
		{
			name: "alpha out of bounds high",
			yaml: `
version: 1
thresholds:
  alpha: 1.5
`,
			expectedField: "thresholds.alpha",
		},
		{
			name: "alpha out of bounds zero",
			yaml: `
version: 1
thresholds:
  alpha: 0.0
`,
			expectedField: "thresholds.alpha",
		},
		{
			name: "min_samples too low",
			yaml: `
version: 1
thresholds:
  min_samples: 1
`,
			expectedField: "thresholds.min_samples",
		},
		{
			name: "invalid correction",
			yaml: `
version: 1
thresholds:
  correction: invalid
`,
			expectedField: "thresholds.correction",
		},
		{
			name: "invalid zero_base",
			yaml: `
version: 1
thresholds:
  zero_base: unknown
`,
			expectedField: "thresholds.zero_base",
		},
		{
			name: "metrics fail less than warn",
			yaml: `
version: 1
thresholds:
  metrics:
    sec/op: { warn: 15, fail: 5 }
`,
			expectedField: "thresholds.metrics.sec/op.fail",
		},
		{
			name: "custom fail less than warn",
			yaml: `
version: 1
thresholds:
  custom:
    p95-ms: { warn: 20, fail: 10, direction: lower_is_better }
`,
			expectedField: "thresholds.custom.p95-ms.fail",
		},
		{
			name: "custom missing direction",
			yaml: `
version: 1
thresholds:
  custom:
    p95-ms: { warn: 5, fail: 15, direction: "" }
`,
			expectedField: "thresholds.custom.p95-ms.direction",
		},
		{
			name: "override bad regex",
			yaml: `
version: 1
thresholds:
  overrides:
    - match: "[bad"
      metric: sec/op
      warn: 5
      fail: 10
`,
			expectedField: "thresholds.overrides[0].match",
		},
		{
			name: "override empty match",
			yaml: `
version: 1
thresholds:
  overrides:
    - match: ""
      metric: sec/op
      warn: 5
      fail: 10
`,
			expectedField: "thresholds.overrides[0].match",
		},
		{
			name: "override empty metric",
			yaml: `
version: 1
thresholds:
  overrides:
    - match: "^Benchmark"
      metric: ""
      warn: 5
      fail: 10
`,
			expectedField: "thresholds.overrides[0].metric",
		},
		{
			name: "override fail less than warn",
			yaml: `
version: 1
thresholds:
  overrides:
    - match: "^Benchmark"
      metric: sec/op
      warn: 10
      fail: 5
`,
			expectedField: "thresholds.overrides[0].fail",
		},
		{
			name: "noise max_cv negative",
			yaml: `
version: 1
noise:
  max_cv: -5
`,
			expectedField: "noise.max_cv",
		},
		{
			name: "noise invalid on_noisy",
			yaml: `
version: 1
noise:
  on_noisy: fail
`,
			expectedField: "noise.on_noisy",
		},
		{
			name: "profiling invalid mode",
			yaml: `
version: 1
profiling:
  mode: invalid
`,
			expectedField: "profiling.mode",
		},
		{
			name: "profiling invalid type",
			yaml: `
version: 1
profiling:
  types: [cpu, invalid-type]
`,
			expectedField: "profiling.types[1]",
		},
		{
			name: "baseline invalid store",
			yaml: `
version: 1
baseline:
  store: sqlite
`,
			expectedField: "baseline.store",
		},
		{
			name: "report invalid format",
			yaml: `
version: 1
report:
  formats: [text, html]
`,
			expectedField: "report.formats[1]",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := config.LoadBytes([]byte(tc.yaml))
			if err == nil {
				t.Fatalf("expected validation error mentioning %q, got nil", tc.expectedField)
			}
			if !strings.Contains(err.Error(), tc.expectedField) {
				t.Errorf("expected error to mention field %q, got: %v", tc.expectedField, err)
			}
			if !config.IsValidationError(err) {
				t.Errorf("expected err to be ValidationError, got: %T", err)
			}
		})
	}
}

func TestConfig_JSONSchemaValidation(t *testing.T) {
	t.Parallel()

	schemaPath := filepath.Join("..", "..", "schemas", "config.v1.json")
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("failed reading schema: %v", err)
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("config.v1.json", strings.NewReader(string(schemaData))); err != nil {
		t.Fatalf("failed adding schema resource: %v", err)
	}

	sch, err := compiler.Compile("config.v1.json")
	if err != nil {
		t.Fatalf("failed compiling schema: %v", err)
	}

	cfg := config.Default()
	yamlBytes, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("failed marshaling config to YAML: %v", err)
	}

	var intermediate any
	if err := yaml.Unmarshal(yamlBytes, &intermediate); err != nil {
		t.Fatalf("failed unmarshaling YAML to intermediate: %v", err)
	}

	jsonBytes, err := json.Marshal(intermediate)
	if err != nil {
		t.Fatalf("failed marshaling to JSON: %v", err)
	}

	var jsonTarget any
	if err := json.Unmarshal(jsonBytes, &jsonTarget); err != nil {
		t.Fatalf("failed unmarshaling to JSON target: %v", err)
	}

	if err := sch.Validate(jsonTarget); err != nil {
		t.Errorf("default config failed schema validation: %v", err)
	}
}

func TestResolveConfigPath(t *testing.T) {
	// Not parallel because it mutates environment.
	orig := os.Getenv("PERFGUARD_CONFIG")
	defer func() {
		_ = os.Setenv("PERFGUARD_CONFIG", orig)
	}()

	_ = os.Unsetenv("PERFGUARD_CONFIG")
	if got := config.ResolveConfigPath(); got != "perfguard.yaml" {
		t.Errorf("expected default perfguard.yaml, got %s", got)
	}

	_ = os.Setenv("PERFGUARD_CONFIG", "custom.yaml")
	if got := config.ResolveConfigPath(); got != "custom.yaml" {
		t.Errorf("expected custom.yaml, got %s", got)
	}
}

func TestLoadFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	validPath := filepath.Join(tmpDir, "perfguard.yaml")
	if err := os.WriteFile(validPath, []byte("version: 1\n"), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := config.Load(validPath)
	if err != nil {
		t.Fatalf("expected valid load, got: %v", err)
	}
	if cfg.Version != 1 {
		t.Errorf("expected version 1, got %d", cfg.Version)
	}

	if _, err := config.Load(filepath.Join(tmpDir, "missing.yaml")); err == nil {
		t.Error("expected error for missing file, got nil")
	}
}
