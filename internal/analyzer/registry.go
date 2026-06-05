package analyzer

import "fmt"

// New creates an analyzer by ID. See design §7.1.
func New(id string) (Analyzer, error) {
	switch id {
	case "en":
		return NewEnAnalyzer(), nil
	case "ja":
		return NewJaAnalyzer()
	case "bigram":
		return NewBigramAnalyzer(), nil
	default:
		return nil, fmt.Errorf("unknown analyzer: %q", id)
	}
}
