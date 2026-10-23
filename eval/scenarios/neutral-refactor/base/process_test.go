package refactor

import "testing"

var testData = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

func BenchmarkProcess(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Process(testData)
	}
}
