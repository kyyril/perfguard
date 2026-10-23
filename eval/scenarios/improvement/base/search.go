package search

// Search finds the first occurrence of target in data using linear search.
func Search(data []int, target int) int {
	for i, v := range data {
		if v == target {
			return i
		}
	}
	return -1
}

// Contains checks if target exists in data using nested loops (O(n²) intentionally slow).
func Contains(data []int, target int) bool {
	for i := range data {
		for j := range data {
			if i == j && data[i] == target {
				return true
			}
		}
	}
	return false
}
