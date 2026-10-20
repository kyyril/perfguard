package benchparse_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/kyyril/perfguard/internal/benchparse"
)

func FuzzParse(f *testing.F) {
	// Seed with existing fixtures.
	for _, path := range []string{
		"../../testdata/benchparse/standard.txt",
		"../../testdata/benchparse/custom_metrics.txt",
		"../../testdata/benchparse/truncated.txt",
		"../../testdata/benchparse/empty.txt",
	} {
		data, err := os.ReadFile(path)
		if err == nil {
			f.Add(data)
		}
	}

	// Seed with synthetic inputs.
	f.Add([]byte("BenchmarkSimple-8 100 1000 ns/op\n"))
	f.Add([]byte("BenchmarkInvalid 100\n"))
	f.Add([]byte("\x00\xff\xfe\x01\x02BenchmarkBroken"))
	f.Add([]byte(""))

	f.Fuzz(func(_ *testing.T, data []byte) {
		// Parse must never panic, regardless of input fuzz data.
		_, _ = benchparse.Parse(bytes.NewReader(data), "fuzz/pkg")
	})
}
