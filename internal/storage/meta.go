package storage

import (
	"encoding/json"
	"fmt"

	"github.com/oklog/ulid/v2"
	bolt "go.etcd.io/bbolt"
)

// Bucket names in bbolt. See design §6.1.
var (
	bucketMemories    = []byte("memories")
	bucketCollections = []byte("collections")
	bucketTombstones  = []byte("tombstones")
)

// MetaStore wraps bbolt for Memory metadata, collections, and tombstones.
// See design §6.1: meta.db holds Memory meta, Collection, Triple indexes, tombstones.
type MetaStore struct {
	db *bolt.DB
}

// OpenMeta opens or creates the bbolt metadata database.
func OpenMeta(path string) (*MetaStore, error) {
	db, err := bolt.Open(path, 0o644, &bolt.Options{
		NoFreelistSync: true,
	})
	if err != nil {
		return nil, fmt.Errorf("meta: open bbolt: %w", err)
	}

	// Ensure buckets exist.
	err = db.Update(func(tx *bolt.Tx) error {
		for _, b := range [][]byte{bucketMemories, bucketCollections, bucketTombstones} {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return fmt.Errorf("create bucket %s: %w", b, err)
			}
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("meta: init buckets: %w", err)
	}

	return &MetaStore{db: db}, nil
}

// PutMemory stores a Memory's metadata. See design §6.2.
func (m *MetaStore) PutMemory(mem *Memory) error {
	data, err := json.Marshal(mem)
	if err != nil {
		return fmt.Errorf("meta: marshal memory: %w", err)
	}

	return m.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketMemories)
		return b.Put(mem.ID[:], data)
	})
}

// GetMemory retrieves a Memory by ID.
func (m *MetaStore) GetMemory(id ulid.ULID) (*Memory, error) {
	var mem Memory
	err := m.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketMemories)
		data := b.Get(id[:])
		if data == nil {
			return fmt.Errorf("memory %s not found", id)
		}
		return json.Unmarshal(data, &mem)
	})
	if err != nil {
		return nil, fmt.Errorf("meta: get memory: %w", err)
	}
	return &mem, nil
}

// DeleteMemory marks a memory as tombstoned (soft delete). See design §6.4, N7.
func (m *MetaStore) DeleteMemory(id ulid.ULID, hard bool) error {
	return m.db.Update(func(tx *bolt.Tx) error {
		memBucket := tx.Bucket(bucketMemories)
		if hard {
			// Physical delete — only with explicit flag.
			if err := memBucket.Delete(id[:]); err != nil {
				return err
			}
			return tx.Bucket(bucketTombstones).Delete(id[:])
		}
		// Soft delete: mark tombstone and set Deleted flag.
		data := memBucket.Get(id[:])
		if data == nil {
			return fmt.Errorf("memory %s not found", id)
		}
		var mem Memory
		if err := json.Unmarshal(data, &mem); err != nil {
			return fmt.Errorf("unmarshal: %w", err)
		}
		mem.Deleted = true
		updated, err := json.Marshal(&mem)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		if err := memBucket.Put(id[:], updated); err != nil {
			return err
		}
		// Also record in tombstones bucket for fast filtering.
		return tx.Bucket(bucketTombstones).Put(id[:], []byte{1})
	})
}

// IsTombstoned checks if a memory ID is soft-deleted.
func (m *MetaStore) IsTombstoned(id ulid.ULID) bool {
	var found bool
	m.db.View(func(tx *bolt.Tx) error {
		found = tx.Bucket(bucketTombstones).Get(id[:]) != nil
		return nil
	})
	return found
}

// ListMemories returns all non-deleted memories in a collection.
func (m *MetaStore) ListMemories(collection string) ([]*Memory, error) {
	var result []*Memory
	err := m.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketMemories)
		ts := tx.Bucket(bucketTombstones)
		return b.ForEach(func(k, v []byte) error {
			// Skip tombstoned.
			if ts.Get(k) != nil {
				return nil
			}
			var mem Memory
			if err := json.Unmarshal(v, &mem); err != nil {
				return err
			}
			if collection == "" || mem.Collection == collection {
				result = append(result, &mem)
			}
			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("meta: list memories: %w", err)
	}
	return result, nil
}

// PutCollection stores a collection config.
func (m *MetaStore) PutCollection(cfg CollectionConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("meta: marshal collection: %w", err)
	}
	return m.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketCollections).Put([]byte(cfg.Name), data)
	})
}

// GetCollection retrieves a collection config by name.
func (m *MetaStore) GetCollection(name string) (*CollectionConfig, error) {
	var cfg CollectionConfig
	err := m.db.View(func(tx *bolt.Tx) error {
		data := tx.Bucket(bucketCollections).Get([]byte(name))
		if data == nil {
			return fmt.Errorf("collection %q not found", name)
		}
		return json.Unmarshal(data, &cfg)
	})
	if err != nil {
		return nil, fmt.Errorf("meta: get collection: %w", err)
	}
	return &cfg, nil
}

// DB returns the underlying bbolt database (for graph store sharing).
func (m *MetaStore) DB() *bolt.DB {
	return m.db
}

// Close closes the bbolt database.
func (m *MetaStore) Close() error {
	return m.db.Close()
}
