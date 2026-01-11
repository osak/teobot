package chatgpt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

type ChatGpt struct {
	apiKey  string
	apiBase string
}

type OpenAIObject interface {
	GetType() string
}

type RawObject struct {
	Type  string
	Bytes []byte
}

func (r *RawObject) GetType() string { return r.Type }

type OutputText struct {
	Text string `json:"text"`
}

func (o *OutputText) GetType() string { return "output_text" }

type Refusal struct {
	Refusal string `json:"refusal"`
}

func (r *Refusal) GetType() string { return "refusal" }

type Message struct {
	// Message content of the output. Possible types:
	//   * (For output) OutputText
	//   * (For output) Refusal
	//   * (For input) InputText
	//   * (For input) InputImage
	Content []OpenAIObjectWrapper `json:"content"`
	ID      string                `json:"id"`
	Role    string                `json:"role"`
	Status  string                `json:"status"`
}

func (m *Message) GetType() string { return "message" }

type FunctionCall struct {
	// A JSON string of the arguments to pass to the function
	Arguments string `json:"arguments"`
	CallID    string `json:"call_id"`
	// The name of the function to run.
	Name   string `json:"name"`
	ID     string `json:"id"`
	Status string `json:"status"`
}

func (f *FunctionCall) GetType() string { return "function_call" }

type SummaryText struct {
	Text string `json:"text"`
}

func (s *SummaryText) GetType() string { return "summary_text" }

type ReasoningText struct {
	Text string `json:"text"`
}

func (r *ReasoningText) GetType() string { return "reasoning_text" }

type Reasoning struct {
	ID      string          `json:"id"`
	Summary []SummaryText   `json:"summary"`
	Content []ReasoningText `json:"content"`
	Status  string          `json:"status"`
}

func (r *Reasoning) GetType() string { return "reasoning" }

type CustomToolCall struct {
	CallID string `json:"call_id"`
	Input  string `json:"input"`
	// The name of the function to run.
	Name string `json:"name"`
	ID   string `json:"id"`
}

func (c *CustomToolCall) GetType() string { return "custom_tool_call" }

// https://platform.openai.com/docs/api-reference/responses/object
type ResponsesResponse struct {
	ID        string `json:"id"`
	CreatedAt int64  `json:"created_at"`
	Status    string `json:"status"`
	Error     struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	IncompleteDetails struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
	MaxOutputTokens int    `json:"max_output_tokens"`
	MaxToolCalls    int    `json:"max_tool_calls"`
	Model           string `json:"model"`
	// Output from the model. Possible types (currently implemented):
	//   * Message
	//   * FunctionCall
	//   * CustomToolCall
	Output             []OpenAIObjectWrapper `json:"output"`
	PreviousResponseID string                `json:"previous_response_id"`
}

type InputText struct {
	Text string `json:"text"`
}

func (i *InputText) GetType() string { return "input_text" }

type InputImage struct {
	Detail   string `json:"detail"`
	ImageUrl string `json:"image_url"`
}

func (i *InputImage) GetType() string { return "input_image" }

type ResponsesRequest struct {
	// Possible types:
	//   * Message
	Input []OpenAIObjectWrapper `json:"input"`
	Type  string                `json:"type"`
}

type OpenAIObjectWrapper struct {
	Obj OpenAIObject
}

func unmarshalAs[T any](obj *T, b []byte) (*T, error) {
	if err := json.Unmarshal(b, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func (o *OpenAIObjectWrapper) UnmarshalJSON(b []byte) error {
	var typeTag struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(b, &typeTag); err != nil {
		return fmt.Errorf("expected OpenAI typed struct but `type` is not found : %w", err)
	}

	var err error
	switch typeTag.Type {
	case "custom_tool_call":
		o.Obj, err = unmarshalAs(&CustomToolCall{}, b)
	case "function_call":
		o.Obj, err = unmarshalAs(&FunctionCall{}, b)
	case "message":
		o.Obj, err = unmarshalAs(&Message{}, b)
	case "output_text":
		o.Obj, err = unmarshalAs(&OutputText{}, b)
	case "reasoning":
		o.Obj, err = unmarshalAs(&Reasoning{}, b)
	case "reasoning_text":
		o.Obj, err = unmarshalAs(&ReasoningText{}, b)
	case "refusal":
		o.Obj, err = unmarshalAs(&Refusal{}, b)
	case "summary_text":
		o.Obj, err = unmarshalAs(&SummaryText{}, b)
	default:
		o.Obj = &RawObject{
			Type:  typeTag.Type,
			Bytes: b,
		}
	}
	if err != nil {
		return fmt.Errorf("failed to unmarshal OpenAI object of type %s: %w", typeTag.Type, err)
	}
	return nil
}

func handleError(msg string, err error) {
	if err != nil {
		slog.Error(msg, "error", err.Error())
	}
}

func New(apiKey string) *ChatGpt {
	return &ChatGpt{
		apiKey:  apiKey,
		apiBase: "https://api.openai.com/v1",
	}
}

func (c *ChatGpt) Responses(ctx context.Context) (*ResponsesResponse, error) {
	return nil, nil
}

func doRequest[T any](c *ChatGpt, ctx context.Context, path string, payload any) (*T, error) {
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
	defer handleError("failed to close response body", resp.Body.Close())

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ChatGPT API error: %d (%s)", resp.StatusCode, string(body))
	}

	var t T
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}
	return &t, nil
}
