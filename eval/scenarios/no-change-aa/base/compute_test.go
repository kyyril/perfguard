package nochange

import "testing"

func BenchmarkCompute(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Compute(100)
	}
}
