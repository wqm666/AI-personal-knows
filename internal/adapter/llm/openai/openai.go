package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/personal-know/internal/port"
)

type Client struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

func New(baseURL, apiKey, model string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) Chat(ctx context.Context, messages []port.LLMMessage) (string, error) {
	return c.doChat(ctx, messages, false)
}

func (c *Client) ChatJSON(ctx context.Context, messages []port.LLMMessage, schema any) (string, error) {
	return c.doChat(ctx, messages, true)
}

func (c *Client) doChat(ctx context.Context, messages []port.LLMMessage, jsonMode bool) (string, error) {
	apiMessages := make([]apiMessage, len(messages))
	for i, m := range messages {
		apiMessages[i] = apiMessage{Role: m.Role, Content: m.Content}
	}

	body := map[string]any{
		"model":    c.model,
		"messages": apiMessages,
	}
	if jsonMode {
		body["response_format"] = map[string]string{"type": "json_object"}
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	url := c.baseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		return "", fmt.Errorf("llm request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result chatResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse llm response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty llm response")
	}

	return result.Choices[0].Message.Content, nil
}

type apiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}
