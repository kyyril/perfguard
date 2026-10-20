package baseline

import (
	"fmt"

	"github.com/kyyril/perfguard/internal/model"
)

// ReasonEnvMismatch is the reason code when environments or benchmark parameters differ.
const ReasonEnvMismatch = "env_mismatch"

// CompatibilityResult holds the compatibility status and diagnostic differences.
type CompatibilityResult struct {
	Compatible bool     `json:"compatible"`
	Reason     string   `json:"reason,omitempty"`
	Diffs      []string `json:"diffs,omitempty"`
}

// Compatible compares environment and run parameters between base and head runs in stored mode.
// It verifies GOOS, GOARCH, CPU model, GOMAXPROCS, Go minor version, and benchmark configuration.
func Compatible(base, head model.Run) CompatibilityResult {
	var diffs []string

	// 1. Operating System
	if base.Env.OS != "" && head.Env.OS != "" && base.Env.OS != head.Env.OS {
		diffs = append(diffs, fmt.Sprintf("OS mismatch: base %s vs head %s", base.Env.OS, head.Env.OS))
	}

	// 2. CPU Architecture
	if base.Env.Arch != "" && head.Env.Arch != "" && base.Env.Arch != head.Env.Arch {
		diffs = append(diffs, fmt.Sprintf("Arch mismatch: base %s vs head %s", base.Env.Arch, head.Env.Arch))
	}

	// 3. CPU Model
	if base.Env.CPUModel != "" && head.Env.CPUModel != "" && base.Env.CPUModel != head.Env.CPUModel {
		diffs = append(diffs, fmt.Sprintf("CPU model mismatch: base %s vs head %s", base.Env.CPUModel, head.Env.CPUModel))
	}

	// 4. GOMAXPROCS
	if base.Env.GOMAXPROCS > 0 && head.Env.GOMAXPROCS > 0 && base.Env.GOMAXPROCS != head.Env.GOMAXPROCS {
		diffs = append(diffs, fmt.Sprintf("GOMAXPROCS mismatch: base %d vs head %d", base.Env.GOMAXPROCS, head.Env.GOMAXPROCS))
	}

	// 5. Go Minor Version
	baseMinor := parseGoMinorVersion(base.Env.GoVersion)
	headMinor := parseGoMinorVersion(head.Env.GoVersion)
	if baseMinor != "" && headMinor != "" && baseMinor != headMinor {
		diffs = append(diffs, fmt.Sprintf("Go minor version mismatch: base %s vs head %s", baseMinor, headMinor))
	}

	// 6. Benchmark Run Parameters
	if base.Params.Benchtime != "" && head.Params.Benchtime != "" && base.Params.Benchtime != head.Params.Benchtime {
		diffs = append(diffs, fmt.Sprintf("Benchtime mismatch: base %s vs head %s", base.Params.Benchtime, head.Params.Benchtime))
	}
	if base.Params.Count > 0 && head.Params.Count > 0 && base.Params.Count != head.Params.Count {
		diffs = append(diffs, fmt.Sprintf("Count mismatch: base %d vs head %d", base.Params.Count, head.Params.Count))
	}
	if base.Params.CPU > 0 && head.Params.CPU > 0 && base.Params.CPU != head.Params.CPU {
		diffs = append(diffs, fmt.Sprintf("CPU procs mismatch: base %d vs head %d", base.Params.CPU, head.Params.CPU))
	}
	if base.Params.Include != head.Params.Include {
		diffs = append(diffs, fmt.Sprintf("Include filter mismatch: base %q vs head %q", base.Params.Include, head.Params.Include))
	}

	// 7. General parameter/env hash check
	baseHash := ComputeEnvHash(base.Env, base.Params)
	headHash := ComputeEnvHash(head.Env, head.Params)
	if len(diffs) == 0 && baseHash != headHash {
		diffs = append(diffs, fmt.Sprintf("Parameter hash mismatch: base %s vs head %s", baseHash, headHash))
	}

	if len(diffs) > 0 {
		return CompatibilityResult{
			Compatible: false,
			Reason:     ReasonEnvMismatch,
			Diffs:      diffs,
		}
	}

	return CompatibilityResult{
		Compatible: true,
	}
}
