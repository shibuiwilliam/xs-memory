// Package organizer implements async LLM jobs for memory organization.
// All jobs are non-critical-path and optional. See design §10.
package organizer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/xs-memory/xs-memory/internal/provider"
)

// JobKind identifies the type of organization job. See design §10.
type JobKind string

const (
	JobExtract    JobKind = "extract"    // entity/relation extraction
	JobAutoTag    JobKind = "autotag"    // auto-tagging/classification
	JobImportance JobKind = "importance" // importance scoring
	JobDedup      JobKind = "dedup"      // duplicate detection
	JobSummarize  JobKind = "summarize"  // consolidation/summary
)

// ExtractResult is the output of entity extraction.
type ExtractResult struct {
	Entities  []Entity   `json:"entities"`
	Relations []Relation `json:"relations"`
}

// Entity is an extracted entity.
type Entity struct {
	Name string `json:"name"`
	Type string `json:"type"` // person, place, concept, etc.
}

// Relation is an extracted relation.
type Relation struct {
	Subject   string  `json:"subject"`
	Predicate string  `json:"predicate"`
	Object    string  `json:"object"`
	Weight    float32 `json:"weight"`
}

// TagResult is the output of auto-tagging.
type TagResult struct {
	Tags []string `json:"tags"`
	Type string   `json:"type"` // episodic, semantic, procedural
}

// ImportanceResult is the output of importance scoring.
type ImportanceResult struct {
	Score  float32 `json:"score"`
	Reason string  `json:"reason"`
}

// Organizer runs async LLM jobs. See design §10.
type Organizer struct {
	llm    provider.LLM
	logger *slog.Logger
}

// New creates an Organizer. LLM may be nil (all jobs become no-ops).
func New(llm provider.LLM, logger *slog.Logger) *Organizer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Organizer{llm: llm, logger: logger}
}

// ExtractEntities extracts entities and relations from text. See design §10.
func (o *Organizer) ExtractEntities(ctx context.Context, text string) (*ExtractResult, error) {
	if o.llm == nil {
		return &ExtractResult{}, nil
	}

	prompt := fmt.Sprintf(`Extract entities and relations from the following text.
Return JSON with format: {"entities": [{"name": "...", "type": "..."}], "relations": [{"subject": "...", "predicate": "...", "object": "...", "weight": 1.0}]}

Text:
%s`, truncate(text, 2000))

	resp, err := o.llm.Complete(ctx, provider.CompletionRequest{
		System: "You are an entity extraction system. Return only valid JSON.",
		Prompt: prompt,
		JSON:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("organizer: extract: %w", err)
	}

	var result ExtractResult
	if err := json.Unmarshal([]byte(resp.Text), &result); err != nil {
		o.logger.Warn("organizer: extract parse failed", "error", err)
		return &ExtractResult{}, nil
	}
	return &result, nil
}

// AutoTag generates tags and classifies memory type. See design §10.
func (o *Organizer) AutoTag(ctx context.Context, text string) (*TagResult, error) {
	if o.llm == nil {
		return &TagResult{}, nil
	}

	prompt := fmt.Sprintf(`Analyze the following text and:
1. Suggest relevant tags (up to 5)
2. Classify as: episodic (events/conversations), semantic (facts/knowledge), or procedural (skills/procedures)
Return JSON: {"tags": ["tag1", "tag2"], "type": "semantic"}

Text:
%s`, truncate(text, 2000))

	resp, err := o.llm.Complete(ctx, provider.CompletionRequest{
		System: "You are a text classification system. Return only valid JSON.",
		Prompt: prompt,
		JSON:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("organizer: autotag: %w", err)
	}

	var result TagResult
	if err := json.Unmarshal([]byte(resp.Text), &result); err != nil {
		o.logger.Warn("organizer: autotag parse failed", "error", err)
		return &TagResult{}, nil
	}
	return &result, nil
}

// ScoreImportance evaluates the importance of a memory. See design §10.
func (o *Organizer) ScoreImportance(ctx context.Context, text string) (*ImportanceResult, error) {
	if o.llm == nil {
		return &ImportanceResult{Score: 0.5}, nil
	}

	prompt := fmt.Sprintf(`Rate the importance of the following text on a scale of 0.0 to 1.0.
Consider: specificity, actionability, uniqueness, and potential future usefulness.
Return JSON: {"score": 0.7, "reason": "brief explanation"}

Text:
%s`, truncate(text, 2000))

	resp, err := o.llm.Complete(ctx, provider.CompletionRequest{
		System: "You are an importance scoring system. Return only valid JSON.",
		Prompt: prompt,
		JSON:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("organizer: importance: %w", err)
	}

	var result ImportanceResult
	if err := json.Unmarshal([]byte(resp.Text), &result); err != nil {
		o.logger.Warn("organizer: importance parse failed", "error", err)
		return &ImportanceResult{Score: 0.5}, nil
	}
	return &result, nil
}

func truncate(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "..."
	}
	return s
}
