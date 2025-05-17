package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/osak/teobot/internal/api"
)

// Message represents a chat message
type Message struct {
	Role       string      `json:"role"`
	Content    string      `json:"content"`
	Name       string      `json:"name,omitempty"`
	ToolCalls  []*ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

// Tool represents a function that can be called by ChatGPT
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction defines a function that can be called by ChatGPT
type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// ToolCall represents a call to a function from ChatGPT
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// ToolMessage represents a response from a tool call
type ToolMessage struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	ToolCallID string `json:"tool_call_id"`
}

// ChatResponse represents a response from the AI
type ChatResponse struct {
	Message   Message    `json:"message"`
	ImageURLs []string   `json:"imageUrls"`
	ToolCalls []ToolCall `json:"toolCalls,omitempty"`
}

// ChatContext holds the conversation context
type ChatContext struct {
	History       []Message
	SystemMessage string
	Tools         []Tool
}

// ChatGPT client for OpenAI's ChatGPT API
type ChatGPT struct {
	apiKey string
	client *http.Client
}

// NewChatGPT creates a new ChatGPT client
func NewChatGPT(apiKey string) *ChatGPT {
	return &ChatGPT{
		apiKey: apiKey,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// Chat sends a message to ChatGPT and gets a response
func (c *ChatGPT) Chat(messages []Message, tools []Tool) (*Message, error) {
	// Prepare the request payload
	reqBody := map[string]interface{}{
		"model":    "gpt-4o",
		"messages": messages,
	}

	// Add tools if provided
	if len(tools) > 0 {
		reqBody["tools"] = tools
	}

	reqData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	// Create the request
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(reqData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check for error response
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ChatGPT API error: %d - %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var chatResp struct {
		Choices []struct {
			Message *Message `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, err
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in ChatGPT response")
	}

	return chatResp.Choices[0].Message, nil
}

// DallE client for DALL-E image generation API
type DallE struct {
	apiKey string
	client *http.Client
}

// NewDallE creates a new DALL-E client
func NewDallE(apiKey string) *DallE {
	return &DallE{
		apiKey: apiKey,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// GenerateImage creates an image from a text prompt
func (d *DallE) GenerateImage(prompt string) (string, error) {
	// Prepare the request payload
	reqBody := map[string]interface{}{
		"model":  "dall-e-3",
		"prompt": prompt,
		"n":      1,
		"size":   "1024x1024",
	}

	reqData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	// Create the request
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/images/generations", bytes.NewBuffer(reqData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.apiKey)

	// Send the request
	resp, err := d.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Check for error response
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("DALL-E API error: %d - %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var imageResp struct {
		Data []struct {
			URL string `json:"url"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&imageResp); err != nil {
		return "", err
	}

	if len(imageResp.Data) == 0 {
		return "", fmt.Errorf("no images generated in DALL-E response")
	}

	return imageResp.Data[0].URL, nil
}

// TeobotService is the main service that coordinates bot operations
type TeobotService struct {
	chatGPT  *ChatGPT
	jmaAPI   api.JmaAPI
	dalleAPI *DallE
	logger   *slog.Logger
}

// NewTeobotService creates a new Teobot service
func NewTeobotService(chatGPT *ChatGPT, jmaAPI api.JmaAPI, dalleAPI *DallE) *TeobotService {
	return &TeobotService{
		chatGPT:  chatGPT,
		jmaAPI:   jmaAPI,
		dalleAPI: dalleAPI,
		logger:   slog.New(slog.NewTextHandler(os.Stdout, nil)).With("component", "teobot-service"),
	}
}

// NewChatContext creates a new chat context with default settings
func (t *TeobotService) NewChatContext(extraContext string) *ChatContext {
	systemMsg := fmt.Sprintf(`
	あなたは「ておくれロボ」という名前のチャットボットです。あなたはsocial.mikutter.hachune.netというMastodonサーバーで、teobotというアカウント名で活動しています。
あなたは無機質なロボットでありながら、おっちょこちょいで憎めない失敗することもある、総合的に見ると愛らしい存在として振る舞うことが期待されています。
返答を書く際には、以下のルールに従ってください。

- 文体は友達と話すようなくだけた感じにして、「です・ます」調は避けてください。
- 発言の語尾には必ず「ロボ」を付けてください。例えば「～あるロボ」「～だロボ」といった具合です。
- 返答は2～3文程度の短さであることが望ましいですが、質問に詳しく答える必要があるなど、必要であれば長くなっても構いません。ただし絶対に400文字は超えないでください。
- チャットの入力が@xxxという形式のメンションで始まっていることがありますが、これらは無視してください。

<extraContext>
%s
</extraContext>
`, extraContext)

	// Define the tools available to the model
	tools := []Tool{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_current_date_and_time",
				Description: "現在の日付と時刻を ISO8601 形式の文字列で返します。",
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_current_version",
				Description: "ておくれロボのバージョン情報を返します。",
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_area_code_mapping",
				Description: "都道府県名からエリアコードへのマッピングを返します。このエリアコードは天気予報APIで使うことができます。",
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_weather_forecast",
				Description: "直近3日の天気予報を返します。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"areaCode": map[string]any{
							"description": "天気予報を取得したい地域のエリアコード",
							"type":        "string",
						},
					},
					"required": []string{"areaCode"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "rand",
				Description: "整数の乱数を生成します。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"min": map[string]any{
							"description": "乱数の最小値",
							"type":        "integer",
							"default":     0,
						},
						"max": map[string]any{
							"description": "乱数の最大値",
							"type":        "integer",
							"default":     100,
						},
					},
				},
			},
		},
	}

	return &ChatContext{
		History:       []Message{},
		SystemMessage: systemMsg,
		Tools:         tools,
	}
}

