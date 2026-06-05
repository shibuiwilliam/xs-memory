package ingest

import (
	"regexp"
	"strings"
)

// Code-aware chunking using regex/indent heuristics.
// This is the default (no CGO) implementation. Tree-sitter is behind build tag.
// See design §9, §15.

// codePatterns matches function/class boundaries in common languages.
var codePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?m)^func\s+`),                                 // Go
	regexp.MustCompile(`(?m)^def\s+`),                                  // Python
	regexp.MustCompile(`(?m)^class\s+`),                                // Python/Java/JS
	regexp.MustCompile(`(?m)^\s*(public|private|protected)\s+`),        // Java/C#
	regexp.MustCompile(`(?m)^(export\s+)?(function|const|let|var)\s+`), // JS/TS
}

// ChunkCode splits source code into chunks at function/class boundaries.
// Falls back to line-based splitting if no patterns match.
func ChunkCode(code string, maxLines int) []string {
	if maxLines <= 0 {
		maxLines = 50
	}

	lines := strings.Split(code, "\n")
	if len(lines) <= maxLines {
		return []string{code}
	}

	// Try to find function/class boundaries.
	boundaries := findBoundaries(lines)
	if len(boundaries) > 1 {
		return splitAtBoundaries(lines, boundaries, maxLines)
	}

	// Fallback: split by blank lines or fixed-size chunks.
	return splitByLines(lines, maxLines)
}

// findBoundaries identifies line indices that look like function/class starts.
func findBoundaries(lines []string) []int {
	var boundaries []int
	for i, line := range lines {
		for _, pat := range codePatterns {
			if pat.MatchString(line) {
				boundaries = append(boundaries, i)
				break
			}
		}
	}
	return boundaries
}

// splitAtBoundaries splits lines at detected boundaries, merging small chunks.
func splitAtBoundaries(lines []string, boundaries []int, maxLines int) []string {
	var chunks []string
	for i := 0; i < len(boundaries); i++ {
		start := boundaries[i]
		end := len(lines)
		if i+1 < len(boundaries) {
			end = boundaries[i+1]
		}
		chunk := strings.Join(lines[start:end], "\n")
		if len(chunk) > 0 {
			chunks = append(chunks, chunk)
		}
	}

	// Include any preamble before first boundary.
	if len(boundaries) > 0 && boundaries[0] > 0 {
		preamble := strings.Join(lines[:boundaries[0]], "\n")
		if len(strings.TrimSpace(preamble)) > 0 {
			chunks = append([]string{preamble}, chunks...)
		}
	}

	return chunks
}

// splitByLines splits into fixed-size line chunks.
func splitByLines(lines []string, maxLines int) []string {
	var chunks []string
	for i := 0; i < len(lines); i += maxLines {
		end := i + maxLines
		if end > len(lines) {
			end = len(lines)
		}
		chunk := strings.Join(lines[i:end], "\n")
		if len(strings.TrimSpace(chunk)) > 0 {
			chunks = append(chunks, chunk)
		}
	}
	return chunks
}
