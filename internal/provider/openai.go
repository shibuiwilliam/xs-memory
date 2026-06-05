package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// OpenAIEmbedder uses the OpenAI API for embeddings. See design §11.
type OpenAIEmbedder struct {
	apiKey string
	model  string
	dim    int
	client *http.Client
}

func NewOpenAIEmbedder(model string, dim int) *OpenAIEmbedder {
	return &OpenAIEmbedder{
		apiKey: os.Getenv("OPENAI_API_KEY"),
		model:  model,
		dim:    dim,
		client: &http.Client{},
	}
}

type openaiEmbedReq struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type openaiEmbedResp struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func (o *OpenAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if o.apiKey == "" {
		return nil, fmt.Errorf("openai: OPENAI_API_KEY not set")
	}

	body := openaiEmbedReq{Model: o.model, Input: texts}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/embeddings", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("openai: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai: status %d: %s", resp.StatusCode, respBody)
	}

	var result openaiEmbedResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("openai: decode: %w", err)
	}

	embeddings := make([][]float32, len(result.Data))
	for i, d := range result.Data {
		embeddings[i] = d.Embedding
	}
	return embeddings, nil
}

func (o *OpenAIEmbedder) Dim() int   { return o.dim }
func (o *OpenAIEmbedder) ID() string { return "openai:" + o.model }
