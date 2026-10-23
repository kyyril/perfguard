package nochange

// Compute performs a deterministic calculation.
func Compute(n int) int {
	sum := 0
	for i := 0; i < n; i++ {
		sum += i * i
	}
	return sum
}
