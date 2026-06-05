package metrics

import (
	"fmt"
	"testing"
)

func TestTopKBounded(t *testing.T) {
	tk := NewTopKTerms(5, false)

	// Insert 10 distinct terms.
	for i := 0; i < 10; i++ {
		tk.Record([]string{fmt.Sprintf("term_%d", i)})
	}

	// Should never exceed maxK (5). See addendum3 M7.
	if tk.Len() > 5 {
		t.Errorf("Len() = %d, want ≤ 5", tk.Len())
	}
}

func TestTopKCounting(t *testing.T) {
	tk := NewTopKTerms(100, false)

	tk.Record([]string{"a", "b"})
	tk.Record([]string{"a", "c"})
	tk.Record([]string{"a"})

	snap := tk.Snapshot()
	// "a" should be first with count 3.
	if len(snap) == 0 || snap[0].Token != "a" || snap[0].Count != 3 {
		t.Errorf("top entry = %+v, want {a, 3}", snap[0])
	}
}

func TestTopKDecay(t *testing.T) {
	tk := NewTopKTerms(100, false)
	tk.Record([]string{"x"})
	tk.Record([]string{"x"})

	// Decay by 0.5.
	tk.Decay(0.5)

	snap := tk.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("entries = %d, want 1", len(snap))
	}
	if snap[0].Count != 1.0 { // 2.0 * 0.5 = 1.0
		t.Errorf("count = %f, want 1.0", snap[0].Count)
	}

	// Heavy decay should evict entries below threshold.
	tk.Decay(0.001)
	snap = tk.Snapshot()
	if len(snap) != 0 {
		t.Errorf("entries after heavy decay = %d, want 0", len(snap))
	}
}

func TestTopKReset(t *testing.T) {
	tk := NewTopKTerms(100, false)
	tk.Record([]string{"x", "y"})

	tk.Reset()
	if tk.Len() != 0 {
		t.Errorf("Len after reset = %d, want 0", tk.Len())
	}
}
