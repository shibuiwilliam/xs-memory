package xsmem

import (
	"github.com/xs-memory/xs-memory/internal/metrics"
	"github.com/xs-memory/xs-memory/internal/provider"
)

// config holds store configuration. See design §14.
type config struct {
	BlockCacheMB    int
	Embedder        provider.Embedder
	LLM             provider.LLM
	ChunkTokens     int
	ChunkOverlap    int
	DefaultAnalyzer string
	MetricsCfg      metrics.Config // see addendum3 §4
}

func defaultConfig() config {
	return config{
		BlockCacheMB: 256, // see design §6.3, §14
		ChunkTokens:  512,
		ChunkOverlap: 64,
	}
}

// Option configures a Store.
type Option func(*config)

// WithBlockCacheMB sets the block cache memory budget in megabytes.
// See design §6.3 (block cache LRU) and N2 (memory budget).
func WithBlockCacheMB(mb int) Option {
	return func(c *config) {
		c.BlockCacheMB = mb
	}
}

// WithEmbedder sets the embedding provider.
func WithEmbedder(e provider.Embedder) Option {
	return func(c *config) {
		c.Embedder = e
	}
}

// WithDefaultAnalyzer sets the default analyzer for auto-created collections.
func WithDefaultAnalyzer(id string) Option {
	return func(c *config) {
		c.DefaultAnalyzer = id
	}
}

// WithChunkConfig sets chunking parameters. See design §14.
func WithChunkConfig(tokens, overlap int) Option {
	return func(c *config) {
		c.ChunkTokens = tokens
		c.ChunkOverlap = overlap
	}
}

// WithLLM sets the LLM provider for the organizer. See design §10.
func WithLLM(llm provider.LLM) Option {
	return func(c *config) {
		c.LLM = llm
	}
}

// WithMetrics sets the metrics configuration. See addendum3 §4, M1.
func WithMetrics(cfg metrics.Config) Option {
	return func(c *config) {
		c.MetricsCfg = cfg
	}
}

// MetricsConfig re-exports for public API access. See addendum3 §4.
type MetricsConfig = metrics.Config

// MetricsSnapshot re-exports for public API access. See addendum3 §3.
type MetricsSnapshot = metrics.Snapshot

// StructuralStats re-exports for public API access. See addendum3 §1.6.
type StructuralStatsInfo = metrics.StructuralStats
