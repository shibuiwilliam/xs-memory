package analyzer

import (
	"github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"
)

// JaAnalyzer is the Japanese morphological analyzer using Kagome.
// Pipeline: NFKC normalize → Kagome tokenize → filter particles/aux.
// See design §7.1, D6, N8.
type JaAnalyzer struct {
	tok *tokenizer.Tokenizer
}

func NewJaAnalyzer() (*JaAnalyzer, error) {
	tok, err := tokenizer.New(ipa.Dict(), tokenizer.OmitBosEos())
	if err != nil {
		return nil, err
	}
	return &JaAnalyzer{tok: tok}, nil
}

// jaStopPOS lists part-of-speech categories to filter.
// We keep nouns, verbs, adjectives, adverbs — skip particles, auxiliaries, symbols.
var jaStopPOS = map[string]bool{
	"助詞":      true, // particles
	"助動詞":     true, // auxiliary verbs
	"記号":      true, // symbols
	"BOS/EOS": true,
}

func (a *JaAnalyzer) Analyze(text string) []string {
	text = normalize(text)
	tokens := a.tok.Tokenize(text)

	result := make([]string, 0, len(tokens))
	for _, t := range tokens {
		features := t.Features()
		if len(features) == 0 {
			continue
		}
		pos := features[0]
		if jaStopPOS[pos] {
			continue
		}
		surface := t.Surface
		if len(surface) == 0 {
			continue
		}
		result = append(result, surface)
	}
	return result
}

func (a *JaAnalyzer) ID() string { return "ja" }
