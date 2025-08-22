package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type CreateResponsesRequest struct {
	Model           string      `json:"model"`
	Input           []Message   `json:"input"`
	MaxOutputTokens *int        `json:"max_output_tokens,omitempty"`
	Temperature     *float64    `json:"temperature,omitempty"`
	TopP            *float64    `json:"top_p,omitempty"`
	N               *int        `json:"n,omitempty"`
	Stream          *bool       `json:"stream,omitempty"`
	Stop            interface{} `json:"stop,omitempty"`
	Seed            *int        `json:"seed,omitempty"`
	Store           *bool       `json:"store,omitempty"`
	Tools           []Tool      `json:"tools,omitempty"`
	ToolChoice      interface{} `json:"tool_choice,omitempty"`
	TopLogprobs     *int        `json:"top_logprobs,omitempty"`
	Reasoning       *Reasoning  `json:"reasoning,omitempty"`
}

type MessageContent struct {
	Text string `json:"text"`
	Type string `json:"type"`
}

type Message struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
	Status  string      `json:"status"`
	Type    string      `json:"type"`
}

type ResponseFormat struct {
	Type string `json:"type"`
}

type Reasoning struct {
	Effort  *string `json:"effort,omitempty"`
	Summary *string `json:"summary,omitempty"`
}

type Tool struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

type Function struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type CreateResponsesResponse struct {
	ID                string    `json:"id"`
	Object            string    `json:"object"`
	Created           int64     `json:"created"`
	Model             string    `json:"model"`
	Choices           []Choice  `json:"choices"`
	Usage             Usage     `json:"usage"`
	Output            []Message `json:"output"`
	SystemFingerprint string    `json:"system_fingerprint,omitempty"`
}

type Choice struct {
	Index        int         `json:"index"`
	Message      Message     `json:"message"`
	Logprobs     interface{} `json:"logprobs"`
	FinishReason string      `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Param   string `json:"param,omitempty"`
		Code    string `json:"code,omitempty"`
	} `json:"error"`
}

type OpenAIClient struct {
	apiKey  string
	baseURL string
	client  *http.Client
	logger  *slog.Logger
}

func NewOpenAIClient(apiKey string) *OpenAIClient {
	return &OpenAIClient{
		apiKey:  apiKey,
		baseURL: "https://api.openai.com/v1",
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		logger: slog.Default(),
	}
}

func (o *OpenAIClient) WithLogger(logger *slog.Logger) *OpenAIClient {
	o.logger = logger
	return o
}

func (o *OpenAIClient) CreateResponses(ctx context.Context, req CreateResponsesRequest) (*CreateResponsesResponse, error) {
	logger := o.logger.With(
		slog.String("model", req.Model),
		slog.Int("messages_count", len(req.Input)),
	)
	logger.Debug("starting OpenAI API request")

	url := fmt.Sprintf("%s/responses", o.baseURL)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	logger.Debug("request marshaled", slog.Int("size_bytes", len(jsonData)))

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", o.apiKey))

	logger.Debug("sending request", slog.String("url", url))
	start := time.Now()

	resp, err := o.client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("request cancelled: %w", ctx.Err())
		}
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	logger.Debug("received response",
		slog.Int("status_code", resp.StatusCode),
		slog.Duration("elapsed", time.Since(start)))

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	logger.Debug("Body", slog.String("body", string(body)))

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("API error: %s (type: %s, code: %s)", errResp.Error.Message, errResp.Error.Type, errResp.Error.Code)
	}

	var result CreateResponsesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	logger.Info("OpenAI API request completed successfully",
		slog.String("response_id", result.ID),
		slog.String("model", result.Model),
		slog.Int("choices_count", len(result.Choices)),
		slog.Int("prompt_tokens", result.Usage.PromptTokens),
		slog.Int("completion_tokens", result.Usage.CompletionTokens),
		slog.Int("total_tokens", result.Usage.TotalTokens),
		slog.Duration("elapsed", time.Since(start)))

	return &result, nil
}
