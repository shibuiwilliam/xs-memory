package storage

import (
	"testing"
)

func TestMemtable(t *testing.T) {
	mt := NewMemtable()

	id := newTestULID(t)
	mt.Put(id, []byte("hello world"))

	if mt.Len() != 1 {
		t.Errorf("Len = %d, want 1", mt.Len())
	}
	if mt.Size() != 11 {
		t.Errorf("Size = %d, want 11", mt.Size())
	}

	data, ok := mt.Get(id)
	if !ok || string(data) != "hello world" {
		t.Errorf("Get = %q, ok=%v", data, ok)
	}

	// Update.
	mt.Put(id, []byte("updated"))
	if mt.Len() != 1 {
		t.Errorf("Len after update = %d, want 1", mt.Len())
	}
	data, _ = mt.Get(id)
	if string(data) != "updated" {
		t.Errorf("after update = %q", data)
	}
}

func TestSegmentFlushAndRead(t *testing.T) {
	dir := t.TempDir()
	cache := NewBlockCache(1024 * 1024) // 1MB budget

	mt := NewMemtable()
	id1 := newTestULID(t)
	id2 := newTestULID(t)
	mt.Put(id1, []byte("content one"))
	mt.Put(id2, []byte("日本語コンテンツ"))

	sw := &SegmentWriter{}
	seg, err := sw.Flush(dir, mt.Entries())
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}

	// Read back via block cache.
	data1, err := seg.ReadBlock(id1, cache)
	if err != nil {
		t.Fatalf("ReadBlock id1: %v", err)
	}
	if string(data1) != "content one" {
		t.Errorf("ReadBlock id1 = %q", data1)
	}

	data2, err := seg.ReadBlock(id2, cache)
	if err != nil {
		t.Fatalf("ReadBlock id2: %v", err)
	}
	if string(data2) != "日本語コンテンツ" {
		t.Errorf("ReadBlock id2 = %q", data2)
	}

	// Second read should hit cache.
	data1again, err := seg.ReadBlock(id1, cache)
	if err != nil {
		t.Fatalf("ReadBlock cache hit: %v", err)
	}
	if string(data1again) != "content one" {
		t.Errorf("cache hit = %q", data1again)
	}
}

func TestSegmentOpenAndRead(t *testing.T) {
	dir := t.TempDir()
	cache := NewBlockCache(1024 * 1024)

	mt := NewMemtable()
	id := newTestULID(t)
	mt.Put(id, []byte("persisted data"))

	sw := &SegmentWriter{}
	seg, err := sw.Flush(dir, mt.Entries())
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}

	// Reopen from disk.
	seg2, err := OpenSegment(seg.path, seg.id)
	if err != nil {
		t.Fatalf("OpenSegment: %v", err)
	}

	data, err := seg2.ReadBlock(id, cache)
	if err != nil {
		t.Fatalf("ReadBlock: %v", err)
	}
	if string(data) != "persisted data" {
		t.Errorf("ReadBlock = %q", data)
	}
}

func TestBlockCacheEvictionWithSegment(t *testing.T) {
	dir := t.TempDir()
	// Tiny cache: 30 bytes. Blocks are ~11-24 bytes each.
	cache := NewBlockCache(30)

	mt := NewMemtable()
	id1 := newTestULID(t)
	id2 := newTestULID(t)
	id3 := newTestULID(t)
	mt.Put(id1, []byte("aaaaaaaaaa")) // 10 bytes
	mt.Put(id2, []byte("bbbbbbbbbb")) // 10 bytes
	mt.Put(id3, []byte("cccccccccc")) // 10 bytes

	sw := &SegmentWriter{}
	seg, err := sw.Flush(dir, mt.Entries())
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}

	// Read all three — cache should evict older ones.
	seg.ReadBlock(id1, cache)
	seg.ReadBlock(id2, cache)
	seg.ReadBlock(id3, cache)

	// Budget should not be exceeded.
	if cache.UsedBytes() > 30 {
		t.Errorf("cache exceeded budget: %d > 30", cache.UsedBytes())
	}
}
