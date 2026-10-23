package search

import "testing"

var benchData = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}

func BenchmarkSearch(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Contains(benchData, 15)
	}
}
