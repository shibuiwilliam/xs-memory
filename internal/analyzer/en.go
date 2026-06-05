package analyzer

// EnAnalyzer is the English text analyzer.
// Pipeline: NFKC normalize → Unicode word split → lowercase → stop words.
// See design §7.1.
type EnAnalyzer struct{}

func NewEnAnalyzer() *EnAnalyzer {
	return &EnAnalyzer{}
}

func (a *EnAnalyzer) Analyze(text string) []string {
	text = normalize(text)
	words := splitWords(text)

	tokens := make([]string, 0, len(words))
	for _, w := range words {
		if len(w) == 0 || enStopWords[w] {
			continue
		}
		tokens = append(tokens, w)
	}
	return tokens
}

func (a *EnAnalyzer) ID() string { return "en" }
