package runner

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"github.com/kyyril/perfguard/internal/envinfo"
)

// PreflightOptions configures preflight noise and affinity checks.
type PreflightOptions struct {
	NumCPU  int
	PinCPUs string
}

// PreflightResult contains diagnostics and warnings discovered prior to measurement.
type PreflightResult struct {
	Diagnostics envinfo.Preflight `json:"diagnostics"`
	Warnings    []string          `json:"warnings"`
	PinWarning  string            `json:"pin_warning,omitempty"`
}

// CheckPreflight evaluates system noise (load, governor, cgroups) and CPU pinning feasibility.
func CheckPreflight(opts PreflightOptions) PreflightResult {
	numCPU := opts.NumCPU
	if numCPU <= 0 {
		numCPU = runtime.NumCPU()
	}

	diag := envinfo.CheckPreflight(numCPU)
	var warnings []string
	warnings = append(warnings, diag.Warnings...)

	var pinWarning string
	if opts.PinCPUs != "" {
		if runtime.GOOS != "linux" {
			pinWarning = fmt.Sprintf("CPU pinning (%q) is only supported on Linux and was ignored on %s", opts.PinCPUs, runtime.GOOS)
			warnings = append(warnings, pinWarning)
		} else {
			if _, err := parseCPUSet(opts.PinCPUs); err != nil {
				pinWarning = fmt.Sprintf("invalid pin_cpus specification %q: %v", opts.PinCPUs, err)
				warnings = append(warnings, pinWarning)
			}
		}
	}

	return PreflightResult{
		Diagnostics: diag,
		Warnings:    warnings,
		PinWarning:  pinWarning,
	}
}

// parseCPUSet parses CPU affinity expressions such as "0", "0-3", "0,2,4", or "0-1,4-5".
func parseCPUSet(expr string) ([]int, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, nil
	}

	var cpus []int
	seen := make(map[int]bool)

	groups := strings.Split(expr, ",")
	for _, g := range groups {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}

		if strings.Contains(g, "-") {
			parts := strings.SplitN(g, "-", 2)
			start, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
			end, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("invalid cpu range: %s", g)
			}
			if start < 0 || end < start {
				return nil, fmt.Errorf("invalid cpu range bounds: %s", g)
			}
			for c := start; c <= end; c++ {
				if !seen[c] {
					seen[c] = true
					cpus = append(cpus, c)
				}
			}
		} else {
			cpu, err := strconv.Atoi(g)
			if err != nil || cpu < 0 {
				return nil, fmt.Errorf("invalid cpu number: %s", g)
			}
			if !seen[cpu] {
				seen[cpu] = true
				cpus = append(cpus, cpu)
			}
		}
	}

	if len(cpus) == 0 {
		return nil, fmt.Errorf("empty cpu set")
	}

	return cpus, nil
}
