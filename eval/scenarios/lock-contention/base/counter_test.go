package counter

import "testing"

func BenchmarkCounter(b *testing.B) {
	c := New()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Inc(i)
			i++
		}
	})
}
