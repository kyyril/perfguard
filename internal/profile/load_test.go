package profile_test

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"

	pprof "github.com/google/pprof/profile"
	"github.com/kyyril/perfguard/internal/profile"
)

func createSyntheticProfile(allocVal, workerVal int64) *pprof.Profile {
	fnMain := &pprof.Function{ID: 1, Name: "main.main", Filename: "main.go"}
	fnWorker := &pprof.Function{ID: 2, Name: "main.worker", Filename: "worker.go"}
	fnAlloc := &pprof.Function{ID: 3, Name: "main.alloc", Filename: "alloc.go"}

	locMain := &pprof.Location{ID: 1, Line: []pprof.Line{{Function: fnMain}}}
	locWorker := &pprof.Location{ID: 2, Line: []pprof.Line{{Function: fnWorker}}}
	locAlloc := &pprof.Location{ID: 3, Line: []pprof.Line{{Function: fnAlloc}}}

	return &pprof.Profile{
		SampleType: []*pprof.ValueType{
			{Type: "alloc_space", Unit: "bytes"},
			{Type: "alloc_objects", Unit: "count"},
		},
		PeriodType: &pprof.ValueType{Type: "space", Unit: "bytes"},
		Function:   []*pprof.Function{fnMain, fnWorker, fnAlloc},
		Location:   []*pprof.Location{locMain, locWorker, locAlloc},
		Sample: []*pprof.Sample{
			// Leaf is locAlloc (idx 0), caller is worker, caller is main
			{
				Location: []*pprof.Location{locAlloc, locWorker, locMain},
				Value:    []int64{allocVal, 10},
			},
			// Leaf is locWorker, caller is main
			{
				Location: []*pprof.Location{locWorker, locMain},
				Value:    []int64{workerVal, 5},
			},
		},
	}
}

