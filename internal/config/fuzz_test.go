package config_test

import (
	"testing"

	"github.com/kyyril/perfguard/internal/config"
)

func FuzzParse(f *testing.F) {
	seeds := [][]byte{
		[]byte("version: 1\n"),
		[]byte("version: 1\nbenchmarks:\n  count: 10\n"),
		[]byte("version: 2\n"),
		[]byte("invalid: yaml: :"),
		[]byte(""),
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(_ *testing.T, data []byte) {
		// Ensure parser never panics on arbitrary inputs.
		_, _ = config.LoadBytes(data)
	})
}
