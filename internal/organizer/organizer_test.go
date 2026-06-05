package organizer

import (
	"context"
	"testing"

	"github.com/xs-memory/xs-memory/internal/provider"
)

func TestExtractWithMock(t *testing.T) {
	llm := provider.NewMockLLM(`{"entities": [{"name": "Tokyo Tower", "type": "landmark"}], "relations": [{"subject": "Tokyo Tower", "predicate": "located_in", "object": "Tokyo", "weight": 1.0}]}`)
	org := New(llm, nil)

	result, err := org.ExtractEntities(context.Background(), "Tokyo Tower is a famous landmark in Tokyo.")
	if err != nil {
		t.Fatalf("ExtractEntities: %v", err)
	}
	if len(result.Entities) != 1 {
		t.Errorf("entities = %d, want 1", len(result.Entities))
	}
	if len(result.Relations) != 1 {
		t.Errorf("relations = %d, want 1", len(result.Relations))
	}
	if result.Relations[0].Predicate != "located_in" {
		t.Errorf("predicate = %q", result.Relations[0].Predicate)
	}
}

func TestAutoTagWithMock(t *testing.T) {
	llm := provider.NewMockLLM(`{"tags": ["programming", "go"], "type": "procedural"}`)
	org := New(llm, nil)

	result, err := org.AutoTag(context.Background(), "How to write Go tests")
	if err != nil {
		t.Fatalf("AutoTag: %v", err)
	}
	if len(result.Tags) != 2 {
		t.Errorf("tags = %d", len(result.Tags))
	}
	if result.Type != "procedural" {
		t.Errorf("type = %q", result.Type)
	}
}

func TestScoreImportanceWithMock(t *testing.T) {
	llm := provider.NewMockLLM(`{"score": 0.85, "reason": "specific and actionable"}`)
	org := New(llm, nil)

	result, err := org.ScoreImportance(context.Background(), "Critical production fix steps")
	if err != nil {
		t.Fatalf("ScoreImportance: %v", err)
	}
	if result.Score != 0.85 {
		t.Errorf("score = %f, want 0.85", result.Score)
	}
}

func TestOrganizerWithoutLLM(t *testing.T) {
	// No LLM: all jobs should return defaults, not error.
	org := New(nil, nil)

	result, err := org.ExtractEntities(context.Background(), "test")
	if err != nil {
		t.Fatalf("ExtractEntities without LLM: %v", err)
	}
	if len(result.Entities) != 0 {
		t.Error("expected empty entities without LLM")
	}

	imp, err := org.ScoreImportance(context.Background(), "test")
	if err != nil {
		t.Fatalf("ScoreImportance without LLM: %v", err)
	}
	if imp.Score != 0.5 {
		t.Errorf("default importance = %f, want 0.5", imp.Score)
	}
}

func TestExtractJapanese(t *testing.T) {
	llm := provider.NewMockLLM(`{"entities": [{"name": "東京タワー", "type": "landmark"}, {"name": "港区", "type": "location"}], "relations": [{"subject": "東京タワー", "predicate": "所在地", "object": "港区", "weight": 1.0}]}`)
	org := New(llm, nil)

	result, err := org.ExtractEntities(context.Background(), "東京タワーは港区にあります")
	if err != nil {
		t.Fatalf("ExtractEntities Japanese: %v", err)
	}
	if len(result.Entities) != 2 {
		t.Errorf("Japanese entities = %d, want 2", len(result.Entities))
	}
}
