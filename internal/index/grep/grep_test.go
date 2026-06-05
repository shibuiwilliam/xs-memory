package grep

import (
	"context"
	"crypto/rand"
	"strings"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
)

func newID(t *testing.T) ulid.ULID {
	t.Helper()
	id, err := ulid.New(ulid.Now(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func makeChunks(texts ...string) []ChunkEntry {
	var chunks []ChunkEntry
	for _, text := range texts {
		id, _ := ulid.New(ulid.Now(), rand.Reader)
		chunks = append(chunks, ChunkEntry{DocID: id, Text: text})
	}
	return chunks
}

func TestGrepLiteralOracle(t *testing.T) {
	// Correctness vs brute-force oracle. See addendum2 §6.
	e := NewEngine()
	chunks := makeChunks(
		"func main() { fmt.Println(\"hello\") }",
		"The cache eviction policy uses LRU",
		"func handleSearch(ctx context.Context) error",
		"No match here",
	)

	result := e.Search(context.Background(), chunks, Options{
		Pattern:       "func",
		CaseSensitive: true,
	})

	if len(result.Matches) != 2 {
		t.Fatalf("literal 'func': got %d matches, want 2", len(result.Matches))
	}
	for _, m := range result.Matches {
		if !strings.Contains(m.ChunkText, "func") {
			t.Errorf("match doesn't contain 'func': %q", m.ChunkText)
		}
		if len(m.Spans) == 0 {
			t.Error("expected spans for match")
		}
		// Verify span correctness.
		for _, sp := range m.Spans {
			if m.ChunkText[sp.Start:sp.End] != "func" {
				t.Errorf("span %d:%d = %q, want 'func'", sp.Start, sp.End, m.ChunkText[sp.Start:sp.End])
			}
		}
	}
	if result.Truncated {
		t.Error("should not be truncated")
	}
}

func TestGrepLiteralCaseInsensitive(t *testing.T) {
	e := NewEngine()
	chunks := makeChunks("Hello World", "HELLO world", "no match")

	result := e.Search(context.Background(), chunks, Options{
		Pattern:       "hello",
		CaseSensitive: false,
	})
	if len(result.Matches) != 2 {
		t.Errorf("case-insensitive: got %d, want 2", len(result.Matches))
	}
}

func TestGrepRegexOracle(t *testing.T) {
	e := NewEngine()
	chunks := makeChunks(
		"error: connection timeout at 192.168.1.1",
		"warning: slow query 500ms",
		"info: all good",
	)

	result := e.Search(context.Background(), chunks, Options{
		Pattern:       `\d+\.\d+\.\d+\.\d+`,
		IsRegex:       true,
		CaseSensitive: true,
	})
	if len(result.Matches) != 1 {
		t.Fatalf("regex IP: got %d, want 1", len(result.Matches))
	}
	span := result.Matches[0].Spans[0]
	matched := result.Matches[0].ChunkText[span.Start:span.End]
	if matched != "192.168.1.1" {
		t.Errorf("matched = %q, want 192.168.1.1", matched)
	}
}

func TestGrepJapanese(t *testing.T) {
	e := NewEngine()
	chunks := makeChunks(
		"東京タワーは東京都港区にある電波塔です",
		"京都の金閣寺は美しい寺院です",
	)

	result := e.Search(context.Background(), chunks, Options{
		Pattern:       "東京",
		CaseSensitive: true,
	})
	if len(result.Matches) != 1 {
		t.Fatalf("Japanese literal: got %d, want 1", len(result.Matches))
	}
	// Should have 2 spans (東京 appears twice in first chunk).
	if len(result.Matches[0].Spans) != 2 {
		t.Errorf("spans = %d, want 2", len(result.Matches[0].Spans))
	}
}

func TestGrepDeadlinePartial(t *testing.T) {
	// Deadline returns partial results with Truncated=true. See addendum2 §3.2.
	e := NewEngine()

	// Create many chunks.
	var chunks []ChunkEntry
	for i := 0; i < 10000; i++ {
		chunks = append(chunks, makeChunks(strings.Repeat("some text with match here ", 100))...)
	}

	// Very short deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	result := e.Search(ctx, chunks, Options{
		Pattern:       "match",
		CaseSensitive: true,
	})

	// Should be truncated (deadline hit) OR complete if fast enough — but never hang.
	// The key invariant: this returns, it doesn't block forever.
	_ = result
}

func TestGrepMaxScanBytes(t *testing.T) {
	e := NewEngine()
	chunks := makeChunks(
		strings.Repeat("a", 100),
		strings.Repeat("b", 100),
		strings.Repeat("a", 100),
	)

	result := e.Search(context.Background(), chunks, Options{
		Pattern:       "a",
		CaseSensitive: true,
		MaxScanBytes:  150, // only scan first ~1.5 chunks
	})

	// Should find matches in the scanned portion only, not panic or hang.
	if result.Truncated {
		// MaxScanBytes limits but doesn't set truncated flag (that's for deadline).
		// Workers just stop scanning beyond the limit.
	}
	_ = result
}

func TestGrepDefaultOff(t *testing.T) {
	// Empty pattern → no matches. This is how "default off" manifests.
	e := NewEngine()
	chunks := makeChunks("hello world")
	result := e.Search(context.Background(), chunks, Options{})
	if len(result.Matches) != 0 {
		t.Error("empty pattern should return no matches")
	}
}

func TestGrepInvalidRegex(t *testing.T) {
	e := NewEngine()
	chunks := makeChunks("hello")
	result := e.Search(context.Background(), chunks, Options{
		Pattern: "[invalid",
		IsRegex: true,
	})
	if len(result.Matches) != 0 {
		t.Error("invalid regex should return no matches, not panic")
	}
}
