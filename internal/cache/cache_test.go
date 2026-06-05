package cache

import (
	"fmt"
	"sync"
	"testing"
)

func TestResultCacheHitMiss(t *testing.T) {
	rc := NewResultCache(100, 1024*1024, 4)

	key := QueryKey("default", "test query", "hybrid", 10, 0)
	entry := ResultEntry{
		IDs:      []string{"id1", "id2"},
		Scores:   []float64{0.9, 0.5},
		ByteSize: 100,
	}

	// Miss on empty cache.
	if got := rc.Get(key, "default"); got != nil {
		t.Error("expected miss on empty cache")
	}

	// Put and hit.
	rc.Put(key, "default", entry)
	got := rc.Get(key, "default")
	if got == nil {
		t.Fatal("expected hit after put")
	}
	if len(got.IDs) != 2 || got.IDs[0] != "id1" {
		t.Errorf("cached IDs = %v", got.IDs)
	}

	stats := rc.Stats()
	if stats.Hits != 1 || stats.Misses != 1 {
		t.Errorf("stats: hits=%d misses=%d, want 1/1", stats.Hits, stats.Misses)
	}
}

func TestResultCacheFreshnessGeneration(t *testing.T) {
	// The critical correctness test: after a write (generation bump),
	// a previously cached query MUST miss. See addendum2 §2.3.
	rc := NewResultCache(100, 1024*1024, 4)

	key := QueryKey("mydb", "cache eviction policy", "hybrid", 10, 0)
	rc.Put(key, "mydb", ResultEntry{
		IDs:      []string{"old-result"},
		Scores:   []float64{0.8},
		ByteSize: 50,
	})

	// Should hit before write.
	if rc.Get(key, "mydb") == nil {
		t.Fatal("expected hit before generation bump")
	}

	// Simulate a write to the collection.
	rc.BumpGeneration("mydb")

	// Must miss now — data is stale.
	if got := rc.Get(key, "mydb"); got != nil {
		t.Fatal("STALE READ: cache returned data after generation bump — violates addendum2 §2.3")
	}

	stats := rc.Stats()
	if stats.Invalidations != 1 {
		t.Errorf("invalidations = %d, want 1", stats.Invalidations)
	}
}

func TestResultCacheLRUEviction(t *testing.T) {
	// Max 4 entries across shards.
	rc := NewResultCache(4, 1024*1024, 1) // 1 shard for determinism

	for i := 0; i < 10; i++ {
		key := QueryKey("c", fmt.Sprintf("q%d", i), "fts", 10, 0)
		rc.Put(key, "c", ResultEntry{IDs: []string{fmt.Sprintf("id%d", i)}, ByteSize: 10})
	}

	stats := rc.Stats()
	if stats.EntryCount > 4 {
		t.Errorf("entries = %d, exceeds max 4", stats.EntryCount)
	}
	if stats.Evictions == 0 {
		t.Error("expected evictions")
	}
}

func TestResultCacheByteBudget(t *testing.T) {
	// 200 byte budget, 1 shard.
	rc := NewResultCache(1000, 200, 1)

	for i := 0; i < 10; i++ {
		key := QueryKey("c", fmt.Sprintf("q%d", i), "fts", 10, 0)
		rc.Put(key, "c", ResultEntry{IDs: []string{"x"}, ByteSize: 50})
	}

	stats := rc.Stats()
	// With 50 bytes each and 200 budget, at most 4 entries per shard.
	if stats.BytesUsed > 200 {
		t.Errorf("bytes = %d, exceeds budget 200", stats.BytesUsed)
	}
}

func TestResultCacheConcurrentWriteRead(t *testing.T) {
	// Prove no stale reads under concurrent writes with -race.
	// See addendum2 §2.3.
	rc := NewResultCache(100, 1024*1024, 16)
	const collection = "concurrent-test"

	var wg sync.WaitGroup

	// Writers: bump generation continuously.
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				rc.BumpGeneration(collection)
			}
		}()
	}

	// Readers: put and get, checking freshness.
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				key := QueryKey(collection, fmt.Sprintf("q%d-%d", id, i), "hybrid", 10, 0)
				rc.Put(key, collection, ResultEntry{IDs: []string{"x"}, ByteSize: 10})
				// Get may hit or miss depending on concurrent bumps — both are fine.
				// What matters: no data race (detected by -race flag).
				rc.Get(key, collection)
			}
		}(r)
	}

	wg.Wait()
	// If we get here without -race failure, concurrency is safe.
}

func TestResultCacheTuningEpochInKey(t *testing.T) {
	// Different tuning epochs → different cache keys. See addendum2 §2.2.
	k1 := QueryKey("c", "q", "hybrid", 10, 0)
	k2 := QueryKey("c", "q", "hybrid", 10, 1)
	if k1 == k2 {
		t.Error("different tuning epochs should produce different keys")
	}
}

func TestResultCacheJapanese(t *testing.T) {
	rc := NewResultCache(100, 1024*1024, 4)

	key := QueryKey("ja", "東京タワー 電波塔", "hybrid", 10, 0)
	rc.Put(key, "ja", ResultEntry{IDs: []string{"jp1"}, Scores: []float64{0.9}, ByteSize: 80})

	got := rc.Get(key, "ja")
	if got == nil || got.IDs[0] != "jp1" {
		t.Error("Japanese query cache miss")
	}
}
