package evaltest

import "time"

// Process simulates work with a sleep.
func Process(items int) {
	for i := 0; i < items; i++ {
		time.Sleep(1 * time.Microsecond)
	}
}
