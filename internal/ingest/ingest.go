// Package ingest implements the ingestion pipeline: parse → chunk → embed → index.
// See design §9.
package ingest

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/oklog/ulid/v2"

	"github.com/xs-memory/xs-memory/internal/provider"
	"github.com/xs-memory/xs-memory/internal/storage"
)

// Config holds ingestion configuration. See design §14.
type Config struct {
	ChunkTokens  int // target chunk size in tokens, default 512
	ChunkOverlap int // overlap tokens, default 64
}

// DefaultConfig returns default ingestion settings.
func DefaultConfig() Config {
	return Config{
		ChunkTokens:  512,
		ChunkOverlap: 64,
	}
}

// Result is the output of the ingestion pipeline for one memory.
type Result struct {
	Memory *storage.Memory
	Chunks []storage.Chunk
}

// Pipeline orchestrates the ingestion flow. See design §9.
type Pipeline struct {
	embedder provider.Embedder // may be nil (degraded mode per N4)
	cfg      Config
}

// NewPipeline creates an ingestion pipeline.
func NewPipeline(embedder provider.Embedder, cfg Config) *Pipeline {
	if cfg.ChunkTokens == 0 {
		cfg = DefaultConfig()
	}
	return &Pipeline{embedder: embedder, cfg: cfg}
}

// Ingest processes text content into a Memory with Chunks.
// See design §9: parse → normalize → chunk → embed → index.
func (p *Pipeline) Ingest(ctx context.Context, opts IngestOpts) (*Result, error) {
	now := time.Now()

	mem := &storage.Memory{
		ID:          opts.ID,
		Collection:  opts.Collection,
		Content:     opts.Content,
		ContentType: opts.ContentType,
		Source:      opts.Source,
		Type:        opts.Type,
		Metadata:    opts.Metadata,
		Importance:  opts.Importance,
		CreatedAt:   now,
		UpdatedAt:   now,
		AccessedAt:  now,
	}

	// Content-addressed blob ref for dedup. See design §9.
	hash := sha256.Sum256([]byte(opts.Content))
	mem.BlobRef = fmt.Sprintf("%x", hash[:])

	// Chunk the content.
	texts := chunkText(opts.Content, p.cfg.ChunkTokens, p.cfg.ChunkOverlap)

	chunks := make([]storage.Chunk, len(texts))
	for i, text := range texts {
		chunkID, err := ulid.New(ulid.Now(), ulid.DefaultEntropy())
		if err != nil {
			return nil, fmt.Errorf("ingest: generate chunk id: %w", err)
		}
		chunks[i] = storage.Chunk{
			ID:       chunkID,
			MemoryID: opts.ID,
			Ord:      i,
			Text:     text,
			Tokens:   estimateTokens(text),
		}
	}

	// Generate embeddings if provider available (N4: not required).
	if p.embedder != nil {
		chunkTexts := make([]string, len(chunks))
		for i := range chunks {
			chunkTexts[i] = chunks[i].Text
		}
		vectors, err := p.embedder.Embed(ctx, chunkTexts)
		if err != nil {
			// Degraded mode: log warning but continue without vectors.
			// FTS will still work. See design §9.
			_ = err // TODO: slog.Warn
		} else {
			for i := range chunks {
				if i < len(vectors) {
					chunks[i].Vector = vectors[i]
				}
			}
		}
	}

	return &Result{Memory: mem, Chunks: chunks}, nil
}

// IngestOpts are the input parameters for ingestion.
type IngestOpts struct {
	ID          ulid.ULID
	Collection  string
	Content     string
	ContentType string
	Source      string
	Type        storage.MemoryType
	Metadata    map[string]any
	Importance  float32
}

// chunkText splits text into chunks of approximately maxTokens size with overlap.
// Uses sentence boundaries when possible. See design §9.
func chunkText(text string, maxTokens, overlap int) []string {
	if maxTokens <= 0 {
		maxTokens = 512
	}

	tokens := estimateTokens(text)
	if tokens <= maxTokens {
		return []string{text}
	}

	// Split on sentence boundaries (period, question mark, newline).
	sentences := splitSentences(text)

	var chunks []string
	var current strings.Builder
	currentTokens := 0

	for _, s := range sentences {
		sTokens := estimateTokens(s)
		if currentTokens+sTokens > maxTokens && currentTokens > 0 {
			chunks = append(chunks, strings.TrimSpace(current.String()))
			// Overlap: keep some trailing content.
			overlapText := getOverlap(current.String(), overlap)
			current.Reset()
			current.WriteString(overlapText)
			currentTokens = estimateTokens(overlapText)
		}
		current.WriteString(s)
		currentTokens += sTokens
	}
	if currentTokens > 0 {
		chunks = append(chunks, strings.TrimSpace(current.String()))
	}

	return chunks
}

// splitSentences splits text into sentence-like segments.
func splitSentences(text string) []string {
	var sentences []string
	var current strings.Builder

	for _, r := range text {
		current.WriteRune(r)
		if r == '.' || r == '。' || r == '!' || r == '?' || r == '\n' {
			sentences = append(sentences, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		sentences = append(sentences, current.String())
	}
	return sentences
}

// getOverlap returns the trailing portion of text approximating overlapTokens.
func getOverlap(text string, overlapTokens int) string {
	runes := []rune(text)
	// Rough estimate: 1 token ≈ 3 runes for CJK, 4 chars for English.
	overlapRunes := overlapTokens * 3
	if overlapRunes >= len(runes) {
		return text
	}
	return string(runes[len(runes)-overlapRunes:])
}

// estimateTokens provides a rough token count.
// For CJK: ~1 token per character. For English: ~1 token per 4 chars.
func estimateTokens(text string) int {
	runeCount := utf8.RuneCountInString(text)
	if runeCount == 0 {
		return 0
	}
	// Simple heuristic: use rune count as upper bound.
	return runeCount
}
