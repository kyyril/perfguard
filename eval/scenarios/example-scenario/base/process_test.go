package evaltest

import "testing"

func BenchmarkProcess(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Process(10)
	}
}
