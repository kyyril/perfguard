package sorting

// Sort sorts the slice in ascending order using quicksort.
func Sort(data []int) {
	if len(data) <= 1 {
		return
	}
	quickSort(data, 0, len(data)-1)
}

func quickSort(data []int, low, high int) {
	if low < high {
		pi := partition(data, low, high)
		quickSort(data, low, pi-1)
		quickSort(data, pi+1, high)
	}
}

func partition(data []int, low, high int) int {
	pivot := data[high]
	i := low - 1
	for j := low; j < high; j++ {
		if data[j] < pivot {
			i++
			data[i], data[j] = data[j], data[i]
		}
	}
	data[i+1], data[high] = data[high], data[i+1]
	return i + 1
}
