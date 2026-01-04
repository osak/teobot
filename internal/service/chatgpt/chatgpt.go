package chatgpt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type ChatGpt struct {
	apiKey  string
	apiBase string
}

func New(apiKey string) *ChatGpt {
	return &ChatGpt{
		apiKey:  apiKey,
		apiBase: "https://api.openai.com/v1",
	}
}

func (c *ChatGpt) sendRequest(ctx context.Context, path string, payload any) (any, error) {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.apiBase+path, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
}
