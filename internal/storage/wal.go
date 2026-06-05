package storage

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// WAL record format (per design §6.2, §6.5):
//   [4 bytes] payload length (little-endian uint32)
//   [N bytes] payload (JSON-encoded operation)
//   [4 bytes] CRC32C of payload

var crc32cTable = crc32.MakeTable(crc32.Castagnoli)

// WALOp identifies the operation type in a WAL record.
type WALOp byte

const (
	WALOpPut    WALOp = 1
	WALOpDelete WALOp = 2
)

// WALRecord is a single entry in the write-ahead log. See design §6.2.
type WALRecord struct {
	Op       WALOp
	MemoryID [16]byte // ULID bytes
	Data     []byte   // JSON-encoded Memory for Put, nil for Delete
}

// WAL is the write-ahead log. Writes are serialized through it for durability.
// See design §6.2 and §6.5.
type WAL struct {
	mu   sync.Mutex
	file *os.File
	dir  string
}

// OpenWAL opens or creates the WAL in the given directory. See design §6.2.
func OpenWAL(dir string) (*WAL, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("wal: create dir: %w", err)
	}

	p := filepath.Join(dir, "current.wal")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("wal: open: %w", err)
	}

	return &WAL{file: f, dir: dir}, nil
}

// Append writes a record to the WAL. See design §6.2 step 1.
func (w *WAL) Append(rec WALRecord) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Encode: [op(1)] + [memoryID(16)] + [data(N)]
	payloadLen := 1 + 16 + len(rec.Data)
	payload := make([]byte, payloadLen)
	payload[0] = byte(rec.Op)
	copy(payload[1:17], rec.MemoryID[:])
	copy(payload[17:], rec.Data)

	checksum := crc32.Checksum(payload, crc32cTable)

	// Write: [length(4)] + [payload(N)] + [crc(4)]
	header := make([]byte, 4)
	binary.LittleEndian.PutUint32(header, uint32(payloadLen))

	trailer := make([]byte, 4)
	binary.LittleEndian.PutUint32(trailer, checksum)

	if _, err := w.file.Write(header); err != nil {
		return fmt.Errorf("wal: write header: %w", err)
	}
	if _, err := w.file.Write(payload); err != nil {
		return fmt.Errorf("wal: write payload: %w", err)
	}
	if _, err := w.file.Write(trailer); err != nil {
		return fmt.Errorf("wal: write checksum: %w", err)
	}

	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("wal: sync: %w", err)
	}
	return nil
}

// Replay reads all valid records from the WAL. Invalid/truncated records at
// the end are silently skipped (crash recovery). See design §6.5.
func (w *WAL) Replay() ([]WALRecord, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("wal: seek: %w", err)
	}

	var records []WALRecord
	for {
		// Read length header.
		var lenBuf [4]byte
		if _, err := io.ReadFull(w.file, lenBuf[:]); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break // clean end or truncated record
			}
			return nil, fmt.Errorf("wal: read len: %w", err)
		}
		payloadLen := binary.LittleEndian.Uint32(lenBuf[:])

		// Sanity check to avoid OOM on corrupt data.
		if payloadLen > 64*1024*1024 {
			break // corrupt, stop replay
		}

		// Read payload.
		payload := make([]byte, payloadLen)
		if _, err := io.ReadFull(w.file, payload); err != nil {
			break // truncated
		}

		// Read and verify CRC.
		var crcBuf [4]byte
		if _, err := io.ReadFull(w.file, crcBuf[:]); err != nil {
			break // truncated
		}
		expected := binary.LittleEndian.Uint32(crcBuf[:])
		actual := crc32.Checksum(payload, crc32cTable)
		if expected != actual {
			break // corrupt record, stop replay
		}

		if len(payload) < 17 {
			break // too short
		}

		rec := WALRecord{
			Op: WALOp(payload[0]),
		}
		copy(rec.MemoryID[:], payload[1:17])
		if len(payload) > 17 {
			rec.Data = make([]byte, len(payload)-17)
			copy(rec.Data, payload[17:])
		}
		records = append(records, rec)
	}

	return records, nil
}

// Truncate resets the WAL (after a successful flush). See design §6.2 step 3.
func (w *WAL) Truncate() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.file.Truncate(0); err != nil {
		return fmt.Errorf("wal: truncate: %w", err)
	}
	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("wal: seek after truncate: %w", err)
	}
	return nil
}

// Close closes the WAL file.
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}
