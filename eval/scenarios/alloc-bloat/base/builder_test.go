package builder

import "testing"

var testParts = []string{
	"Hello", " ", "world", "!", " ",
	"This", " ", "is", " ", "a", " ", "test", ".",
}

func BenchmarkBuild(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Build(testParts)
	}
}
