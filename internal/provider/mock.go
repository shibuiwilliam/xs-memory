package provider

import (
	"context"
	"hash/fnv"
	"math"
)

// MockEmbedder generates deterministic embeddings for testing.
// Uses FNV hash of text to produce reproducible vectors. See design §11.
type MockEmbedder struct {
	dim int
}

func NewMockEmbedder(dim int) *MockEmbedder {
	return &MockEmbedder{dim: dim}
}

func (m *MockEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, text := range texts {
		result[i] = mockEmbed(text, m.dim)
	}
	return result, nil
}

func (m *MockEmbedder) Dim() int   { return m.dim }
func (m *MockEmbedder) ID() string { return "mock" }

// mockEmbed generates a deterministic unit vector from text.
func mockEmbed(text string, dim int) []float32 {
	h := fnv.New64a()
	h.Write([]byte(text))
	seed := h.Sum64()

	vec := make([]float32, dim)
	var norm float64
	for j := range vec {
		// Simple deterministic pseudo-random from seed.
		seed = seed*6364136223846793005 + 1442695040888963407
		val := float64(int64(seed)) / float64(math.MaxInt64)
		vec[j] = float32(val)
		norm += val * val
	}

	// Normalize to unit vector.
	norm = math.Sqrt(norm)
	if norm > 0 {
		for j := range vec {
			vec[j] = float32(float64(vec[j]) / norm)
		}
	}
	return vec
}

// MockLLM is a test LLM that returns canned responses.
type MockLLM struct {
	Response string
}

func NewMockLLM(response string) *MockLLM {
	return &MockLLM{Response: response}
}

func (m *MockLLM) Complete(_ context.Context, _ CompletionRequest) (CompletionResponse, error) {
	return CompletionResponse{Text: m.Response}, nil
}
