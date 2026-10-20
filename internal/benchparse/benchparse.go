// Package benchparse parses Go benchmark output and maps it to model types.
// It wraps golang.org/x/perf/benchfmt for format handling, normalizes benchmark
// names (stripping GOMAXPROCS suffixes and preserving sub-benchmarks), and
// accumulates samples across iterations.
package benchparse

import (
	"fmt"
	"io"
	"regexp"
	"strconv"

	"golang.org/x/perf/benchfmt"

	"github.com/kyyril/perfguard/internal/model"
)

// procsRe matches the trailing GOMAXPROCS suffix "-N" on a benchmark name.
var procsRe = regexp.MustCompile(`-(\d+)$`)

// Parse reads Go benchmark output from r, associates results with pkg, and
// returns one BenchmarkResult per normalized name. Multiple measurements of
// the same benchmark accumulate as additional Samples.
//
// Non-benchmark lines, configuration headers, and malformed lines are ignored,
// allowing valid samples from truncated or noisy outputs to be extracted.
func Parse(r io.Reader, pkg string) ([]model.BenchmarkResult, error) {
	reader := benchfmt.NewReader(r, pkg)

	index := make(map[string]int)
	var results []model.BenchmarkResult

	for reader.Scan() {
		rec := reader.Result()
		res, ok := rec.(*benchfmt.Result)
		if !ok {
			// Ignore syntax errors, comments, and non-benchmark records
			// so partially truncated or noisy outputs still yield valid samples.
			continue
		}

		name, procs := normalizeName(string(res.Name.Full()))
		if name == "" {
			continue
		}

		sample := buildSample(res)

		if idx, exists := index[name]; exists {
			results[idx].Samples = append(results[idx].Samples, sample)
			if results[idx].Procs == 0 && procs > 0 {
				results[idx].Procs = procs
			}
		} else {
			index[name] = len(results)
			results = append(results, model.BenchmarkResult{
				Package: pkg,
				Name:    name,
				Procs:   procs,
				Samples: []model.Sample{sample},
			})
		}
	}

	if err := reader.Err(); err != nil {
		return nil, fmt.Errorf("benchparse: read error in %s: %w", pkg, err)
	}

	return results, nil
}

// normalizeName strips the trailing GOMAXPROCS "-N" suffix from the full
// benchmark name, extracts the procs value, and ensures the benchmark name
// retains the canonical "Benchmark" prefix.
//
// Examples:
//
//	"Foo-8"                  -> "BenchmarkFoo", 8
//	"Foo/sub-4"              -> "BenchmarkFoo/sub", 4
//	"Foo"                    -> "BenchmarkFoo", 0
//	"BenchmarkPrefixed-8"    -> "BenchmarkBenchmarkPrefixed", 8
func normalizeName(full string) (name string, procs int) {
	if len(full) == 0 {
		return "", 0
	}
	m := procsRe.FindStringSubmatchIndex(full)
	clean := full
	if m != nil {
		p, err := strconv.Atoi(full[m[2]:m[3]])
		if err == nil {
			procs = p
			clean = full[:m[0]]
		}
	}
	if len(clean) == 0 {
		return "", procs
	}
	// benchfmt always strips the leading "Benchmark" keyword from the line.
	// We restore it unconditionally to yield the full canonical benchmark name.
	return "Benchmark" + clean, procs
}

// buildSample converts a benchfmt.Result into a model.Sample.
// benchfmt automatically converts standard time metrics to "sec/op" and
// memory to "B/op". Custom metrics (such as "p95-ms") are captured as-is.
func buildSample(res *benchfmt.Result) model.Sample {
	values := make(map[string]float64, len(res.Values))
	for _, v := range res.Values {
		values[v.Unit] = v.Value
	}
	return model.Sample{
		Iterations: int64(res.Iters),
		Values:     values,
	}
}
