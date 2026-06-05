// Package analyzer provides text analysis pipelines for tokenization.
// Analyzers are pluggable per collection (design §7.1, D6, N8).
package analyzer

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Analyzer tokenizes text for indexing and search. See design §7.1.
type Analyzer interface {
	// Analyze returns tokens from the input text.
	Analyze(text string) []string
	// ID returns the analyzer identifier (pinned per collection).
	ID() string
}

// normalize applies NFKC normalization and lowercasing. See design §7.1.
func normalize(s string) string {
	s = norm.NFKC.String(s)
	return strings.ToLower(s)
}

// isStopWord checks English stop words. Minimal set for MVP.
var enStopWords = map[string]bool{
	"a": true, "an": true, "the": true, "is": true, "are": true,
	"was": true, "were": true, "be": true, "been": true, "being": true,
	"have": true, "has": true, "had": true, "do": true, "does": true,
	"did": true, "will": true, "would": true, "could": true, "should": true,
	"may": true, "might": true, "shall": true, "can": true,
	"to": true, "of": true, "in": true, "for": true, "on": true,
	"with": true, "at": true, "by": true, "from": true, "as": true,
	"into": true, "through": true, "during": true, "before": true, "after": true,
	"and": true, "but": true, "or": true, "nor": true, "not": true,
	"so": true, "yet": true, "both": true, "either": true, "neither": true,
	"it": true, "its": true, "this": true, "that": true, "these": true,
	"those": true, "i": true, "me": true, "my": true, "we": true, "our": true,
	"you": true, "your": true, "he": true, "him": true, "his": true,
	"she": true, "her": true, "they": true, "them": true, "their": true,
}

// splitWords splits text on non-letter/non-digit boundaries (Unicode-aware).
func splitWords(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}
