package provider

import (
	"context"
	"math"
	"testing"
)

func TestMockEmbedder(t *testing.T) {
	e := NewMockEmbedder(128)
	if e.Dim() != 128 {
		t.Errorf("Dim = %d", e.Dim())
	}
	if e.ID() != "mock" {
		t.Errorf("ID = %q", e.ID())
	}

	ctx := context.Background()
	vecs, err := e.Embed(ctx, []string{"hello", "world"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vecs) != 2 {
		t.Fatalf("got %d vectors", len(vecs))
	}
	if len(vecs[0]) != 128 {
		t.Fatalf("dim = %d", len(vecs[0]))
	}

	// Deterministic: same input → same output.
	vecs2, _ := e.Embed(ctx, []string{"hello"})
	for i := range vecs[0] {
		if vecs[0][i] != vecs2[0][i] {
			t.Fatalf("not deterministic at dim %d", i)
		}
	}

	// Unit vector check.
	var norm float64
	for _, v := range vecs[0] {
		norm += float64(v) * float64(v)
	}
	norm = math.Sqrt(norm)
	if math.Abs(norm-1.0) > 0.01 {
		t.Errorf("norm = %f, want ~1.0", norm)
	}
}

func TestMockEmbedderDifferentTexts(t *testing.T) {
	e := NewMockEmbedder(64)
	ctx := context.Background()

	v1, _ := e.Embed(ctx, []string{"hello"})
	v2, _ := e.Embed(ctx, []string{"goodbye"})

	// Different texts should produce different vectors.
	same := true
	for i := range v1[0] {
		if v1[0][i] != v2[0][i] {
			same = false
			break
		}
	}
	if same {
		t.Error("different texts should produce different vectors")
	}
}

func TestMockEmbedderJapanese(t *testing.T) {
	e := NewMockEmbedder(64)
	ctx := context.Background()

	vecs, err := e.Embed(ctx, []string{"東京タワー", "京都の金閣寺"})
	if err != nil {
		t.Fatalf("Embed Japanese: %v", err)
	}
	if len(vecs) != 2 {
		t.Fatalf("got %d vectors", len(vecs))
	}
}

func TestMockLLM(t *testing.T) {
	llm := NewMockLLM(`{"result": "test"}`)
	resp, err := llm.Complete(context.Background(), CompletionRequest{Prompt: "test"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != `{"result": "test"}` {
		t.Errorf("response = %q", resp.Text)
	}
}
