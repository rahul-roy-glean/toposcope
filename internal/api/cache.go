package api

import (
	"os"
	"strconv"
	"sync"

	"github.com/toposcope/toposcope/pkg/graph"
)

// SnapshotCache is a thread-safe LRU cache for loaded graph snapshots.
type SnapshotCache struct {
	mu      sync.Mutex
	maxSize int
	entries map[string]*cacheEntry
	order   []string // oldest first
}

type cacheEntry struct {
	snap *graph.Snapshot
}

// NewSnapshotCache creates a cache with the given maximum number of entries.
// If maxSize <= 0, it defaults to 20.
func NewSnapshotCache(maxSize int) *SnapshotCache {
	if maxSize <= 0 {
		maxSize = 20
	}
	return &SnapshotCache{
		maxSize: maxSize,
		entries: make(map[string]*cacheEntry),
	}
}

// NewSnapshotCacheFromEnv creates a cache with size from SNAPSHOT_CACHE_SIZE env var.
func NewSnapshotCacheFromEnv() *SnapshotCache {
	size := 20
	if v := os.Getenv("SNAPSHOT_CACHE_SIZE"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			size = parsed
		}
	}
	return NewSnapshotCache(size)
}

// Get retrieves a snapshot from the cache, or nil if not found.
func (c *SnapshotCache) Get(id string) *graph.Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[id]
	if !ok {
		return nil
	}

	// Move to end (most recently used)
	c.moveToEnd(id)
	return entry.snap
}

// Put adds a snapshot to the cache, evicting the oldest if full.
func (c *SnapshotCache) Put(id string, snap *graph.Snapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.entries[id]; ok {
		c.entries[id] = &cacheEntry{snap: snap}
		c.moveToEnd(id)
		return
	}

	// Evict oldest if at capacity
	for len(c.entries) >= c.maxSize && len(c.order) > 0 {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.entries, oldest)
	}

	c.entries[id] = &cacheEntry{snap: snap}
	c.order = append(c.order, id)
}

func (c *SnapshotCache) moveToEnd(id string) {
	for i, k := range c.order {
		if k == id {
			c.order = append(c.order[:i], c.order[i+1:]...)
			c.order = append(c.order, id)
			return
		}
	}
}
