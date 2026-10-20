package pkgpanic_test

import "testing"

func BenchmarkPanicking(b *testing.B) {
	panic("intentional benchmark panic for testing")
}
