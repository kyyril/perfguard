package refactor

// Process transforms input data.
func Process(data []int) int {
	result := 0
	for _, v := range data {
		result += v * 2
	}
	return result
}
