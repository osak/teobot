package main

import (
	"context"
	"os"

	"github.com/osak/teobot/internal/service/chatgpt"
)

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	chatGpt := chatgpt.New(apiKey)
	ctx := context.Background()
}
