// Package profile collects, loads, merges, normalizes, and compares
// pprof profiles for regressed benchmarks.
package profile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Supported profile types.
const (
	TypeCPU    = "cpu"
	TypeAllocs = "allocs"
	TypeMutex  = "mutex"
	TypeBlock  = "block"
)

var (
	// ErrUnsupportedType indicates an unknown profile type.
	ErrUnsupportedType = errors.New("profile: unsupported profile type")

	// ErrInvalidIterations indicates iterations is <= 0.
	ErrInvalidIterations = errors.New("profile: iterations must be positive")

	// ErrMissingBenchmark indicates no benchmark name was provided.
	ErrMissingBenchmark = errors.New("profile: benchmark name is required")

	// ErrMissingBinary indicates no test binary path was provided.
	ErrMissingBinary = errors.New("profile: binary path is required")

	// ErrProfileEmpty indicates the generated profile file is empty or missing.
	ErrProfileEmpty = errors.New("profile: generated profile is empty or missing")
)

// SupportedTypes lists all profile types recognized by PerfGuard.
var SupportedTypes = []string{TypeCPU, TypeAllocs, TypeMutex, TypeBlock}

// ValidateType returns an error if the profile type is not supported.
func ValidateType(t string) error {
	switch t {
	case TypeCPU, TypeAllocs, TypeMutex, TypeBlock:
		return nil
	default:
		return fmt.Errorf("%w: %q (supported: cpu, allocs, mutex, block)", ErrUnsupportedType, t)
	}
}

// ExecSpec defines the execution parameters passed to the executor for a single profile pass.
type ExecSpec struct {
	BinaryPath    string
	WorkDir       string
	BenchRegex    string
	Benchtime     string
	Timeout       time.Duration
	ExtraEnv      []string
	CPUProfile    string
	MemProfile    string
	MutexProfile  string
	MutexFraction int
	BlockProfile  string
	BlockRate     int
}

// Executor executes a test binary with the given profile execution spec.
type Executor interface {
	ExecuteProfile(ctx context.Context, spec ExecSpec) error
}

// CollectOptions specifies parameters for a profile collection pass.
type CollectOptions struct {
	BinaryPath string
	WorkDir    string
	Benchmark  string        // Benchmark name (e.g. "BenchmarkFast")
	Iterations int64         // N for -test.benchtime=Nx
	Types      []string      // Profile types to collect (e.g. "cpu", "allocs")
	Runs       int           // Number of runs per side (default 1)
	OutputDir  string        // Target directory for profile files
	Prefix     string        // Filename prefix (e.g. "base" or "head")
	Timeout    time.Duration // Process timeout
	ExtraEnv   []string      // Extra environment variables
}

// File records metadata for a single collected profile file.
type File struct {
	Type string `json:"type"`
	Run  int    `json:"run"`
	Path string `json:"path"`
}

// CollectResult records all profile files collected during a profile pass.
type CollectResult struct {
	Benchmark string `json:"benchmark"`
	Files     []File `json:"files"`
}

// BuildBenchRegex returns an exact match regex for a benchmark.
func BuildBenchRegex(benchmark string) string {
	return "^" + benchmark + "$"
}

// BuildBenchtime formats an iteration count N into the -test.benchtime=Nx syntax.
func BuildBenchtime(iterations int64) string {
	return fmt.Sprintf("%dx", iterations)
}

// Collect executes profile passes for the specified benchmark and types.
// It runs one benchmark per execution with -test.benchtime=Nx.
func Collect(ctx context.Context, exec Executor, opts CollectOptions) (*CollectResult, error) {
	if exec == nil {
		return nil, errors.New("profile: executor is required")
	}
	if opts.BinaryPath == "" {
		return nil, ErrMissingBinary
	}
	if opts.Benchmark == "" {
		return nil, ErrMissingBenchmark
	}
	if opts.Iterations <= 0 {
		return nil, ErrInvalidIterations
	}
	if len(opts.Types) == 0 {
		return nil, errors.New("profile: at least one profile type is required")
	}
	for _, t := range opts.Types {
		if err := ValidateType(t); err != nil {
			return nil, err
		}
	}

	runs := opts.Runs
	if runs <= 0 {
		runs = 1
	}

	if opts.OutputDir != "" {
		if err := os.MkdirAll(opts.OutputDir, 0o750); err != nil {
			return nil, fmt.Errorf("profile: create output dir %s: %w", opts.OutputDir, err)
		}
	}

	result := &CollectResult{
		Benchmark: opts.Benchmark,
		Files:     make([]File, 0, len(opts.Types)*runs),
	}

	benchRegex := BuildBenchRegex(opts.Benchmark)
	benchtime := BuildBenchtime(opts.Iterations)

	for _, t := range opts.Types {
		for runIdx := 1; runIdx <= runs; runIdx++ {
			filename := buildFilename(opts.Prefix, t, runIdx, runs)
			fullPath := filepath.Join(opts.OutputDir, filename)

			spec := ExecSpec{
				BinaryPath: opts.BinaryPath,
				WorkDir:    opts.WorkDir,
				BenchRegex: benchRegex,
				Benchtime:  benchtime,
				Timeout:    opts.Timeout,
				ExtraEnv:   opts.ExtraEnv,
			}

			switch t {
			case TypeCPU:
				spec.CPUProfile = fullPath
			case TypeAllocs:
				spec.MemProfile = fullPath
			case TypeMutex:
				spec.MutexProfile = fullPath
				spec.MutexFraction = 1
			case TypeBlock:
				spec.BlockProfile = fullPath
				spec.BlockRate = 1
			}

			if err := exec.ExecuteProfile(ctx, spec); err != nil {
				return nil, fmt.Errorf("profile: execute %s profile run %d: %w", t, runIdx, err)
			}

			fi, err := os.Stat(fullPath)
			if err != nil || fi.Size() == 0 {
				return nil, fmt.Errorf("profile: %s profile file %s: %w", t, fullPath, ErrProfileEmpty)
			}

			result.Files = append(result.Files, File{
				Type: t,
				Run:  runIdx,
				Path: fullPath,
			})
		}
	}

	return result, nil
}

func buildFilename(prefix, profileType string, runIdx, totalRuns int) string {
	if totalRuns <= 1 {
		if prefix != "" {
			return fmt.Sprintf("%s.%s.pb.gz", prefix, profileType)
		}
		return fmt.Sprintf("%s.pb.gz", profileType)
	}

	if prefix != "" {
		return fmt.Sprintf("%s.%s.run%d.pb.gz", prefix, profileType, runIdx)
	}
	return fmt.Sprintf("%s.run%d.pb.gz", profileType, runIdx)
}
