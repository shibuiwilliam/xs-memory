// Package grep provides a literal/regex scan over chunk text as an optional
// search lane. Pure Go (literal + RE2), parallel, deadline-bounded.
// See addendum2 §3.
package grep

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/oklog/ulid/v2"
)

// Span marks an exact match location in chunk text. See addendum2 §3.3.
type Span struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Match is a grep result for one chunk. See addendum2 §3.3.
type Match struct {
	DocID     ulid.ULID `json:"doc_id"`
	ChunkText string    `json:"chunk_text"`
	Spans     []Span    `json:"spans"`
}

// Result is the output of a grep search.
type Result struct {
	Matches   []Match `json:"matches"`
	Truncated bool    `json:"truncated"` // true if deadline was hit
}

// Options controls grep behavior. See addendum2 §3.4.
type Options struct {
	Pattern       string
	IsRegex       bool
	CaseSensitive bool
	MaxScanBytes  int // 0 = unlimited
}

// ChunkSource provides chunk text for scanning. Abstracted so grep
// doesn't depend on storage internals directly.
type ChunkSource interface {
	// AllChunks returns (docID, chunkText) pairs for the given collection.
	// Should respect the context deadline.
	AllChunks(ctx context.Context, collection string) ([]ChunkEntry, error)
}

// ChunkEntry is a single chunk to scan.
type ChunkEntry struct {
	DocID ulid.ULID
	Text  string
}

// Engine performs grep scans. See addendum2 §3.1.
type Engine struct{}

// NewEngine creates a grep engine.
func NewEngine() *Engine {
	return &Engine{}
}

// Search scans chunks for literal or regex matches. Deadline-bounded:
// if the context deadline is hit, returns partial results with Truncated=true.
// See addendum2 §3.2.
func (e *Engine) Search(ctx context.Context, chunks []ChunkEntry, opts Options) Result {
	if opts.Pattern == "" {
		return Result{}
	}

	var matcher func(text string) []Span
	if opts.IsRegex {
		m, err := buildRegexMatcher(opts.Pattern, opts.CaseSensitive)
		if err != nil {
			return Result{} // invalid regex → no matches
		}
		matcher = m
	} else {
		matcher = buildLiteralMatcher(opts.Pattern, opts.CaseSensitive)
	}

	// Parallel scan with workers. See addendum2 §3.2.
	const numWorkers = 4
	type indexedMatch struct {
		idx   int
		match Match
	}

	resultCh := make(chan indexedMatch, len(chunks))
	chunkCh := make(chan int, len(chunks))
	truncated := false

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var scanned int
			for idx := range chunkCh {
				// Check deadline.
				select {
				case <-ctx.Done():
					return
				default:
				}

				chunk := chunks[idx]
				if opts.MaxScanBytes > 0 {
					scanned += len(chunk.Text)
					if scanned > opts.MaxScanBytes {
						return
					}
				}

				spans := matcher(chunk.Text)
				if len(spans) > 0 {
					resultCh <- indexedMatch{
						idx: idx,
						match: Match{
							DocID:     chunk.DocID,
							ChunkText: chunk.Text,
							Spans:     spans,
						},
					}
				}
			}
		}()
	}

	// Feed chunks.
	for i := range chunks {
		select {
		case <-ctx.Done():
			truncated = true
			break
		case chunkCh <- i:
		}
		if truncated {
			break
		}
	}
	close(chunkCh)
	wg.Wait()
	close(resultCh)

	// Check if context expired during scan.
	if ctx.Err() != nil {
		truncated = true
	}

	// Collect results.
	var matches []Match
	for im := range resultCh {
		matches = append(matches, im.match)
	}

	return Result{Matches: matches, Truncated: truncated}
}

// buildLiteralMatcher creates a literal substring matcher. See addendum2 §3.1.
func buildLiteralMatcher(pattern string, caseSensitive bool) func(string) []Span {
	if !caseSensitive {
		pattern = strings.ToLower(pattern)
	}
	return func(text string) []Span {
		searchIn := text
		if !caseSensitive {
			searchIn = strings.ToLower(text)
		}
		var spans []Span
		start := 0
		for {
			idx := strings.Index(searchIn[start:], pattern)
			if idx < 0 {
				break
			}
			absStart := start + idx
			spans = append(spans, Span{Start: absStart, End: absStart + len(pattern)})
			start = absStart + 1
			if start >= len(searchIn) {
				break
			}
		}
		return spans
	}
}

// buildRegexMatcher creates an RE2 regex matcher. Linear-time, safe.
// See addendum2 §3.1.
func buildRegexMatcher(pattern string, caseSensitive bool) (func(string) []Span, error) {
	if !caseSensitive {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("grep: invalid regex: %w", err)
	}
	return func(text string) []Span {
		locs := re.FindAllStringIndex(text, -1)
		spans := make([]Span, len(locs))
		for i, loc := range locs {
			spans[i] = Span{Start: loc[0], End: loc[1]}
		}
		return spans
	}, nil
}
