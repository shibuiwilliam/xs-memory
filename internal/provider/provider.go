// Package provider defines interfaces and implementations for embedding and LLM providers.
// See design §11.
package provider

import "context"

// Embedder generates vector embeddings from text. See design §11.
type Embedder interface {
	// Embed generates embeddings for the given texts.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Dim returns the embedding dimension.
	Dim() int
	// ID returns the model identifier (pinned per collection, see N6).
	ID() string
}

// LLM provides text completion. See design §11.
type LLM interface {
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}

// CompletionRequest is a request to an LLM.
type CompletionRequest struct {
	System string
	Prompt string
	JSON   bool // request JSON output
}

// CompletionResponse is a response from an LLM.
type CompletionResponse struct {
	Text string
}
