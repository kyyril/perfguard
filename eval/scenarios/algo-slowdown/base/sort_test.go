package sorting

import (
	"math/rand"
	"testing"
)

func BenchmarkSort(b *testing.B) {
	data := make([]int, 100)
	for i := range data {
		data[i] = rand.Intn(1000)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		temp := make([]int, len(data))
		copy(temp, data)
		b.StartTimer()

		Sort(temp)
	}
}
