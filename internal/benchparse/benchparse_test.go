package benchparse_test

import (
	"errors"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/kyyril/perfguard/internal/benchparse"
	"github.com/kyyril/perfguard/internal/model"
)

type errorReader struct {
	err error
}

func (e *errorReader) Read(_ []byte) (int, error) {
	return 0, e.err
}

func TestParseStandardFixture(t *testing.T) {
	data, err := os.ReadFile("../../testdata/benchparse/standard.txt")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	results, err := benchparse.Parse(strings.NewReader(string(data)), "example/pkg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 4 {
		t.Fatalf("expected 4 benchmarks, got %d", len(results))
	}

	// Verify BenchmarkSimple has 2 samples and procs 8.
	simple := results[0]
	if simple.Name != "BenchmarkSimple" {
		t.Errorf("expected name BenchmarkSimple, got %q", simple.Name)
	}
	if simple.Procs != 8 {
		t.Errorf("expected procs 8, got %d", simple.Procs)
	}
	if simple.Package != "example/pkg" {
		t.Errorf("expected package example/pkg, got %q", simple.Package)
	}
	if len(simple.Samples) != 2 {
		t.Fatalf("expected 2 samples for BenchmarkSimple, got %d", len(simple.Samples))
	}

	// Check unit conversion: ns/op to sec/op.
	s0 := simple.Samples[0]
	expectedSec := 1042e-9
	if diff := cmp.Diff(expectedSec, s0.Values["sec/op"], cmpopts.EquateApprox(0, 1e-12)); diff != "" {
		t.Errorf("sec/op mismatch (-want +got):\n%s", diff)
	}
	if s0.Values["B/op"] != 32 {
		t.Errorf("expected 32 B/op, got %v", s0.Values["B/op"])
	}
	if s0.Values["allocs/op"] != 2 {
		t.Errorf("expected 2 allocs/op, got %v", s0.Values["allocs/op"])
	}

	// Verify sub-benchmarks.
	sub1 := results[1]
	if sub1.Name != "BenchmarkSub/case1" {
		t.Errorf("expected name BenchmarkSub/case1, got %q", sub1.Name)
	}
	if sub1.Procs != 8 {
		t.Errorf("expected procs 8, got %d", sub1.Procs)
	}

	sub2 := results[2]
	if sub2.Name != "BenchmarkSub/case2" {
		t.Errorf("expected name BenchmarkSub/case2, got %q", sub2.Name)
	}

	// Verify benchmark without -N suffix.
	noProcs := results[3]
	if noProcs.Name != "BenchmarkNoProcs" {
		t.Errorf("expected name BenchmarkNoProcs, got %q", noProcs.Name)
	}
	if noProcs.Procs != 0 {
		t.Errorf("expected procs 0, got %d", noProcs.Procs)
	}
}

func TestParseCustomMetrics(t *testing.T) {
	data, err := os.ReadFile("../../testdata/benchparse/custom_metrics.txt")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	results, err := benchparse.Parse(strings.NewReader(string(data)), "example/pkg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 benchmarks, got %d", len(results))
	}

	latency := results[0]
	if latency.Name != "BenchmarkLatency" {
		t.Errorf("expected BenchmarkLatency, got %q", latency.Name)
	}
	if len(latency.Samples) != 2 {
		t.Fatalf("expected 2 samples, got %d", len(latency.Samples))
	}

	// Check custom metric p95-ms.
	if math.Abs(latency.Samples[0].Values["p95-ms"]-1.52) > 1e-6 {
		t.Errorf("expected 1.52 p95-ms, got %v", latency.Samples[0].Values["p95-ms"])
	}
	if math.Abs(latency.Samples[1].Values["p95-ms"]-1.48) > 1e-6 {
		t.Errorf("expected 1.48 p95-ms, got %v", latency.Samples[1].Values["p95-ms"])
	}

	throughput := results[1]
	if throughput.Name != "BenchmarkThroughput/large" {
		t.Errorf("expected BenchmarkThroughput/large, got %q", throughput.Name)
	}
	if throughput.Procs != 16 {
		t.Errorf("expected procs 16, got %d", throughput.Procs)
	}
	if math.Abs(throughput.Samples[0].Values["p95-ms"]-3.10) > 1e-6 {
		t.Errorf("expected 3.10 p95-ms, got %v", throughput.Samples[0].Values["p95-ms"])
	}
}

func TestParseTruncatedFixture(t *testing.T) {
	data, err := os.ReadFile("../../testdata/benchparse/truncated.txt")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	results, err := benchparse.Parse(strings.NewReader(string(data)), "example/pkg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 valid benchmarks from truncated input, got %d", len(results))
	}

	if results[0].Name != "BenchmarkValid1" {
		t.Errorf("expected BenchmarkValid1, got %q", results[0].Name)
	}
	if len(results[0].Samples) != 2 {
		t.Errorf("expected 2 samples for BenchmarkValid1, got %d", len(results[0].Samples))
	}

	if results[1].Name != "BenchmarkValid2/sub" {
		t.Errorf("expected BenchmarkValid2/sub, got %q", results[1].Name)
	}
	if len(results[1].Samples) != 1 {
		t.Errorf("expected 1 sample for BenchmarkValid2/sub, got %d", len(results[1].Samples))
	}
}

