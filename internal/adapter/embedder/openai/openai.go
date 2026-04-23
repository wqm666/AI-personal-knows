package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Embedder struct {
	baseURL string
	apiKey  string
	model   string
	dim     int
	client  *http.Client
}

func New(baseURL, apiKey, model string) *Embedder {
	dim := 1536
	if model == "text-embedding-3-large" {
		dim = 3072
	}
	return &Embedder{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		dim:     dim,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (e *Embedder) Dimension() int { return e.dim }

func (e *Embedder) Embed(ctx context.Context, text string) ([]float64, error) {
	body := map[string]any{
		"model": e.model,
		"input": text,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := e.baseURL + "/v1/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		return nil, fmt.Errorf("embedding request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result embeddingResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse embedding response: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}

	return result.Data[0].Embedding, nil
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}
