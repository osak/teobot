package teobot

import (
	"context"
	"encoding/json/v2"
	"fmt"

	"github.com/google/uuid"
	"github.com/osak/teobot/internal/db"
)

type Repository struct {
	queries *db.Queries
}

func NewRepository(queries *db.Queries) Repository {
	return Repository{
		queries: queries,
	}
}

func (r Repository) LoadRecentMessages(ctx context.Context, n int32) ([]Message, error) {
	rows, err := r.queries.GetRecentFullChatgptMessages(ctx, n)
	if err != nil {
		return nil, fmt.Errorf("fetch recent thread ids: %v", err)
	}
	return r.hydrateChatGptMessages(ctx, rows)
}

func (r Repository) LoadMessagesInThread(ctx context.Context, threadID uuid.UUID) ([]Message, error) {
	chatGptMessages, err := r.queries.GetFullChatgptMessagesByThreadId(ctx, threadID)
	if err != nil {
		return nil, fmt.Errorf("load chatgpt messages: %w", err)
	}
	return r.hydrateChatGptMessages(ctx, chatGptMessages)
}

func (r Repository) hydrateChatGptMessages(ctx context.Context, cgMessages []db.ChatgptMessage) ([]Message, error) {
	mesIds := make([]uuid.UUID, len(cgMessages))
	for i, mes := range cgMessages {
		mesIds[i] = mes.ID
	}

	images, err := r.queries.GetImagesByChatGptMessageIds(ctx, mesIds)
	if err != nil {
		return nil, fmt.Errorf("load images: %w", err)
	}

	imagesByMessageID := make(map[uuid.UUID][]string)
	for _, image := range images {
		mesID := image.ChatgptMessageID
		imagesByMessageID[mesID] = append(imagesByMessageID[mesID], image.Url)
	}

	messages := make([]Message, len(cgMessages))
	for i, cgMes := range cgMessages {
		images := imagesByMessageID[cgMes.ID]
		messages[i], err = parseChatGptMessage(cgMes, images)
		if err != nil {
			return nil, fmt.Errorf("parse chatgpt message: %w", err)
		}
	}

	return messages, nil
}

func parseChatGptMessage(chatgptMessage db.ChatgptMessage, images []string) (Message, error) {
	var dbMessage dbMessageBlob
	if err := json.Unmarshal(chatgptMessage.JsonBody, &dbMessage); err != nil {
		return Message{}, fmt.Errorf("failed to parse message %v: %w", chatgptMessage.ID, err)
	}

	text, rawMeta := splitMeta(dbMessage.Content)
	msg := Message{
		ID: chatgptMessage.ID,
		User: &User{
			Name: chatgptMessage.UserName,
		},
		Text:      text,
		Timestamp: chatgptMessage.Timestamp.Time,
		ImageUrls: images,
		RawMeta: map[ChannelType]any{
			ChannelTypeMastodon: rawMeta,
		},
	}
	return msg, nil
}
