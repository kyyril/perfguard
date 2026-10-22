package profile_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kyyril/perfguard/internal/profile"
)

type mockExecutor struct {
	calls     []profile.ExecSpec
	errOnCall int
	touchFile bool
	emptyFile bool
}

func (m *mockExecutor) ExecuteProfile(_ context.Context, spec profile.ExecSpec) error {
	m.calls = append(m.calls, spec)
	callNum := len(m.calls)
	if m.errOnCall == callNum {
		return errors.New("mock execution failed")
	}

	if m.touchFile {
		var path string
		switch {
		case spec.CPUProfile != "":
			path = spec.CPUProfile
		case spec.MemProfile != "":
			path = spec.MemProfile
		case spec.MutexProfile != "":
			path = spec.MutexProfile
		case spec.BlockProfile != "":
			path = spec.BlockProfile
		}

		if path != "" {
			_ = os.MkdirAll(filepath.Dir(path), 0o750)
			content := []byte("synthetic-profile-data")
			if m.emptyFile {
				content = []byte{}
			}
			_ = os.WriteFile(path, content, 0o600)
		}
	}

	return nil
}

func TestValidateType(t *testing.T) {
	t.Parallel()

	valid := []string{profile.TypeCPU, profile.TypeAllocs, profile.TypeMutex, profile.TypeBlock}
	for _, vt := range valid {
		if err := profile.ValidateType(vt); err != nil {
			t.Errorf("expected %q to be valid, got: %v", vt, err)
		}
	}

	invalid := []string{"", "memory", "goroutine", "trace", "disk"}
	for _, iv := range invalid {
		if err := profile.ValidateType(iv); !errors.Is(err, profile.ErrUnsupportedType) {
			t.Errorf("expected ErrUnsupportedType for %q, got: %v", iv, err)
		}
	}
}

func TestBuildHelpers(t *testing.T) {
	t.Parallel()

	if got := profile.BuildBenchRegex("BenchmarkFast"); got != "^BenchmarkFast$" {
		t.Errorf("expected ^BenchmarkFast$, got %q", got)
	}

	if got := profile.BuildBenchtime(5000); got != "5000x" {
		t.Errorf("expected 5000x, got %q", got)
	}
}

func TestCollect_ValidationErrors(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	mock := &mockExecutor{}

	tests := []struct {
		name    string
		exec    profile.Executor
		opts    profile.CollectOptions
		wantErr error
	}{
		{
			name: "nil executor",
			exec: nil,
			opts: profile.CollectOptions{
				BinaryPath: "test.exe",
				Benchmark:  "BenchmarkFast",
				Iterations: 100,
				Types:      []string{profile.TypeCPU},
			},
			wantErr: errors.New("profile: executor is required"),
		},
		{
			name: "missing binary",
			exec: mock,
			opts: profile.CollectOptions{
				BinaryPath: "",
				Benchmark:  "BenchmarkFast",
				Iterations: 100,
				Types:      []string{profile.TypeCPU},
			},
			wantErr: profile.ErrMissingBinary,
		},
		{
			name: "missing benchmark",
			exec: mock,
			opts: profile.CollectOptions{
				BinaryPath: "test.exe",
				Benchmark:  "",
				Iterations: 100,
				Types:      []string{profile.TypeCPU},
			},
			wantErr: profile.ErrMissingBenchmark,
		},
		{
			name: "invalid iterations",
			exec: mock,
			opts: profile.CollectOptions{
				BinaryPath: "test.exe",
				Benchmark:  "BenchmarkFast",
				Iterations: 0,
				Types:      []string{profile.TypeCPU},
			},
			wantErr: profile.ErrInvalidIterations,
		},
		{
			name: "missing types",
			exec: mock,
			opts: profile.CollectOptions{
				BinaryPath: "test.exe",
				Benchmark:  "BenchmarkFast",
				Iterations: 100,
				Types:      nil,
			},
			wantErr: errors.New("profile: at least one profile type is required"),
		},
		{
			name: "unsupported type",
			exec: mock,
			opts: profile.CollectOptions{
				BinaryPath: "test.exe",
				Benchmark:  "BenchmarkFast",
				Iterations: 100,
				Types:      []string{"invalid-type"},
			},
			wantErr: profile.ErrUnsupportedType,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := profile.Collect(context.Background(), tc.exec, tc.opts)
			if err == nil {
				t.Fatalf("expected error containing %v, got nil", tc.wantErr)
			}
			if !errors.Is(err, tc.wantErr) && err.Error() != tc.wantErr.Error() {
				t.Errorf("expected error %v, got: %v", tc.wantErr, err)
			}
		})
	}
	_ = tmpDir
}

