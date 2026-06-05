package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// FileLock provides exclusive access to a store directory.
// See design §12:排他ロックファイル for double-open detection.
type FileLock struct {
	path string
	file *os.File
}

// LockStore acquires an exclusive lock on the store directory.
func LockStore(storePath string) (*FileLock, error) {
	lockPath := filepath.Join(storePath, "LOCK")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("store locked: another process has %s open", storePath)
		}
		return nil, fmt.Errorf("lock store: %w", err)
	}
	// Write PID for diagnostics.
	fmt.Fprintf(f, "%d\n", os.Getpid())
	return &FileLock{path: lockPath, file: f}, nil
}

// Unlock releases the store lock.
func (l *FileLock) Unlock() error {
	l.file.Close()
	return os.Remove(l.path)
}
