package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Default constants.
const (
	DefaultConfigPath = "perfguard.yaml"
	DefaultOutputDir  = ".perfguard"
	DefaultLogLevel   = "info"
)

// ValidationError records an issue at a specific field path.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors collects multiple validation issues.
type ValidationErrors []ValidationError

func (ve ValidationErrors) Error() string {
	if len(ve) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, err := range ve {
		if i > 0 {
			sb.WriteString("; ")
		}
		sb.WriteString(err.Error())
	}
	return sb.String()
}

// Config represents complete PerfGuard settings.
type Config struct {
	Version    int              `yaml:"version"`
	Benchmarks BenchmarksConfig `yaml:"benchmarks"`
	Execution  ExecutionConfig  `yaml:"execution"`
	Thresholds ThresholdsConfig `yaml:"thresholds"`
	Noise      NoiseConfig      `yaml:"noise"`
	Profiling  ProfilingConfig  `yaml:"profiling"`
	Baseline   BaselineConfig   `yaml:"baseline"`
	Report     ReportConfig     `yaml:"report"`
	GitHub     GitHubConfig     `yaml:"github"`
}

type BenchmarksConfig struct {
	Packages  []string `yaml:"packages"`
	Include   string   `yaml:"include"`
	Exclude   []string `yaml:"exclude"`
	Count     int      `yaml:"count"`
	Benchtime string   `yaml:"benchtime"`
	CPU       int      `yaml:"cpu"`
	Timeout   string   `yaml:"timeout"`
}

type ExecutionConfig struct {
	Mode           string `yaml:"mode"`
	Interleave     bool   `yaml:"interleave"`
	WarmupRuns     int    `yaml:"warmup_runs"`
	MaxExtraRuns   int    `yaml:"max_extra_runs"`
	ExtraRoundStep int    `yaml:"extra_round_step"`
	PinCPUs        string `yaml:"pin_cpus"`
}

type ThresholdValue struct {
	Warn float64 `yaml:"warn"`
	Fail float64 `yaml:"fail"`
}

type CustomMetricThreshold struct {
	Warn      float64 `yaml:"warn"`
	Fail      float64 `yaml:"fail"`
	Direction string  `yaml:"direction"`
}

type Override struct {
	Match  string  `yaml:"match"`
	Metric string  `yaml:"metric"`
	Warn   float64 `yaml:"warn"`
	Fail   float64 `yaml:"fail"`
}

type ThresholdsConfig struct {
	Alpha      float64                          `yaml:"alpha"`
	MinSamples int                              `yaml:"min_samples"`
	Correction string                           `yaml:"correction"`
	ZeroBase   string                           `yaml:"zero_base"`
	Metrics    map[string]ThresholdValue        `yaml:"metrics"`
	Custom     map[string]CustomMetricThreshold `yaml:"custom"`
	Overrides  []Override                       `yaml:"overrides"`
}

type NoiseConfig struct {
	MaxCV   float64 `yaml:"max_cv"`
	OnNoisy string  `yaml:"on_noisy"`
}

type ProfilingConfig struct {
	Mode       string   `yaml:"mode"`
	Types      []string `yaml:"types"`
	Runs       int      `yaml:"runs"`
	MinShare   float64  `yaml:"min_share"`
	MinSamples int      `yaml:"min_samples"`
	Top        int      `yaml:"top"`
}

type BaselineConfig struct {
	Store  string `yaml:"store"`
	Branch string `yaml:"branch"`
	Dir    string `yaml:"dir"`
}

type ReportConfig struct {
	Formats []string `yaml:"formats"`
}

type GitHubConfig struct {
	Comment     bool `yaml:"comment"`
	StepSummary bool `yaml:"step_summary"`
}

