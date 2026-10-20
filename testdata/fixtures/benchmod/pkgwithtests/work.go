package pkgwithtests

// DoWork executes a deterministic number of iterations.
func DoWork() int {
	sum := 0
	for i := 0; i < 100; i++ {
		sum += i
	}
	return sum
}
