package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/osak/teobot/internal/service/chatgpt"
)

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	chatGpt := chatgpt.New(apiKey)
	ctx := context.Background()
	req := chatgpt.ResponsesRequest{
		Input: []chatgpt.OpenAIObject{
			&chatgpt.Message{
				Role: "user",
				Content: []chatgpt.MessageContent{
					chatgpt.InputText{
						Text: "Hello",
					},
				},
			},
		},
		Model: "gpt-5-mini",
	}
	res, err := chatGpt.Responses(ctx, &req)
	if err != nil {
		panic(err)
	}
	jsonRes, err := json.MarshalIndent(res, "", "\t")
	if err != nil {
		panic(err)
	}
	fmt.Printf("response: %v\n", string(jsonRes))
}
