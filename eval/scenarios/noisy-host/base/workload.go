package workload

import (
	"math"
	"runtime"
	"sync"
	"time"
)

// Compute performs a CPU-intensive calculation.
func Compute(n int) float64 {
	result := 0.0
	for i := 0; i < n; i++ {
		result += math.Sqrt(float64(i))
	}
	return result
}

// NoiseGenerator simulates background CPU load to induce measurement noise.
type NoiseGenerator struct {
	stop chan struct{}
	wg   sync.WaitGroup
}

// NewNoiseGenerator creates a noise generator that will use all CPU cores.
func NewNoiseGenerator() *NoiseGenerator {
	return &NoiseGenerator{
		stop: make(chan struct{}),
	}
}

// Start begins generating CPU noise in the background.
func (n *NoiseGenerator) Start() {
	numCPU := runtime.NumCPU()
	for i := 0; i < numCPU; i++ {
		n.wg.Add(1)
		go n.worker()
	}
}

// Stop terminates all noise generation workers.
func (n *NoiseGenerator) Stop() {
	close(n.stop)
	n.wg.Wait()
}

func (n *NoiseGenerator) worker() {
	defer n.wg.Done()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-n.stop:
			return
		case <-ticker.C:
			// Burst of CPU work
			for i := 0; i < 10000; i++ {
				_ = math.Sqrt(float64(i))
			}
		}
	}
}
