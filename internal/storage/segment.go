package storage

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
	"sync"
	"sync/atomic"

	"github.com/oklog/ulid/v2"
)

// Segment file format (simplified for MVP, see design §6.2):
//   Header: [4 magic] [4 version] [4 block_count] [4 reserved]
//   Block index: [block_count × (16 bytes ULID key + 4 offset + 4 length)]
//   Blocks: [payload bytes...]
//   Footer: [4 CRC of entire file up to footer]

var segmentMagic = [4]byte{'S', 'M', 'E', 'M'}

// Segment represents an immutable index segment on disk. See design §6.2.
type Segment struct {
	path  string
	id    uint64
	index []blockRef // in-memory block index (small, always resident per §6.3)
	mu    sync.RWMutex
}

type blockRef struct {
	key    ulid.ULID
	offset uint32
	length uint32
}

// Memtable is the mutable in-memory segment. When it exceeds the threshold,
// it is flushed to an immutable Segment. See design §6.2.
type Memtable struct {
	mu      sync.RWMutex
	entries map[ulid.ULID][]byte // memory ID -> serialized data
	size    int                  // approximate byte size
}

// NewMemtable creates a new empty memtable.
func NewMemtable() *Memtable {
	return &Memtable{
		entries: make(map[ulid.ULID][]byte),
	}
}

// Put adds or updates an entry.
func (mt *Memtable) Put(id ulid.ULID, data []byte) {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	if old, ok := mt.entries[id]; ok {
		mt.size -= len(old)
	}
	mt.entries[id] = data
	mt.size += len(data)
}

// Get retrieves an entry.
func (mt *Memtable) Get(id ulid.ULID) ([]byte, bool) {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	data, ok := mt.entries[id]
	return data, ok
}

// Size returns the approximate byte size.
func (mt *Memtable) Size() int {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	return mt.size
}

// Len returns entry count.
func (mt *Memtable) Len() int {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	return len(mt.entries)
}

// Entries returns a snapshot of all entries (for flushing).
func (mt *Memtable) Entries() map[ulid.ULID][]byte {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	cp := make(map[ulid.ULID][]byte, len(mt.entries))
	for k, v := range mt.entries {
		cp[k] = v
	}
	return cp
}

// SegmentWriter writes a memtable snapshot to an immutable segment file.
// See design §6.2 step 3.
type SegmentWriter struct {
	nextID atomic.Uint64
}

// Flush writes entries to a new segment file and returns the segment.
func (sw *SegmentWriter) Flush(dir string, entries map[ulid.ULID][]byte) (*Segment, error) {
	segID := sw.nextID.Add(1)
	path := fmt.Sprintf("%s/%05d.seg", dir, segID)

	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("segment: create: %w", err)
	}
	defer f.Close()

	// Build sorted key list for deterministic output.
	keys := make([]ulid.ULID, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}

	blockCount := uint32(len(keys))

	// Write header.
	header := make([]byte, 16)
	copy(header[0:4], segmentMagic[:])
	binary.LittleEndian.PutUint32(header[4:8], 1) // version
	binary.LittleEndian.PutUint32(header[8:12], blockCount)
	if _, err := f.Write(header); err != nil {
		return nil, fmt.Errorf("segment: write header: %w", err)
	}

	// Calculate offsets. Data starts after header + index.
	indexSize := blockCount * 24 // 16 (ULID) + 4 (offset) + 4 (length)
	dataStart := uint32(16 + indexSize)

	// Build block index and concatenate data.
	refs := make([]blockRef, len(keys))
	var dataBlob []byte
	for i, k := range keys {
		data := entries[k]
		refs[i] = blockRef{
			key:    k,
			offset: dataStart + uint32(len(dataBlob)),
			length: uint32(len(data)),
		}
		dataBlob = append(dataBlob, data...)
	}

	// Write index.
	for _, ref := range refs {
		idx := make([]byte, 24)
		copy(idx[0:16], ref.key[:])
		binary.LittleEndian.PutUint32(idx[16:20], ref.offset)
		binary.LittleEndian.PutUint32(idx[20:24], ref.length)
		if _, err := f.Write(idx); err != nil {
			return nil, fmt.Errorf("segment: write index: %w", err)
		}
	}

	// Write data blocks.
	if _, err := f.Write(dataBlob); err != nil {
		return nil, fmt.Errorf("segment: write data: %w", err)
	}

	// Write CRC footer.
	// Re-read entire content for CRC (for simplicity in MVP).
	if _, err := f.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("segment: seek: %w", err)
	}
	allData := make([]byte, int(dataStart)+len(dataBlob))
	if _, err := f.Read(allData); err != nil {
		return nil, fmt.Errorf("segment: read for crc: %w", err)
	}
	checksum := crc32.Checksum(allData, crc32cTable)
	crcBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(crcBuf, checksum)
	if _, err := f.Write(crcBuf); err != nil {
		return nil, fmt.Errorf("segment: write crc: %w", err)
	}

	return &Segment{
		path:  path,
		id:    segID,
		index: refs,
	}, nil
}

// ReadBlock reads a block from a segment file using the block cache.
// See design §6.3: cache hit → return, miss → pread → insert(LRU evict) → return.
func (s *Segment) ReadBlock(id ulid.ULID, cache *BlockCache) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Find in index.
	var ref *blockRef
	for i := range s.index {
		if s.index[i].key == id {
			ref = &s.index[i]
			break
		}
	}
	if ref == nil {
		return nil, fmt.Errorf("segment: block %s not found", id)
	}

	cacheKey := fmt.Sprintf("seg:%d:blk:%s", s.id, id)

	// Check cache first.
	if data := cache.Get(cacheKey); data != nil {
		return data, nil
	}

	// Cache miss: read from disk.
	f, err := os.Open(s.path)
	if err != nil {
		return nil, fmt.Errorf("segment: open for read: %w", err)
	}
	defer f.Close()

	data := make([]byte, ref.length)
	if _, err := f.ReadAt(data, int64(ref.offset)); err != nil {
		return nil, fmt.Errorf("segment: read block: %w", err)
	}

	cache.Put(cacheKey, data)
	return data, nil
}

// OpenSegment reads a segment file's header and block index into memory.
// Only the index is resident; data blocks are loaded on demand. See design §6.3.
func OpenSegment(path string, id uint64) (*Segment, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("segment: open: %w", err)
	}
	defer f.Close()

	// Read header.
	header := make([]byte, 16)
	if _, err := f.Read(header); err != nil {
		return nil, fmt.Errorf("segment: read header: %w", err)
	}
	if string(header[0:4]) != string(segmentMagic[:]) {
		return nil, fmt.Errorf("segment: bad magic")
	}
	blockCount := binary.LittleEndian.Uint32(header[8:12])

	// Read index.
	refs := make([]blockRef, blockCount)
	for i := uint32(0); i < blockCount; i++ {
		idx := make([]byte, 24)
		if _, err := f.Read(idx); err != nil {
			return nil, fmt.Errorf("segment: read index: %w", err)
		}
		copy(refs[i].key[:], idx[0:16])
		refs[i].offset = binary.LittleEndian.Uint32(idx[16:20])
		refs[i].length = binary.LittleEndian.Uint32(idx[20:24])
	}

	return &Segment{
		path:  path,
		id:    id,
		index: refs,
	}, nil
}
