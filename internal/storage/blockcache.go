package storage

import (
	"container/list"
	"sync"
)

// BlockCache is a memory-budgeted LRU cache for segment data blocks.
// This is the core mechanism for N2 (explicit memory budget control).
// See design §6.3.
type BlockCache struct {
	mu       sync.Mutex
	capacity int // max bytes
	used     int
	items    map[string]*list.Element
	order    *list.List // front = most recent
}

type cacheEntry struct {
	key  string
	data []byte
}

// NewBlockCache creates a cache with the given byte capacity.
// See design §6.3 and N2.
func NewBlockCache(capacityBytes int) *BlockCache {
	return &BlockCache{
		capacity: capacityBytes,
		items:    make(map[string]*list.Element),
		order:    list.New(),
	}
}

// Get retrieves a block from cache. Returns nil on miss.
func (c *BlockCache) Get(key string) []byte {
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[key]; ok {
		c.order.MoveToFront(el)
		return el.Value.(*cacheEntry).data
	}
	return nil
}

// Put inserts a block. Evicts LRU entries if budget would be exceeded.
func (c *BlockCache) Put(key string, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If key already exists, update in place.
	if el, ok := c.items[key]; ok {
		old := el.Value.(*cacheEntry)
		c.used -= len(old.data)
		old.data = data
		c.used += len(data)
		c.order.MoveToFront(el)
		return
	}

	// Evict until there's room.
	for c.used+len(data) > c.capacity && c.order.Len() > 0 {
		c.evict()
	}

	entry := &cacheEntry{key: key, data: data}
	el := c.order.PushFront(entry)
	c.items[key] = el
	c.used += len(data)
}

// evict removes the least recently used entry. Must hold mu.
func (c *BlockCache) evict() {
	tail := c.order.Back()
	if tail == nil {
		return
	}
	entry := tail.Value.(*cacheEntry)
	c.order.Remove(tail)
	delete(c.items, entry.key)
	c.used -= len(entry.data)
}

// Len returns the number of cached blocks.
func (c *BlockCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

// UsedBytes returns current memory usage.
func (c *BlockCache) UsedBytes() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.used
}

// Stats returns cache statistics.
type CacheStats struct {
	Capacity int
	Used     int
	Count    int
}

func (c *BlockCache) Stats() CacheStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return CacheStats{
		Capacity: c.capacity,
		Used:     c.used,
		Count:    len(c.items),
	}
}
