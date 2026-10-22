package profile

import (
	"errors"
	"fmt"
	"os"

	pprof "github.com/google/pprof/profile"
)

var (
	// ErrNoProfiles indicates an empty list of profiles was provided for merging.
	ErrNoProfiles = errors.New("profile: no profiles provided")

	// ErrSampleTypeNotFound indicates requested sample type is not in the profile.
	ErrSampleTypeNotFound = errors.New("profile: sample type not found")

	// ErrInvalidTotalOps indicates total operations count is <= 0.
	ErrInvalidTotalOps = errors.New("profile: total operations must be positive")
)

// FunctionValue holds unscaled and per-operation flat and cumulative metrics for a function.
type FunctionValue struct {
	Name     string  `json:"name"`
	FileName string  `json:"file_name"`
	Flat     float64 `json:"flat"`
	Cum      float64 `json:"cum"`
	RawFlat  int64   `json:"raw_flat"`
	RawCum   int64   `json:"raw_cum"`
}

// Load reads and parses a pprof profile from a file path.
func Load(path string) (*pprof.Profile, error) {
	if path == "" {
		return nil, errors.New("profile: path is required")
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("profile: open file %s: %w", path, err)
	}
	defer f.Close()

	prof, err := pprof.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("profile: parse file %s: %w", path, err)
	}

	return prof, nil
}

// Merge merges multiple pprof profiles into a single profile using pprof.Merge.
func Merge(profiles []*pprof.Profile) (*pprof.Profile, error) {
	if len(profiles) == 0 {
		return nil, ErrNoProfiles
	}
	if len(profiles) == 1 {
		return profiles[0], nil
	}

	for _, p := range profiles {
		if p != nil && p.PeriodType == nil && len(p.SampleType) > 0 {
			p.PeriodType = p.SampleType[0]
		}
	}

	merged, err := pprof.Merge(profiles)
	if err != nil {
		return nil, fmt.Errorf("profile: merge failed: %w", err)
	}

	return merged, nil
}

// TotalOperations calculates the total operations executed given requested iterations N
// per run and the number of runs. In Go's benchmark framework, -test.benchtime=Nx executes
// N + 1 iterations per run.
func TotalOperations(iterations int64, runs int) int64 {
	if runs <= 0 {
		runs = 1
	}
	return int64(runs) * (iterations + 1)
}

// FindSampleIndex finds the index of the requested sample type in the profile.
// If sampleType is empty, it returns the index of preferred sample types based on
// common profile patterns (alloc_space, samples, cpu, contentions, delay) or 0.
func FindSampleIndex(prof *pprof.Profile, sampleType string) (int, error) {
	if prof == nil || len(prof.SampleType) == 0 {
		return -1, errors.New("profile: profile has no sample types")
	}

	if sampleType != "" {
		for i, st := range prof.SampleType {
			if st.Type == sampleType {
				return i, nil
			}
		}
		return -1, fmt.Errorf("%w: %q", ErrSampleTypeNotFound, sampleType)
	}

	// Default preferred order
	preferred := []string{"alloc_space", "samples", "cpu", "contentions", "delay"}
	for _, pref := range preferred {
		for i, st := range prof.SampleType {
			if st.Type == pref {
				return i, nil
			}
		}
	}

	// Fallback to first sample type
	return 0, nil
}

// ExtractFunctions extracts all functions from a profile, computing raw and per-operation
// normalized flat and cumulative values for the given sample index.
func ExtractFunctions(prof *pprof.Profile, sampleIdx int, totalOps int64) (map[string]FunctionValue, error) {
	if prof == nil {
		return nil, errors.New("profile: profile is nil")
	}
	if sampleIdx < 0 || sampleIdx >= len(prof.SampleType) {
		return nil, fmt.Errorf("profile: sample index %d out of range [0, %d)", sampleIdx, len(prof.SampleType))
	}
	if totalOps <= 0 {
		return nil, ErrInvalidTotalOps
	}

	opsFloat := float64(totalOps)
	res := make(map[string]FunctionValue)

	for _, sample := range prof.Sample {
		if sampleIdx >= len(sample.Value) {
			continue
		}
		val := sample.Value[sampleIdx]
		if val == 0 {
			continue
		}

		// Track cumulative visits in this sample to avoid double counting recursive frames
		seenInSample := make(map[string]bool)

		for locIdx, loc := range sample.Location {
			for lineIdx, line := range loc.Line {
				fn := line.Function
				if fn == nil || fn.Name == "" {
					continue
				}

				name := fn.Name
				fv, exists := res[name]
				if !exists {
					fv = FunctionValue{
						Name:     name,
						FileName: fn.Filename,
					}
				}

				// Flat cost is assigned to the leaf frame (loc 0, innermost line 0)
				if locIdx == 0 && lineIdx == 0 {
					fv.RawFlat += val
				}

				// Cumulative cost is counted once per unique function in this stack trace
				if !seenInSample[name] {
					seenInSample[name] = true
					fv.RawCum += val
				}

				res[name] = fv
			}
		}
	}

	// Normalize per operation
	for name, fv := range res {
		fv.Flat = float64(fv.RawFlat) / opsFloat
		fv.Cum = float64(fv.RawCum) / opsFloat
		res[name] = fv
	}

	return res, nil
}
