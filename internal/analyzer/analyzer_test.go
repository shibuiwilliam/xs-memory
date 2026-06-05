package analyzer

import (
	"testing"
)

func TestEnAnalyzer(t *testing.T) {
	a := NewEnAnalyzer()
	if a.ID() != "en" {
		t.Errorf("ID = %q", a.ID())
	}

	tokens := a.Analyze("The Quick Brown Fox Jumps Over The Lazy Dog")
	// Should lowercase and remove stop words.
	expected := map[string]bool{
		"quick": true, "brown": true, "fox": true,
		"jumps": true, "over": true, "lazy": true, "dog": true,
	}
	if len(tokens) != len(expected) {
		t.Errorf("got %d tokens %v, want %d", len(tokens), tokens, len(expected))
	}
	for _, tok := range tokens {
		if !expected[tok] {
			t.Errorf("unexpected token %q", tok)
		}
	}
}

func TestEnAnalyzerNFKC(t *testing.T) {
	a := NewEnAnalyzer()
	// Full-width letters should be normalized.
	tokens := a.Analyze("Ｈｅｌｌｏ")
	if len(tokens) != 1 || tokens[0] != "hello" {
		t.Errorf("NFKC: got %v", tokens)
	}
}

func TestBigramAnalyzer(t *testing.T) {
	a := NewBigramAnalyzer()
	if a.ID() != "bigram" {
		t.Errorf("ID = %q", a.ID())
	}

	tokens := a.Analyze("hello")
	expected := []string{"he", "el", "ll", "lo"}
	if len(tokens) != len(expected) {
		t.Fatalf("got %d tokens %v, want %d", len(tokens), tokens, len(expected))
	}
	for i, tok := range tokens {
		if tok != expected[i] {
			t.Errorf("token %d = %q, want %q", i, tok, expected[i])
		}
	}
}

func TestBigramAnalyzerJapanese(t *testing.T) {
	a := NewBigramAnalyzer()
	tokens := a.Analyze("東京タワー")
	// Should produce: 東京, 京タ, タワ, ワー
	if len(tokens) != 4 {
		t.Fatalf("got %d tokens %v, want 4", len(tokens), tokens)
	}
	if tokens[0] != "東京" {
		t.Errorf("first bigram = %q, want 東京", tokens[0])
	}
}

func TestJaAnalyzer(t *testing.T) {
	a, err := NewJaAnalyzer()
	if err != nil {
		t.Fatalf("NewJaAnalyzer: %v", err)
	}
	if a.ID() != "ja" {
		t.Errorf("ID = %q", a.ID())
	}

	tokens := a.Analyze("東京タワーは高い建物です")
	// Should contain: 東京, タワー, 高い, 建物
	// Should NOT contain particles: は, です
	tokenSet := make(map[string]bool)
	for _, tok := range tokens {
		tokenSet[tok] = true
	}

	for _, want := range []string{"東京", "タワー"} {
		if !tokenSet[want] {
			t.Errorf("missing expected token %q in %v", want, tokens)
		}
	}
	for _, unwant := range []string{"は", "です"} {
		if tokenSet[unwant] {
			t.Errorf("unexpected particle %q in %v", unwant, tokens)
		}
	}
}

func TestJaAnalyzerSentence(t *testing.T) {
	a, err := NewJaAnalyzer()
	if err != nil {
		t.Fatalf("NewJaAnalyzer: %v", err)
	}

	tokens := a.Analyze("人工知能がプログラミングを支援する")
	tokenSet := make(map[string]bool)
	for _, tok := range tokens {
		tokenSet[tok] = true
	}

	// Key content words should be present.
	for _, want := range []string{"人工", "知能", "プログラミング", "支援"} {
		if !tokenSet[want] {
			t.Errorf("missing %q in %v", want, tokens)
		}
	}
}

func TestRegistryNew(t *testing.T) {
	for _, id := range []string{"en", "ja", "bigram"} {
		a, err := New(id)
		if err != nil {
			t.Errorf("New(%q): %v", id, err)
			continue
		}
		if a.ID() != id {
			t.Errorf("New(%q).ID() = %q", id, a.ID())
		}
	}

	_, err := New("unknown")
	if err == nil {
		t.Error("New(unknown) should error")
	}
}
