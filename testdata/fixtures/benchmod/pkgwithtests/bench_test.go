package pkgwithtests_test

import (
	"testing"

	"example.com/benchmod/pkgwithtests"
)

func BenchmarkFast(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = pkgwithtests.DoWork()
	}
}

func BenchmarkMixed(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = pkgwithtests.DoWork()
		_ = make([]byte, 1024)
	}
}