// Default returns a Config populated with default values per specification.
func Default() *Config {
	return &Config{
		Version: 1,
		Benchmarks: BenchmarksConfig{
			Packages:  []string{"./..."},
			Include:   ".",
			Exclude:   []string{},
			Count:     10,
			Benchtime: "1s",
			CPU:       0,
			Timeout:   "30m",
		},
		Execution: ExecutionConfig{
			Mode:           "same-runner",
			Interleave:     true,
			WarmupRuns:     1,
			MaxExtraRuns:   10,
			ExtraRoundStep: 2,
			PinCPUs:        "",
		},
		Thresholds: ThresholdsConfig{
			Alpha:      0.05,
			MinSamples: 6,
			Correction: "none",
			ZeroBase:   "warn",
			Metrics: map[string]ThresholdValue{
				"sec/op":    {Warn: 5, Fail: 15},
				"B/op":      {Warn: 10, Fail: 25},
				"allocs/op": {Warn: 5, Fail: 20},
			},
			Custom:    map[string]CustomMetricThreshold{},
			Overrides: []Override{},
		},
		Noise: NoiseConfig{
			MaxCV:   10,
			OnNoisy: "inconclusive",
		},
		Profiling: ProfilingConfig{
			Mode:       "on-regression",
			Types:      []string{"cpu", "allocs"},
			Runs:       3,
			MinShare:   1,
			MinSamples: 200,
			Top:        10,
		},
		Baseline: BaselineConfig{
			Store:  "git",
			Branch: "perfguard-data",
			Dir:    DefaultOutputDir + "/baselines",
		},
		Report: ReportConfig{
			Formats: []string{"text", "json"},
		},
		GitHub: GitHubConfig{
			Comment:     true,
			StepSummary: true,
		},
	}
}

// Load reads and parses configuration from a file path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}
	return LoadBytes(data)
}

