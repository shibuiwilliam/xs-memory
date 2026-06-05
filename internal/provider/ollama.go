package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// OllamaEmbedder uses the Ollama API for embeddings. See design §11.
type OllamaEmbedder struct {
	baseURL string
	model   string
	dim     int
	client  *http.Client
}

// NewOllamaEmbedder creates an Ollama embedder.
func NewOllamaEmbedder(baseURL, model string, dim int) *OllamaEmbedder {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &OllamaEmbedder{
		baseURL: baseURL,
		model:   model,
		dim:     dim,
		client:  &http.Client{},
	}
}

type ollamaEmbedReq struct {
	Model string `json:"model"`
	Input any    `json:"input"`
}

type ollamaEmbedResp struct {
	Embeddings [][]float32 `json:"embeddings"`
}

func (o *OllamaEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	body := ollamaEmbedReq{Model: o.model, Input: texts}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("ollama: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/api/embed", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("ollama: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama: status %d: %s", resp.StatusCode, respBody)
	}

	var result ollamaEmbedResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ollama: decode: %w", err)
	}

	return result.Embeddings, nil
}

func (o *OllamaEmbedder) Dim() int   { return o.dim }
func (o *OllamaEmbedder) ID() string { return "ollama:" + o.model }

// OllamaLLM uses the Ollama API for text completion. See design §11.
type OllamaLLM struct {
	baseURL string
	model   string
	client  *http.Client
}

func NewOllamaLLM(baseURL, model string) *OllamaLLM {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &OllamaLLM{
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{},
	}
}

type ollamaGenReq struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	System string `json:"system,omitempty"`
	Stream bool   `json:"stream"`
	Format string `json:"format,omitempty"`
}

type ollamaGenResp struct {
	Response string `json:"response"`
}

func (o *OllamaLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	body := ollamaGenReq{
		Model:  o.model,
		Prompt: req.Prompt,
		System: req.System,
		Stream: false,
	}
	if req.JSON {
		body.Format = "json"
	}

	data, err := json.Marshal(body)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("ollama: marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/api/generate", bytes.NewReader(data))
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("ollama: new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("ollama: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return CompletionResponse{}, fmt.Errorf("ollama: status %d: %s", resp.StatusCode, respBody)
	}

	var result ollamaGenResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return CompletionResponse{}, fmt.Errorf("ollama: decode: %w", err)
	}

	return CompletionResponse{Text: result.Response}, nil
}
