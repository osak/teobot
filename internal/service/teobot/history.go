package teobot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/osak/teobot/internal/db"
)

type dbMessageBlob struct {
	Name    string `json:"name"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

func splitMeta(content string) (string, string) {
	startTag := strings.Index(content, "<metadata>")
	endTag := strings.Index(content, "</metadata>")
	if startTag != -1 && endTag != -1 {
		start := startTag + len("<metadata>")
		return content[0:startTag], content[start:endTag]
	}
	return content, ""
}

func parseChatGptMessage(chatgptMessage db.ChatgptMessage) (Message, error) {
	dec := json.NewDecoder(bytes.NewBuffer(chatgptMessage.JsonBody))
	var dbMessage dbMessageBlob
	if err := dec.Decode(&dbMessage); err != nil {
		return Message{}, fmt.Errorf("failed to parse message %v: %w", chatgptMessage.ID, err)
	}

	text, rawMeta := splitMeta(dbMessage.Content)
	msg := Message{
		ID: chatgptMessage.ID,
		User: &User{
			Name: chatgptMessage.UserName,
		},
		Text: text,
		RawMeta: map[ChannelType]any{
			ChannelTypeMastodon: rawMeta,
		},
	}
	return msg, nil
}

func (t *Teobot) loadConversationHistory(ctx Context, userName string) ([]*Thread, error) {
	threadIds, err := t.queries.GetRecentThreadIdsByUserName(ctx.RunCtx, db.GetRecentThreadIdsByUserNameParams{
		UserName: userName,
		Limit:    50,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch recent thread ids for user `%s`: %v", userName, err)
	}

	threads := make([]*Thread, 0, len(threadIds))
	for _, threadId := range threadIds {
		rows, err := t.queries.GetFullChatgptMessagesByThreadId(ctx.RunCtx, threadId)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch thread %s: %v", threadId, err)
		}

		messages := make([]*Message, 0, len(rows))
		for _, row := range rows {
			msg, err := parseChatGptMessage(row)
			if err != nil {
				return nil, fmt.Errorf("failed to parse message for thread %s: %v", threadId, err)
			}
			messages = append(messages, &msg)
		}
		thread := Thread{
			ID:       threadId,
			Messages: messages,
		}
		threads = append(threads, &thread)
	}
	return threads, nil
}

func (t *Teobot) loadRecentMessages(ctx Context) ([]*Message, error) {
	rows, err := t.queries.GetRecentFullChatgptMessages(ctx.RunCtx, 50)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch recent thread ids: %v", err)
	}
	messages := make([]*Message, 0, len(rows))
	for _, row := range rows {
		message, err := parseChatGptMessage(row)
		if err != nil {
			return nil, fmt.Errorf("failed to parse message %v: %w", row.ID, err)
		}
		messages = append(messages, &message)
	}
	return messages, nil
}