func TestCollect_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	mock := &mockExecutor{touchFile: true}

	opts := profile.CollectOptions{
		BinaryPath: "/path/to/bin.test",
		WorkDir:    "/path/to/workdir",
		Benchmark:  "BenchmarkAlloc",
		Iterations: 2000,
		Types:      []string{profile.TypeCPU, profile.TypeAllocs, profile.TypeMutex, profile.TypeBlock},
		Runs:       2,
		OutputDir:  tmpDir,
		Prefix:     "head",
		Timeout:    10 * time.Second,
	}

	result, err := profile.Collect(context.Background(), mock, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Benchmark != "BenchmarkAlloc" {
		t.Errorf("expected benchmark BenchmarkAlloc, got %q", result.Benchmark)
	}

	// 4 types * 2 runs = 8 files
	if len(result.Files) != 8 {
		t.Fatalf("expected 8 files, got %d", len(result.Files))
	}
	if len(mock.calls) != 8 {
		t.Fatalf("expected 8 executor calls, got %d", len(mock.calls))
	}

	// Verify specs passed to executor
	for i, call := range mock.calls {
		if call.BenchRegex != "^BenchmarkAlloc$" {
			t.Errorf("call %d: expected regex ^BenchmarkAlloc$, got %q", i, call.BenchRegex)
		}
		if call.Benchtime != "2000x" {
			t.Errorf("call %d: expected benchtime 2000x, got %q", i, call.Benchtime)
		}
		if call.BinaryPath != "/path/to/bin.test" {
			t.Errorf("call %d: expected binary /path/to/bin.test, got %q", i, call.BinaryPath)
		}
		if call.WorkDir != "/path/to/workdir" {
			t.Errorf("call %d: expected workdir /path/to/workdir, got %q", i, call.WorkDir)
		}
	}

	// Verify specific flag settings
	cpuCall := mock.calls[0]
	if cpuCall.CPUProfile == "" || cpuCall.MemProfile != "" {
		t.Errorf("expected CPUProfile set and MemProfile empty for CPU pass, got %+v", cpuCall)
	}

	allocCall := mock.calls[2]
	if allocCall.MemProfile == "" || allocCall.CPUProfile != "" {
		t.Errorf("expected MemProfile set for allocs pass, got %+v", allocCall)
	}

	mutexCall := mock.calls[4]
	if mutexCall.MutexProfile == "" || mutexCall.MutexFraction != 1 {
		t.Errorf("expected MutexProfile set with MutexFraction=1, got %+v", mutexCall)
	}

	blockCall := mock.calls[6]
	if blockCall.BlockProfile == "" || blockCall.BlockRate != 1 {
		t.Errorf("expected BlockProfile set with BlockRate=1, got %+v", blockCall)
	}
}

func TestCollect_EmptyFileError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	mock := &mockExecutor{touchFile: true, emptyFile: true}

	opts := profile.CollectOptions{
		BinaryPath: "/path/to/bin.test",
		Benchmark:  "BenchmarkFast",
		Iterations: 100,
		Types:      []string{profile.TypeCPU},
		OutputDir:  tmpDir,
	}

	_, err := profile.Collect(context.Background(), mock, opts)
	if !errors.Is(err, profile.ErrProfileEmpty) {
		t.Fatalf("expected ErrProfileEmpty when file is 0 bytes, got: %v", err)
	}
}
