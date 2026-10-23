package hashtest

import "testing"

var testInput = []byte("the quick brown fox jumps over the lazy dog")

func BenchmarkHash(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Hash(testInput)
	}
}
