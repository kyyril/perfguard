package pkgbroken_test

import "testing"

// Syntax error to guarantee compilation failure.
func BenchmarkBroken(b *testing.B) {
	thisDoesNotCompile := undefinedVariable + 123
}