func almostEqual(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func TestLoadAndMerge(t *testing.T) {
	t.Parallel()

	p1 := createSyntheticProfile(1000, 500)
	p2 := createSyntheticProfile(2000, 1000)

	tmpDir := t.TempDir()
	p1Path := filepath.Join(tmpDir, "p1.pb.gz")
	f1, err := os.Create(p1Path)
	if err != nil {
		t.Fatalf("failed creating p1 file: %v", err)
	}
	if err := p1.Write(f1); err != nil {
		_ = f1.Close()
		t.Fatalf("failed writing p1 profile: %v", err)
	}
	_ = f1.Close()

	// 1. Test Load
	loaded, err := profile.Load(p1Path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(loaded.Sample) != 2 {
		t.Fatalf("expected 2 samples in loaded profile, got %d", len(loaded.Sample))
	}

	// 2. Test Merge empty
	if _, err := profile.Merge(nil); !errors.Is(err, profile.ErrNoProfiles) {
		t.Errorf("expected ErrNoProfiles for nil slice, got: %v", err)
	}

	// 3. Test Merge single
	singleMerged, err := profile.Merge([]*pprof.Profile{p1})
	if err != nil {
		t.Fatalf("Merge single profile failed: %v", err)
	}
	if len(singleMerged.Sample) != 2 {
		t.Errorf("expected 2 samples in single merged, got %d", len(singleMerged.Sample))
	}

	// 4. Test Merge multiple
	merged, err := profile.Merge([]*pprof.Profile{p1, p2})
	if err != nil {
		t.Fatalf("Merge multiple profiles failed: %v", err)
	}
	if merged == nil {
		t.Fatal("merged profile is nil")
	}
}

func TestFindSampleIndex(t *testing.T) {
	t.Parallel()

	prof := createSyntheticProfile(100, 50)

	// Explicit type
	idx, err := profile.FindSampleIndex(prof, "alloc_objects")
	if err != nil || idx != 1 {
		t.Errorf("expected idx 1 for alloc_objects, got %d, err %v", idx, err)
	}

	// Default preferred (alloc_space is first preferred)
	defIdx, err := profile.FindSampleIndex(prof, "")
	if err != nil || defIdx != 0 {
		t.Errorf("expected default idx 0 for alloc_space, got %d, err %v", defIdx, err)
	}

	// Unknown type
	_, err = profile.FindSampleIndex(prof, "nonexistent")
	if !errors.Is(err, profile.ErrSampleTypeNotFound) {
		t.Errorf("expected ErrSampleTypeNotFound, got: %v", err)
	}
}

func TestTotalOperations(t *testing.T) {
	t.Parallel()

	// N=99, runs=1 -> 100 ops
	if ops := profile.TotalOperations(99, 1); ops != 100 {
		t.Errorf("expected 100 ops, got %d", ops)
	}

	// N=99, runs=2 -> 200 ops
	if ops := profile.TotalOperations(99, 2); ops != 200 {
		t.Errorf("expected 200 ops, got %d", ops)
	}

	// N=99, runs=0 defaults to 1 run -> 100 ops
	if ops := profile.TotalOperations(99, 0); ops != 100 {
		t.Errorf("expected 100 ops, got %d", ops)
	}
}

func TestExtractFunctions_NormalizedPerOperation(t *testing.T) {
	t.Parallel()

	// Profile 1: alloc=1000, worker=500
	p1 := createSyntheticProfile(1000, 500)
	// Profile 2: alloc=2000, worker=1000
	p2 := createSyntheticProfile(2000, 1000)

	merged, err := profile.Merge([]*pprof.Profile{p1, p2})
	if err != nil {
		t.Fatalf("failed to merge profiles: %v", err)
	}

	// Total operations: N=99 iterations per run, 2 runs
	// TotalOps = 2 * (99 + 1) = 200 operations
	totalOps := profile.TotalOperations(99, 2)
	if totalOps != 200 {
		t.Fatalf("expected 200 total operations, got %d", totalOps)
	}

	// Sample idx 0 is alloc_space (bytes)
	fns, err := profile.ExtractFunctions(merged, 0, totalOps)
	if err != nil {
		t.Fatalf("ExtractFunctions failed: %v", err)
	}

	/*
		Manual calculation:
		Total alloc raw flat = 1000 + 2000 = 3000
		Total alloc raw cum  = 1000 + 2000 = 3000
		alloc Flat per op = 3000 / 200 = 15.0
		alloc Cum per op  = 3000 / 200 = 15.0

		Total worker raw flat = 500 + 1000 = 1500
		Total worker raw cum  = (500 + 1000) + (1000 + 2000) = 4500
		worker Flat per op = 1500 / 200 = 7.5
		worker Cum per op  = 4500 / 200 = 22.5

		Total main raw flat = 0
		Total main raw cum  = (1000 + 500) + (2000 + 1000) = 4500
		main Flat per op = 0.0
		main Cum per op  = 4500 / 200 = 22.5
	*/

	allocFn, ok := fns["main.alloc"]
	if !ok {
		t.Fatalf("missing main.alloc in extracted functions")
	}
	if allocFn.RawFlat != 3000 || allocFn.RawCum != 3000 {
		t.Errorf("main.alloc raw values mismatch: flat=%d cum=%d (expected 3000, 3000)", allocFn.RawFlat, allocFn.RawCum)
	}
	if !almostEqual(allocFn.Flat, 15.0, 1e-9) {
		t.Errorf("main.alloc Flat per op = %f, expected 15.0", allocFn.Flat)
	}
	if !almostEqual(allocFn.Cum, 15.0, 1e-9) {
		t.Errorf("main.alloc Cum per op = %f, expected 15.0", allocFn.Cum)
	}

	workerFn, ok := fns["main.worker"]
	if !ok {
		t.Fatalf("missing main.worker in extracted functions")
	}
	if workerFn.RawFlat != 1500 || workerFn.RawCum != 4500 {
		t.Errorf("main.worker raw values mismatch: flat=%d cum=%d (expected 1500, 4500)", workerFn.RawFlat, workerFn.RawCum)
	}
	if !almostEqual(workerFn.Flat, 7.5, 1e-9) {
		t.Errorf("main.worker Flat per op = %f, expected 7.5", workerFn.Flat)
	}
	if !almostEqual(workerFn.Cum, 22.5, 1e-9) {
		t.Errorf("main.worker Cum per op = %f, expected 22.5", workerFn.Cum)
	}

	mainFn, ok := fns["main.main"]
	if !ok {
		t.Fatalf("missing main.main in extracted functions")
	}
	if mainFn.RawFlat != 0 || mainFn.RawCum != 4500 {
		t.Errorf("main.main raw values mismatch: flat=%d cum=%d (expected 0, 4500)", mainFn.RawFlat, mainFn.RawCum)
	}
	if !almostEqual(mainFn.Flat, 0.0, 1e-9) {
		t.Errorf("main.main Flat per op = %f, expected 0.0", mainFn.Flat)
	}
	if !almostEqual(mainFn.Cum, 22.5, 1e-9) {
		t.Errorf("main.main Cum per op = %f, expected 22.5", mainFn.Cum)
	}
}

func TestExtractFunctions_RecursiveNoDuplicateCum(t *testing.T) {
	t.Parallel()

	fnRec := &pprof.Function{ID: 1, Name: "main.recursive", Filename: "rec.go"}
	locRec := &pprof.Location{ID: 1, Line: []pprof.Line{{Function: fnRec}}}

	// Call stack: recursive -> recursive -> recursive (3 frames of same function)
	prof := &pprof.Profile{
		SampleType: []*pprof.ValueType{{Type: "samples", Unit: "count"}},
		Function:   []*pprof.Function{fnRec},
		Location:   []*pprof.Location{locRec},
		Sample: []*pprof.Sample{
			{
				Location: []*pprof.Location{locRec, locRec, locRec},
				Value:    []int64{100},
			},
		},
	}

	fns, err := profile.ExtractFunctions(prof, 0, 10)
	if err != nil {
		t.Fatalf("ExtractFunctions failed: %v", err)
	}

	rec, ok := fns["main.recursive"]
	if !ok {
		t.Fatal("missing main.recursive")
	}
	// RawCum should be 100, NOT 300!
	if rec.RawCum != 100 {
		t.Errorf("expected RawCum=100 for recursive frames in single sample, got %d", rec.RawCum)
	}
	if rec.RawFlat != 100 {
		t.Errorf("expected RawFlat=100, got %d", rec.RawFlat)
	}
	if !almostEqual(rec.Cum, 10.0, 1e-9) {
		t.Errorf("expected Cum per op = 10.0, got %f", rec.Cum)
	}
}
