package storage

import (
	"fmt"
	"sync"
	"testing"
)

func TestBlockCachePutGet(t *testing.T) {
	c := NewBlockCache(1024)
	c.Put("key1", []byte("hello"))
	c.Put("key2", []byte("world"))

	if got := c.Get("key1"); string(got) != "hello" {
		t.Errorf("Get key1 = %q, want hello", got)
	}
	if got := c.Get("key2"); string(got) != "world" {
		t.Errorf("Get key2 = %q, want world", got)
	}
	if got := c.Get("missing"); got != nil {
		t.Errorf("Get missing = %q, want nil", got)
	}
}

func TestBlockCacheLRUEviction(t *testing.T) {
	// Cache with 100 bytes budget. See design §6.3.
	c := NewBlockCache(100)

	// Insert 60 bytes.
	c.Put("a", make([]byte, 60))
	if c.Len() != 1 || c.UsedBytes() != 60 {
		t.Fatalf("after a: len=%d used=%d", c.Len(), c.UsedBytes())
	}

	// Insert 60 more — should evict "a" (LRU).
	c.Put("b", make([]byte, 60))
	if c.Len() != 1 {
		t.Fatalf("after b: len=%d, want 1 (evicted a)", c.Len())
	}
	if c.Get("a") != nil {
		t.Error("a should have been evicted")
	}
	if c.Get("b") == nil {
		t.Error("b should be present")
	}
}

func TestBlockCacheMemoryBudget(t *testing.T) {
	// Verify the cache never exceeds its budget. N2 enforcement.
	budget := 256
	c := NewBlockCache(budget)

	for i := 0; i < 100; i++ {
		c.Put(fmt.Sprintf("k%d", i), make([]byte, 10))
		if c.UsedBytes() > budget {
			t.Fatalf("budget exceeded: used=%d, budget=%d", c.UsedBytes(), budget)
		}
	}
}

func TestBlockCacheConcurrent(t *testing.T) {
	c := NewBlockCache(10000)
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("k%d", i)
			c.Put(key, []byte(fmt.Sprintf("val%d", i)))
			c.Get(key)
		}(i)
	}
	wg.Wait()
}

func TestBlockCacheUpdate(t *testing.T) {
	c := NewBlockCache(1024)
	c.Put("key", []byte("old"))
	c.Put("key", []byte("new_value"))

	if got := string(c.Get("key")); got != "new_value" {
		t.Errorf("Get after update = %q, want new_value", got)
	}
	if c.Len() != 1 {
		t.Errorf("Len = %d, want 1", c.Len())
	}
}
