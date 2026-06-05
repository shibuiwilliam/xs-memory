package metrics

// StructuralStats holds index-level structural statistics.
// Pulled lazily and cached by the per-collection generation counter
// so they recompute only after a write. See addendum3 §1.6, M5.
type StructuralStats struct {
	// FTS.
	FTSTermCount int `json:"fts_term_count"` // unique terms
	FTSDocCount  int `json:"fts_doc_count"`  // indexed documents

	// Vector.
	VectorCount    int  `json:"vector_count"`    // indexed vectors
	VectorDim      int  `json:"vector_dim"`      // dimensions
	VectorQuantize bool `json:"vector_quantize"` // int8 quantization active

	// Graph.
	GraphEdgeCount int `json:"graph_edge_count"` // live triples

	// Store-level.
	Memories    int `json:"memories"`
	Collections int `json:"collections"`
	Tombstones  int `json:"tombstones"`
}
