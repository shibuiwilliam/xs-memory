package storage

import (
	"fmt"
	"testing"
)

func BenchmarkBlockCachePut(b *testing.B) {
	c := NewBlockCache(256 * 1024 * 1024) // 256MB
	data := make([]byte, 4096)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Put(fmt.Sprintf("key%d", i), data)
	}
}

func BenchmarkBlockCacheGet(b *testing.B) {
	c := NewBlockCache(256 * 1024 * 1024)
	data := make([]byte, 4096)
	for i := 0; i < 10000; i++ {
		c.Put(fmt.Sprintf("key%d", i), data)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(fmt.Sprintf("key%d", i%10000))
	}
}

func BenchmarkBlockCacheEviction(b *testing.B) {
	c := NewBlockCache(1024 * 1024) // 1MB — forces frequent eviction
	data := make([]byte, 4096)      // 4KB blocks

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Put(fmt.Sprintf("key%d", i), data)
	}
}