func TestParseEmptyFixture(t *testing.T) {
	data, err := os.ReadFile("../../testdata/benchparse/empty.txt")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	results, err := benchparse.Parse(strings.NewReader(string(data)), "example/pkg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestParseEmptyInput(t *testing.T) {
	results, err := benchparse.Parse(strings.NewReader(""), "example/pkg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestParseReaderError(t *testing.T) {
	expectedErr := errors.New("read failed")
	r := &errorReader{err: expectedErr}
	_, err := benchparse.Parse(r, "example/pkg")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
}

func TestNormalizeNameTable(t *testing.T) {
	tests := []struct {
		input     string
		wantName  string
		wantProcs int
		wantCount int
	}{
		{
			input:     "BenchmarkSingle-8 100 100 ns/op\n",
			wantName:  "BenchmarkSingle",
			wantProcs: 8,
			wantCount: 1,
		},
		{
			input:     "BenchmarkSub/test-4 100 100 ns/op\n",
			wantName:  "BenchmarkSub/test",
			wantProcs: 4,
			wantCount: 1,
		},
		{
			input:     "BenchmarkSub/case=1/step-12 100 100 ns/op\n",
			wantName:  "BenchmarkSub/case=1/step",
			wantProcs: 12,
			wantCount: 1,
		},
		{
			input:     "BenchmarkHyphen-Name-8 100 100 ns/op\n",
			wantName:  "BenchmarkHyphen-Name",
			wantProcs: 8,
			wantCount: 1,
		},
		{
			input:     "BenchmarkWithoutProcs 100 100 ns/op\n",
			wantName:  "BenchmarkWithoutProcs",
			wantProcs: 0,
			wantCount: 1,
		},
		{
			input:     "BenchmarkBenchmarkPrefixed-8 100 100 ns/op\n",
			wantName:  "BenchmarkBenchmarkPrefixed",
			wantProcs: 8,
			wantCount: 1,
		},
		{
			input:     "Benchmark-8 100 100 ns/op\n",
			wantName:  "",
			wantProcs: 0,
			wantCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			res, err := benchparse.Parse(strings.NewReader(tc.input), "test/pkg")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(res) != tc.wantCount {
				t.Fatalf("expected %d results, got %d", tc.wantCount, len(res))
			}
			if tc.wantCount > 0 {
				if res[0].Name != tc.wantName {
					t.Errorf("expected name %q, got %q", tc.wantName, res[0].Name)
				}
				if res[0].Procs != tc.wantProcs {
					t.Errorf("expected procs %d, got %d", tc.wantProcs, res[0].Procs)
				}
			}
		})
	}
}

func TestAccumulateMultipleSamples(t *testing.T) {
	input := `BenchmarkA-8 100 100 ns/op
BenchmarkA-8 100 110 ns/op
BenchmarkA-8 100 95 ns/op
`
	results, err := benchparse.Parse(strings.NewReader(input), "pkg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 benchmark, got %d", len(results))
	}
	if len(results[0].Samples) != 3 {
		t.Fatalf("expected 3 samples, got %d", len(results[0].Samples))
	}
}

func TestProcsUpdateOnSubsequentSample(t *testing.T) {
	input := `BenchmarkDynamic 100 100 ns/op
BenchmarkDynamic-8 100 105 ns/op
`
	results, err := benchparse.Parse(strings.NewReader(input), "pkg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 benchmark, got %d", len(results))
	}
	if results[0].Procs != 8 {
		t.Errorf("expected procs to update to 8, got %d", results[0].Procs)
	}
	if len(results[0].Samples) != 2 {
		t.Fatalf("expected 2 samples, got %d", len(results[0].Samples))
	}
}

func TestModelSampleStructure(t *testing.T) {
	input := `BenchmarkOne-4 500 200 ns/op 64 B/op 1 allocs/op
`
	results, err := benchparse.Parse(strings.NewReader(input), "pkg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []model.BenchmarkResult{
		{
			Package: "pkg",
			Name:    "BenchmarkOne",
			Procs:   4,
			Samples: []model.Sample{
				{
					Iterations: 500,
					Values: map[string]float64{
						"sec/op":    200e-9,
						"B/op":      64,
						"allocs/op": 1,
					},
				},
			},
		},
	}

	approxOpt := cmpopts.EquateApprox(0, 1e-12)
	if diff := cmp.Diff(expected, results, approxOpt); diff != "" {
		t.Errorf("results mismatch (-want +got):\n%s", diff)
	}
}
