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
}

type RawObject struct {
	Type  string
	Bytes []byte
}

func (r *RawObject) GetType() string { return r.Type }

type OutputText struct {
	Text string `json:"text"`
}

func (o OutputText) MarshalJSON() ([]byte, error) {
	type Alias OutputText
	return json.Marshal(struct {
		Type string `json:"type"`
		Alias
	}{Type: "output_text", Alias: Alias(o)})
}

type Refusal struct {
	Refusal string `json:"refusal"`
}

func (o Refusal) MarshalJSON() ([]byte, error) {
	type Alias Refusal
	return json.Marshal(struct {
		Type string `json:"type"`
		Alias
	}{Type: "refusal", Alias: Alias(o)})
}

type Message struct {
	// Message content of the output. Possible types:
	//   * (For output) OutputText
	//   * (For output) Refusal
	//   * (For input) InputText
	//   * (For input) InputImage
	Content []OpenAIObjectWrapper `json:"content"`
	ID      string                `json:"id,omitempty"`
	Role    string                `json:"role,omitempty"`
	Status  string                `json:"status,omitempty"`
}

func (o Message) MarshalJSON() ([]byte, error) {
	type Alias Message
	return json.Marshal(struct {
		Type string `json:"type"`
		Alias
	}{Type: "message", Alias: Alias(o)})
}

type FunctionCall struct {
	// A JSON string of the arguments to pass to the function
	Arguments string `json:"arguments"`
	CallID    string `json:"call_id"`
	// The name of the function to run.
	Name   string `json:"name"`
	ID     string `json:"id"`
	Status string `json:"status"`
}

func (o FunctionCall) MarshalJSON() ([]byte, error) {
	type Alias FunctionCall
	return json.Marshal(struct {
		Type string `json:"type"`
		Alias
	}{Type: "function_call", Alias: Alias(o)})
}

type SummaryText struct {
	Text string `json:"text"`
}

func (o SummaryText) MarshalJSON() ([]byte, error) {
	type Alias SummaryText
	return json.Marshal(struct {
		Type string `json:"type"`
		Alias
	}{Type: "summary_text", Alias: Alias(o)})
}

type ReasoningText struct {
	Text string `json:"text"`
}

func (o ReasoningText) MarshalJSON() ([]byte, error) {
	type Alias ReasoningText
	return json.Marshal(struct {
		Type string `json:"type"`
		Alias
	}{Type: "reasoning_text", Alias: Alias(o)})
}

type Reasoning struct {
	ID      string          `json:"id"`
	Summary []SummaryText   `json:"summary"`
	Content []ReasoningText `json:"content"`
	Status  string          `json:"status"`
}

func (o Reasoning) MarshalJSON() ([]byte, error) {
	type Alias Reasoning
	return json.Marshal(struct {
		Type string `json:"type"`
		Alias
	}{Type: "reasoning", Alias: Alias(o)})
}

type CustomToolCall struct {
	CallID string `json:"call_id"`
	Input  string `json:"input"`
	// The name of the function to run.
	Name string `json:"name"`
	ID   string `json:"id"`
}

func (o CustomToolCall) MarshalJSON() ([]byte, error) {
	type Alias CustomToolCall
	return json.Marshal(struct {
		Type string `json:"type"`
		Alias
	}{Type: "custom_tool_call", Alias: Alias(o)})
}

// https://platform.openai.com/docs/api-reference/responses/object
type ResponsesResponse struct {
	ID        string `json:"id"`
	CreatedAt int64  `json:"created_at"`
	Status    string `json:"status"`
	Error     *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	IncompleteDetails *struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details,omitempty"`
	MaxOutputTokens *int   `json:"max_output_tokens,omitempty"`
	MaxToolCalls    *int   `json:"max_tool_calls,omitempty"`
	Model           string `json:"model"`
	// Output from the model. Possible types (currently implemented):
	//   * Message
	//   * FunctionCall
	//   * CustomToolCall
	Output             []OpenAIObjectWrapper `json:"output"`
	PreviousResponseID string                `json:"previous_response_id,omitempty"`
}

type InputText struct {
	Text string `json:"text"`
}

func (o InputText) MarshalJSON() ([]byte, error) {
	type Alias InputText
	return json.Marshal(struct {
		Type string `json:"type"`
		Alias
	}{Type: "input_text", Alias: Alias(o)})
}

type InputImage struct {
	Detail   string `json:"detail"`
	ImageUrl string `json:"image_url"`
}

func (o InputImage) MarshalJSON() ([]byte, error) {
	type Alias InputImage
	return json.Marshal(struct {
		Type string `json:"type"`
		Alias
	}{Type: "input_image", Alias: Alias(o)})
}

type ResponsesRequest struct {
	// Possible types:
	//   * Message
	Input []OpenAIObjectWrapper `json:"input"`
	Model string                `json:"model"`
}

type OpenAIObjectWrapper struct {
	Obj OpenAIObject
}

func Wrap(o OpenAIObject) OpenAIObjectWrapper {
	return OpenAIObjectWrapper{Obj: o}
}

func unmarshalAs[T any](obj *T, b []byte) (*T, error) {
	if err := json.Unmarshal(b, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func (o OpenAIObjectWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(o.Obj)
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

func (c *ChatGpt) Responses(ctx context.Context, request *ResponsesRequest) (*ResponsesResponse, error) {
	return doRequest[ResponsesResponse](c, ctx, "/responses", request)
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
	slog.Info(string(jsonPayload))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer func() { handleError("failed to close response body", resp.Body.Close()) }()

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
