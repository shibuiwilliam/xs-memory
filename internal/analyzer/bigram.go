package analyzer

import "unicode/utf8"

// BigramAnalyzer is the fallback analyzer for unknown languages.
// Generates character bigrams so any text is searchable. See design §7.1.
type BigramAnalyzer struct{}

func NewBigramAnalyzer() *BigramAnalyzer {
	return &BigramAnalyzer{}
}

func (a *BigramAnalyzer) Analyze(text string) []string {
	text = normalize(text)
	runes := []rune(text)

	var tokens []string
	for i := 0; i < len(runes)-1; i++ {
		r1, r2 := runes[i], runes[i+1]
		// Skip whitespace bigrams.
		if r1 == ' ' || r2 == ' ' || r1 == '\n' || r2 == '\n' {
			continue
		}
		if !utf8.ValidRune(r1) || !utf8.ValidRune(r2) {
			continue
		}
		tokens = append(tokens, string([]rune{r1, r2}))
	}
	return tokens
}

func (a *BigramAnalyzer) ID() string { return "bigram" }
