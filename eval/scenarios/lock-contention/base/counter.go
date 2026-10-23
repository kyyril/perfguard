package counter

import "sync"

// Counter tracks counts with per-shard mutexes to reduce contention.
type Counter struct {
	shards [8]shard
}

type shard struct {
	mu    sync.Mutex
	count int64
}

// New creates a new Counter.
func New() *Counter {
	return &Counter{}
}

// Inc increments the counter.
func (c *Counter) Inc(key int) {
	idx := key % len(c.shards)
	c.shards[idx].mu.Lock()
	c.shards[idx].count++
	c.shards[idx].mu.Unlock()
}

// Total returns the sum across all shards.
func (c *Counter) Total() int64 {
	var total int64
	for i := range c.shards {
		c.shards[i].mu.Lock()
		total += c.shards[i].count
		c.shards[i].mu.Unlock()
	}
	return total
}