// Chat sends a message to the AI and gets a response
func (t *TeobotService) Chat(context *ChatContext, userMessage Message) (*ChatResponse, error) {
	// Build the messages array
	var messages []Message

	// Add system message
	messages = append(messages, Message{
		Role:    "system",
		Content: context.SystemMessage,
	})

	// Add conversation history
	messages = append(messages, context.History...)

	// Add the user's message
	messages = append(messages, userMessage)

	// Initialize image URLs array
	imageURLs := []string{}

	// Get response from ChatGPT
	response, err := t.chatGPT.Chat(messages, context.Tools)
	if err != nil {
		return nil, err
	}
	messages = append(messages, *response)

	// Process tool calls if present
	if len(response.ToolCalls) > 0 {
		// Handle tool calls
		for i := 0; i < 10; i++ { // Limit iterations to prevent infinite loops
			if len(response.ToolCalls) == 0 {
				break
			}

			// Process each tool call
			var toolMessages []Message
			for _, toolCall := range response.ToolCalls {
				t.logger.Info("Processing tool call", "toolCall", toolCall)
				toolResult, err := t.executeToolCall(toolCall)
				if err != nil {
					toolResult = fmt.Sprintf("Error: %v", err)
				}
				t.logger.Info("Tool call result", "toolCall", toolCall, "result", toolResult)

				toolMessages = append(toolMessages, Message{
					Role:       "tool",
					Content:    toolResult,
					ToolCallID: toolCall.ID,
				})
			}

			// Add tool messages to history
			messages = append(messages, toolMessages...)

			// Get next response from ChatGPT
			nextResponse, err := t.chatGPT.Chat(messages, context.Tools)
			if err != nil {
				return nil, err
			}
			messages = append(messages, *nextResponse)

			response = nextResponse
			if len(response.ToolCalls) == 0 {
				break
			}
		}
	}

	// Check if the response contains image generation requests
	content := response.Content

	// Simple pattern matching for "[画像生成: prompt]"
	imageGenPattern := "[画像生成:"
	if idx := bytes.Index([]byte(content), []byte(imageGenPattern)); idx >= 0 {
		startIdx := idx + len(imageGenPattern)
		endIdx := bytes.IndexByte([]byte(content[startIdx:]), ']')

		if endIdx > 0 {
			prompt := content[startIdx : startIdx+endIdx]
			promptStr := string(bytes.TrimSpace([]byte(prompt)))

			imgURL, err := t.dalleAPI.GenerateImage(promptStr)
			if err == nil && imgURL != "" {
				imageURLs = append(imageURLs, imgURL)
			}
		}
	}

	return &ChatResponse{
		Message:   *response,
		ImageURLs: imageURLs,
	}, nil
}

// executeToolCall executes a tool call and returns the result
func (t *TeobotService) executeToolCall(toolCall *ToolCall) (string, error) {
	switch toolCall.Function.Name {
	case "get_current_date_and_time":
		// Return current date and time in ISO8601 format
		return time.Now().Format(time.RFC3339), nil

	case "get_current_version":
		// Return version information
		buildDate := time.Now().Format(time.RFC3339)
		versionInfo := map[string]string{
			"buildDate": buildDate,
		}
		jsonData, err := json.Marshal(versionInfo)
		if err != nil {
			return "", err
		}
		return string(jsonData), nil

	case "get_area_code_mapping":
		// Return the area code mapping
		areaCodeMap := t.jmaAPI.GetAreaCodeMap()
		jsonData, err := json.Marshal(areaCodeMap)
		if err != nil {
			return "", err
		}
		return string(jsonData), nil

	case "get_weather_forecast":
		// Parse arguments
		var args struct {
			AreaCode string `json:"areaCode"`
		}

		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		// Get weather forecast
		forecast, err := t.jmaAPI.GetWeatherForecast(api.AreaCode(args.AreaCode))
		if err != nil {
			return "", fmt.Errorf("failed to get weather forecast: %w", err)
		}

		// Convert to JSON
		jsonData, err := json.Marshal(forecast)
		if err != nil {
			return "", err
		}
		return string(jsonData), nil

	case "rand":
		// Generate a random number
		var args struct {
			Min int `json:"min"`
			Max int `json:"max"`
		}
		// Default values
		args.Min = 0
		args.Max = 100

		// Parse arguments if provided
		if toolCall.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
		}

		// Ensure min is less than max
		if args.Min > args.Max {
			args.Min, args.Max = args.Max, args.Min
		}

		// Generate random number
		randomNum := rand.Intn(args.Max-args.Min+1) + args.Min
		return fmt.Sprintf("%d", randomNum), nil

	default:
		return "", fmt.Errorf("unsupported function: %s", toolCall.Function.Name)
	}
}
