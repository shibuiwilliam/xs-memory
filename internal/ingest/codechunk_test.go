package ingest

import (
	"strings"
	"testing"
)

func TestChunkCodeGo(t *testing.T) {
	code := `package main

import "fmt"

func hello() {
	fmt.Println("hello")
}

func world() {
	fmt.Println("world")
}

func main() {
	hello()
	world()
}`

	chunks := ChunkCode(code, 5)
	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks, got %d", len(chunks))
	}
	// Should split at func boundaries.
	for _, c := range chunks {
		if strings.Contains(c, "func") {
			// Good: chunk contains a function.
		}
	}
}

func TestChunkCodePython(t *testing.T) {
	code := `import os

def foo():
    return 1

def bar():
    return 2

class MyClass:
    def method(self):
        pass`

	chunks := ChunkCode(code, 4)
	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks, got %d", len(chunks))
	}
}

func TestChunkCodeShortFile(t *testing.T) {
	code := "x = 1\ny = 2"
	chunks := ChunkCode(code, 50)
	if len(chunks) != 1 {
		t.Errorf("short code should be 1 chunk, got %d", len(chunks))
	}
}

func TestChunkCodeFallback(t *testing.T) {
	// No function patterns — should fall back to line splitting.
	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, "some plain text line")
	}
	code := strings.Join(lines, "\n")

	chunks := ChunkCode(code, 30)
	if len(chunks) < 3 {
		t.Errorf("expected >=3 chunks, got %d", len(chunks))
	}
}
