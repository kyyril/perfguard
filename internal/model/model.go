package model

import "time"

// CurrentSchemaVersion defines the active schema version for serialization.
const CurrentSchemaVersion = 1

// Side represents the benchmark target side: base or head.
type Side string

const (
	SideBase Side = "base"
	SideHead Side = "head"
)

// GitInfo stores commit information.
type GitInfo struct {
	SHA   string `json:"sha"`
	Ref   string `json:"ref"`
	Dirty bool   `json:"dirty"`
}

// Environment records hardware and runtime metadata.
type Environment struct {
	OS            string            `json:"os"`
	Arch          string            `json:"arch"`
	GoVersion     string            `json:"go_version"`
	NumCPU        int               `json:"num_cpu"`
	CPUModel      string            `json:"cpu_model"`
	GOMAXPROCS    int               `json:"gomaxprocs"`
	Hostname      string            `json:"hostname,omitempty"`
	KernelVersion string            `json:"kernel_version,omitempty"`
	Extra         map[string]string `json:"extra,omitempty"`
}

// RunParams stores parameters used during benchmark execution.
type RunParams struct {
	Count          int      `json:"count"`
	Benchtime      string   `json:"benchtime"`
	CPU            int      `json:"cpu"`
	Include        string   `json:"include"`
	Exclude        []string `json:"exclude,omitempty"`
	Mode           string   `json:"mode"`
	WarmupRuns     int      `json:"warmup_runs"`
	MaxExtraRuns   int      `json:"max_extra_runs"`
	ExtraRoundStep int      `json:"extra_round_step"`
	ExtraRuns      int      `json:"extra_runs,omitempty"`
}

// Sample contains single iteration counts and metric values.
type Sample struct {
	Iterations int64              `json:"iterations"`
	Values     map[string]float64 `json:"values"`
}

// BenchmarkResult stores samples for a specific benchmark.
type BenchmarkResult struct {
	Package string   `json:"package"`
	Name    string   `json:"name"`
	Procs   int      `json:"procs"`
	Samples []Sample `json:"samples"`
}

// ProfileRef points to a generated pprof profile.
type ProfileRef struct {
	Benchmark string `json:"benchmark"`
	Type      string `json:"type"`
	Path      string `json:"path"`
	Runs      int    `json:"runs"`
}

// Run represents raw benchmark outputs and metadata for one side.
type Run struct {
	SchemaVersion int               `json:"schema_version"`
	ID            string            `json:"id"`
	Side          Side              `json:"side"`
	CreatedAt     time.Time         `json:"created_at"`
	Git           GitInfo           `json:"git"`
	Env           Environment       `json:"env"`
	Params        RunParams         `json:"params"`
	Benchmarks    []BenchmarkResult `json:"benchmarks"`
	Profiles      []ProfileRef      `json:"profiles,omitempty"`
}

// RunSummary summarizes a run for inclusion in comparison outputs.
type RunSummary struct {
	ID        string      `json:"id"`
	Side      Side        `json:"side"`
	CreatedAt time.Time   `json:"created_at"`
	Git       GitInfo     `json:"git"`
	Env       Environment `json:"env"`
}

// Summary creates a RunSummary from a Run.
func (r Run) Summary() RunSummary {
	return RunSummary{
		ID:        r.ID,
		Side:      r.Side,
		CreatedAt: r.CreatedAt,
		Git:       r.Git,
		Env:       r.Env,
	}
}

// Verdict is the overall result of a comparison.
type Verdict string

const (
	VerdictPass         Verdict = "pass"
	VerdictInconclusive Verdict = "inconclusive"
	VerdictWarn         Verdict = "warn"
	VerdictFail         Verdict = "fail"
)

// Rank returns the numeric severity of a verdict for comparison.
// Higher values indicate worse outcome: fail > warn > inconclusive > pass.
func (v Verdict) Rank() int {
	switch v {
	case VerdictFail:
		return 4
	case VerdictWarn:
		return 3
	case VerdictInconclusive:
		return 2
	case VerdictPass:
		return 1
	default:
		return 0
	}
}

// WorstVerdict returns the more severe verdict between a and b.
func WorstVerdict(a, b Verdict) Verdict {
	if a.Rank() >= b.Rank() {
		return a
	}
	return b
}

// Status represents the state of a single benchmark metric comparison.
type Status string

const (
	StatusUnchanged    Status = "unchanged"
	StatusImproved     Status = "improved"
	StatusWarn         Status = "warn"
	StatusFail         Status = "fail"
	StatusInconclusive Status = "inconclusive"
	StatusNew          Status = "new"
	StatusRemoved      Status = "removed"
)

// Reason code constants matching decision rules.
const (
	ReasonNewBenchmark        = "new_benchmark"
	ReasonRemovedBenchmark    = "removed_benchmark"
	ReasonInsufficientSamples = "insufficient_samples"
	ReasonEnvMismatch         = "env_mismatch"
	ReasonNoisy               = "noisy"
	ReasonZeroBase            = "zero_base"
	ReasonNotSignificant      = "not_significant"
	ReasonExceedsFail         = "exceeds_fail"
	ReasonExceedsWarn         = "exceeds_warn"
	ReasonImprovement         = "improvement"
	ReasonBelowThreshold      = "below_threshold"
)

// MetricComparison represents the statistical comparison of one metric for one benchmark.
type MetricComparison struct {
	Benchmark  string     `json:"benchmark"`
	Metric     string     `json:"metric"`
	BaseMedian float64    `json:"base_median"`
	HeadMedian float64    `json:"head_median"`
	DeltaPct   float64    `json:"delta_pct"`
	CI         [2]float64 `json:"ci"`
	PValue     float64    `json:"p_value"`
	CVBase     float64    `json:"cv_base"`
	CVHead     float64    `json:"cv_head"`
	NBase      int        `json:"n_base"`
	NHead      int        `json:"n_head"`
	Status     Status     `json:"status"`
	Reason     string     `json:"reason"`
}

// Candidate represents a function contributing to performance regression.
type Candidate struct {
	Function  string  `json:"function"`
	FlatDelta float64 `json:"flat_delta"`
	CumDelta  float64 `json:"cum_delta"`
	Touched   bool    `json:"touched"`
}

// Diagnosis contains pprof analysis results for regressed benchmarks.
type Diagnosis struct {
	Benchmark   string      `json:"benchmark"`
	ProfileType string      `json:"profile_type"`
	Candidates  []Candidate `json:"candidates"`
	Notes       []string    `json:"notes,omitempty"`
}

// Comparison represents the complete comparative report between base and head runs.
type Comparison struct {
	SchemaVersion int                `json:"schema_version"`
	Base          RunSummary         `json:"base"`
	Head          RunSummary         `json:"head"`
	Items         []MetricComparison `json:"items"`
	Verdict       Verdict            `json:"verdict"`
	Diagnosis     []Diagnosis        `json:"diagnosis,omitempty"`
	Notes         []string           `json:"notes,omitempty"`
}