// LoadBytes parses YAML configuration bytes, applies defaults, and validates fields.
func LoadBytes(data []byte) (*Config, error) {
	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing YAML config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate checks all fields in Config and returns aggregated ValidationErrors.
func (c *Config) Validate() error {
	var errs ValidationErrors

	if c.Version != 1 {
		errs = append(errs, ValidationError{
			Field:   "version",
			Message: fmt.Sprintf("unsupported version %d, must be 1", c.Version),
		})
	}

	// Benchmarks validation
	if c.Benchmarks.Count < c.Thresholds.MinSamples {
		errs = append(errs, ValidationError{
			Field: "benchmarks.count",
			Message: fmt.Sprintf("count (%d) cannot be less than thresholds.min_samples (%d)",
				c.Benchmarks.Count, c.Thresholds.MinSamples),
		})
	}
	if _, err := regexp.Compile(c.Benchmarks.Include); err != nil {
		errs = append(errs, ValidationError{
			Field:   "benchmarks.include",
			Message: fmt.Sprintf("invalid regex pattern: %v", err),
		})
	}
	for i, ex := range c.Benchmarks.Exclude {
		if _, err := regexp.Compile(ex); err != nil {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("benchmarks.exclude[%d]", i),
				Message: fmt.Sprintf("invalid regex pattern: %v", err),
			})
		}
	}
	if c.Benchmarks.Timeout != "" {
		if _, err := time.ParseDuration(c.Benchmarks.Timeout); err != nil {
			errs = append(errs, ValidationError{
				Field:   "benchmarks.timeout",
				Message: fmt.Sprintf("invalid duration: %v", err),
			})
		}
	}

	// Execution validation
	switch c.Execution.Mode {
	case "same-runner", "stored":
	default:
		errs = append(errs, ValidationError{
			Field:   "execution.mode",
			Message: fmt.Sprintf("invalid mode %q, must be same-runner or stored", c.Execution.Mode),
		})
	}
	if c.Execution.ExtraRoundStep < 1 {
		errs = append(errs, ValidationError{
			Field:   "execution.extra_round_step",
			Message: "must be at least 1",
		})
	}

	// Thresholds validation
	if c.Thresholds.Alpha <= 0 || c.Thresholds.Alpha >= 1 {
		errs = append(errs, ValidationError{
			Field:   "thresholds.alpha",
			Message: "alpha must be between 0 and 1",
		})
	}
	if c.Thresholds.MinSamples < 2 {
		errs = append(errs, ValidationError{
			Field:   "thresholds.min_samples",
			Message: "min_samples must be at least 2",
		})
	}
	switch c.Thresholds.Correction {
	case "none", "holm":
	default:
		errs = append(errs, ValidationError{
			Field:   "thresholds.correction",
			Message: fmt.Sprintf("invalid correction %q, must be none or holm", c.Thresholds.Correction),
		})
	}
	switch c.Thresholds.ZeroBase {
	case "warn", "fail":
	default:
		errs = append(errs, ValidationError{
			Field:   "thresholds.zero_base",
			Message: fmt.Sprintf("invalid zero_base %q, must be warn or fail", c.Thresholds.ZeroBase),
		})
	}

	for metric, tv := range c.Thresholds.Metrics {
		if tv.Fail < tv.Warn {
			errs = append(errs, ValidationError{
				Field: fmt.Sprintf("thresholds.metrics.%s.fail", metric),
				Message: fmt.Sprintf("fail threshold (%.2f) cannot be less than warn threshold (%.2f)",
					tv.Fail, tv.Warn),
			})
		}
	}

	for metric, ct := range c.Thresholds.Custom {
		if ct.Fail < ct.Warn {
			errs = append(errs, ValidationError{
				Field: fmt.Sprintf("thresholds.custom.%s.fail", metric),
				Message: fmt.Sprintf("fail threshold (%.2f) cannot be less than warn threshold (%.2f)",
					ct.Fail, ct.Warn),
			})
		}
		switch ct.Direction {
		case "lower_is_better", "higher_is_better":
		default:
			errs = append(errs, ValidationError{
				Field: fmt.Sprintf("thresholds.custom.%s.direction", metric),
				Message: fmt.Sprintf("missing or invalid direction %q, must be lower_is_better or higher_is_better",
					ct.Direction),
			})
		}
	}

	for i, ov := range c.Thresholds.Overrides {
		if ov.Match == "" {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("thresholds.overrides[%d].match", i),
				Message: "match pattern cannot be empty",
			})
		} else if _, err := regexp.Compile(ov.Match); err != nil {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("thresholds.overrides[%d].match", i),
				Message: fmt.Sprintf("invalid regex pattern: %v", err),
			})
		}
		if ov.Metric == "" {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("thresholds.overrides[%d].metric", i),
				Message: "metric name cannot be empty",
			})
		}
		if ov.Fail < ov.Warn {
			errs = append(errs, ValidationError{
				Field: fmt.Sprintf("thresholds.overrides[%d].fail", i),
				Message: fmt.Sprintf("fail threshold (%.2f) cannot be less than warn threshold (%.2f)",
					ov.Fail, ov.Warn),
			})
		}
	}

	// Noise validation
	if c.Noise.MaxCV < 0 {
		errs = append(errs, ValidationError{
			Field:   "noise.max_cv",
			Message: "max_cv cannot be negative",
		})
	}
	switch c.Noise.OnNoisy {
	case "inconclusive", "warn":
	default:
		errs = append(errs, ValidationError{
			Field:   "noise.on_noisy",
			Message: fmt.Sprintf("invalid on_noisy %q, must be inconclusive or warn", c.Noise.OnNoisy),
		})
	}

	// Profiling validation
	switch c.Profiling.Mode {
	case "off", "on-regression", "always":
	default:
		errs = append(errs, ValidationError{
			Field:   "profiling.mode",
			Message: fmt.Sprintf("invalid mode %q, must be off, on-regression, or always", c.Profiling.Mode),
		})
	}
	for i, pt := range c.Profiling.Types {
		switch pt {
		case "cpu", "allocs", "mutex", "block":
		default:
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("profiling.types[%d]", i),
				Message: fmt.Sprintf("unknown profile type %q, must be cpu, allocs, mutex, or block", pt),
			})
		}
	}

	// Baseline validation
	switch c.Baseline.Store {
	case "fs", "git":
	default:
		errs = append(errs, ValidationError{
			Field:   "baseline.store",
			Message: fmt.Sprintf("invalid store %q, must be fs or git", c.Baseline.Store),
		})
	}

	// Report validation
	for i, f := range c.Report.Formats {
		switch f {
		case "text", "json", "md", "github":
		default:
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("report.formats[%d]", i),
				Message: fmt.Sprintf("invalid format %q, must be text, json, md, or github", f),
			})
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

// ResolveConfigPath returns config path from env PERFGUARD_CONFIG or default.
func ResolveConfigPath() string {
	if p := os.Getenv("PERFGUARD_CONFIG"); p != "" {
		return p
	}
	return DefaultConfigPath
}

// IsValidationError checks whether an error is a ValidationError or ValidationErrors.
func IsValidationError(err error) bool {
	var ve ValidationError
	var ves ValidationErrors
	return errors.As(err, &ve) || errors.As(err, &ves)
}
