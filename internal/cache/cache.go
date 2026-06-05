// Package cache provides a two-tier in-memory cache: result cache and hot-item cache.
// The result cache stores assembled search results keyed by query signature.
// The hot-item cache stores hydrated memory records for the most-used items.
// See addendum2 §2.
package cache

import (
	"container/list"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sync"
	"sync/atomic"
)

// ResultEntry is a cached search result set. See addendum2 §2.2.
type ResultEntry struct {
	IDs        []string  // ordered memory IDs
	Scores     []float64 // corresponding scores
	Generation uint64    // collection generation when computed
	ByteSize   int       // approximate size for budget tracking
}

// HotItem is a cached hydrated memory record. See addendum2 §2.2.
type HotItem struct {
	ID       string
	Content  string
	Metadata map[string]any
	ByteSize int
}

// Stats tracks cache metrics. See addendum2 §2.4.
type Stats struct {
	Hits          uint64 `json:"hits"`
	Misses        uint64 `json:"misses"`
	Evictions     uint64 `json:"evictions"`
	Invalidations uint64 `json:"invalidations"`
	EntryCount    int    `json:"entry_count"`
	BytesUsed     int    `json:"bytes_used"`
	ByteBudget    int    `json:"byte_budget"`
}

// ResultCache caches assembled search results keyed by query signature.
// Uses sharded LRU with per-collection generation counters for freshness.
// See addendum2 §2.1, §2.3.
type ResultCache struct {
	shards     []*resultShard
	shardMask  uint32
	maxTotal   int // max entries across all shards
	byteBudget int

	// Per-collection generation counters. Bumped on any write.
	// See addendum2 §2.3.
	generations sync.Map // collection string → *uint64

	// Metrics.
	hits          atomic.Uint64
	misses        atomic.Uint64
	evictions     atomic.Uint64
	invalidations atomic.Uint64
}

type resultShard struct {
	mu    sync.Mutex
	items map[string]*list.Element
	order *list.List
	bytes int
}

type resultCacheEntry struct {
	key   string
	value ResultEntry
}

// NewResultCache creates a result cache. See addendum2 §2.4.
func NewResultCache(maxEntries, byteBudget, shards int) *ResultCache {
	if shards <= 0 {
		shards = 16
	}
	if maxEntries <= 0 {
		maxEntries = 100
	}
	// Round shards to power of 2.
	n := uint32(1)
	for n < uint32(shards) {
		n <<= 1
	}

	rc := &ResultCache{
		shards:     make([]*resultShard, n),
		shardMask:  n - 1,
		maxTotal:   maxEntries,
		byteBudget: byteBudget,
	}
	for i := range rc.shards {
		rc.shards[i] = &resultShard{
			items: make(map[string]*list.Element),
			order: list.New(),
		}
	}
	return rc
}

// QueryKey computes a cache key from query parameters. See addendum2 §2.2.
// Includes tuning_epoch so a tuning reset invalidates ranking-dependent entries.
func QueryKey(collection, normalizedQuery, mode string, topK int, tuningEpoch uint64) string {
	h := sha256.New()
	h.Write([]byte(collection))
	h.Write([]byte{0})
	h.Write([]byte(normalizedQuery))
	h.Write([]byte{0})
	h.Write([]byte(mode))
	var buf [16]byte
	binary.LittleEndian.PutUint64(buf[:8], uint64(topK))
	binary.LittleEndian.PutUint64(buf[8:], tuningEpoch)
	h.Write(buf[:])
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (rc *ResultCache) getShard(key string) *resultShard {
	// FNV-1a-like hash for shard selection.
	var h uint32 = 2166136261
	for i := 0; i < len(key); i++ {
		h ^= uint32(key[i])
		h *= 16777619
	}
	return rc.shards[h&rc.shardMask]
}

// Get looks up a cached result. Returns nil on miss or stale generation.
// See addendum2 §2.3.
func (rc *ResultCache) Get(key, collection string) *ResultEntry {
	shard := rc.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	el, ok := shard.items[key]
	if !ok {
		rc.misses.Add(1)
		return nil
	}

	entry := el.Value.(*resultCacheEntry)

	// Check generation for freshness. See addendum2 §2.3.
	currentGen := rc.GetGeneration(collection)
	if entry.value.Generation != currentGen {
		// Stale — logically invalidated. Remove it.
		shard.order.Remove(el)
		delete(shard.items, key)
		shard.bytes -= entry.value.ByteSize
		rc.invalidations.Add(1)
		rc.misses.Add(1)
		return nil
	}

	shard.order.MoveToFront(el)
	rc.hits.Add(1)
	return &entry.value
}

// Put stores a result set. See addendum2 §2.3.
func (rc *ResultCache) Put(key, collection string, entry ResultEntry) {
	entry.Generation = rc.GetGeneration(collection)

	shard := rc.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	// Update if exists.
	if el, ok := shard.items[key]; ok {
		old := el.Value.(*resultCacheEntry)
		shard.bytes -= old.value.ByteSize
		old.value = entry
		shard.bytes += entry.ByteSize
		shard.order.MoveToFront(el)
		return
	}

	// Evict if at capacity.
	maxPerShard := rc.maxTotal / len(rc.shards)
	if maxPerShard < 1 {
		maxPerShard = 1
	}
	for shard.order.Len() >= maxPerShard || (rc.byteBudget > 0 && shard.bytes+entry.ByteSize > rc.byteBudget/len(rc.shards)) {
		if shard.order.Len() == 0 {
			break
		}
		rc.evictLocked(shard)
	}

	ce := &resultCacheEntry{key: key, value: entry}
	el := shard.order.PushFront(ce)
	shard.items[key] = el
	shard.bytes += entry.ByteSize
}

func (rc *ResultCache) evictLocked(shard *resultShard) {
	tail := shard.order.Back()
	if tail == nil {
		return
	}
	entry := tail.Value.(*resultCacheEntry)
	shard.order.Remove(tail)
	delete(shard.items, entry.key)
	shard.bytes -= entry.value.ByteSize
	rc.evictions.Add(1)
}

// BumpGeneration increments the generation counter for a collection.
// Called on any write that changes the collection's data. See addendum2 §2.3.
func (rc *ResultCache) BumpGeneration(collection string) {
	val, _ := rc.generations.LoadOrStore(collection, new(uint64))
	atomic.AddUint64(val.(*uint64), 1)
}

// GetGeneration returns the current generation for a collection.
func (rc *ResultCache) GetGeneration(collection string) uint64 {
	val, ok := rc.generations.Load(collection)
	if !ok {
		return 0
	}
	return atomic.LoadUint64(val.(*uint64))
}

// Stats returns cache metrics. See addendum2 §2.4.
func (rc *ResultCache) Stats() Stats {
	var entryCount, bytesUsed int
	for _, shard := range rc.shards {
		shard.mu.Lock()
		entryCount += shard.order.Len()
		bytesUsed += shard.bytes
		shard.mu.Unlock()
	}
	return Stats{
		Hits:          rc.hits.Load(),
		Misses:        rc.misses.Load(),
		Evictions:     rc.evictions.Load(),
		Invalidations: rc.invalidations.Load(),
		EntryCount:    entryCount,
		BytesUsed:     bytesUsed,
		ByteBudget:    rc.byteBudget,
	}
}
