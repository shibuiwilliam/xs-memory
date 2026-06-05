package storage

import (
	"crypto/rand"
	"path/filepath"
	"testing"

	"github.com/oklog/ulid/v2"
)

func newTestULID(t *testing.T) ulid.ULID {
	t.Helper()
	id, err := ulid.New(ulid.Now(), rand.Reader)
	if err != nil {
		t.Fatalf("ulid: %v", err)
	}
	return id
}

func TestWALAppendReplay(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "wal")
	w, err := OpenWAL(dir)
	if err != nil {
		t.Fatalf("OpenWAL: %v", err)
	}
	defer w.Close()

	id1 := newTestULID(t)
	id2 := newTestULID(t)

	// Append two records.
	err = w.Append(WALRecord{Op: WALOpPut, MemoryID: id1, Data: []byte(`{"content":"hello"}`)})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	err = w.Append(WALRecord{Op: WALOpDelete, MemoryID: id2})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	// Replay.
	records, err := w.Replay()
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}

	if records[0].Op != WALOpPut || records[0].MemoryID != id1 {
		t.Errorf("record 0: op=%d id=%s", records[0].Op, ulid.ULID(records[0].MemoryID))
	}
	if string(records[0].Data) != `{"content":"hello"}` {
		t.Errorf("record 0 data: %q", records[0].Data)
	}
	if records[1].Op != WALOpDelete || records[1].MemoryID != id2 {
		t.Errorf("record 1: op=%d id=%s", records[1].Op, ulid.ULID(records[1].MemoryID))
	}
}

func TestWALCrashRecovery(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "wal")
	w, err := OpenWAL(dir)
	if err != nil {
		t.Fatalf("OpenWAL: %v", err)
	}

	id := newTestULID(t)
	err = w.Append(WALRecord{Op: WALOpPut, MemoryID: id, Data: []byte("valid")})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	// Simulate crash: write partial/corrupt data at end.
	w.file.Write([]byte{0xFF, 0xFF, 0xFF})
	w.Close()

	// Reopen and replay — should recover the valid record and skip garbage.
	w2, err := OpenWAL(dir)
	if err != nil {
		t.Fatalf("OpenWAL after crash: %v", err)
	}
	defer w2.Close()

	records, err := w2.Replay()
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1 (crash recovery)", len(records))
	}
	if records[0].MemoryID != id {
		t.Errorf("recovered wrong record")
	}
}

func TestWALTruncate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "wal")
	w, err := OpenWAL(dir)
	if err != nil {
		t.Fatalf("OpenWAL: %v", err)
	}
	defer w.Close()

	id := newTestULID(t)
	w.Append(WALRecord{Op: WALOpPut, MemoryID: id, Data: []byte("data")})

	if err := w.Truncate(); err != nil {
		t.Fatalf("Truncate: %v", err)
	}

	records, err := w.Replay()
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("got %d records after truncate, want 0", len(records))
	}
}

func TestWALJapaneseContent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "wal")
	w, err := OpenWAL(dir)
	if err != nil {
		t.Fatalf("OpenWAL: %v", err)
	}
	defer w.Close()

	id := newTestULID(t)
	jpData := []byte(`{"content":"日本語のテストデータです。東京タワーは高い。"}`)
	err = w.Append(WALRecord{Op: WALOpPut, MemoryID: id, Data: jpData})
	if err != nil {
		t.Fatalf("Append Japanese: %v", err)
	}

	records, err := w.Replay()
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if string(records[0].Data) != string(jpData) {
		t.Errorf("Japanese data mismatch: got %q", records[0].Data)
	}
}
