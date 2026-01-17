package teobot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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

func (t *Teobot) loadConversationHistory(ctx context.Context, userName string) ([]*Thread, error) {
	threadIds, err := t.queries.GetRecentThreadIdsByUserName(ctx, db.GetRecentThreadIdsByUserNameParams{
		UserName: userName,
		Limit:    50,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch recent thread ids for user `%s`: %v", userName, err)
	}

	threads := make([]*Thread, 0, len(threadIds))
	for _, threadId := range threadIds {
		rows, err := t.queries.GetFullChatgptMessagesByThreadId(ctx, threadId)
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

func (t *Teobot) loadRecentMessages(ctx context.Context) ([]*Message, error) {
	rows, err := t.queries.GetRecentFullChatgptMessages(ctx, 50)
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

// ImportThread stores given thread in the DB.
// Returns newly created thread id.
func (t *Teobot) ImportThread(ctx context.Context, thread *Thread) (uuid.UUID, error) {
	if thread.ID != uuid.Nil {
		return uuid.Nil, fmt.Errorf("thread id must not be set, got %s", thread.ID)
	}

	// Start transaction
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)
	qtx := t.queries.WithTx(tx)

	// Create a new thread entry
	dbThread, err := qtx.CreateChatgptThread(ctx, uuid.Must(uuid.NewV7()))
	if err != nil {
		return uuid.Nil, fmt.Errorf("create thread: %w", err)
	}

	// Save messages and associate with the thread
	for i, message := range thread.Messages {
		// Create and save message entry
		if message.ID != uuid.Nil {
			return uuid.Nil, fmt.Errorf("message id must not be set, got %s", message.ID)
		}
		// TODO fix structure
		blob := dbMessageBlob{
			Role:    "user",
			Content: message.Text,
		}
		if message.User.Name == "teobot" {
			blob.Role = "assistant"
		}
		jsonBody, err := json.Marshal(blob)
		if err != nil {
			return uuid.Nil, err
		}

		params := db.CreateChatgptMessageParams{
			ID:           uuid.Must(uuid.NewV7()),
			MessageType:  "mastodon_status",
			JsonBody:     jsonBody,
			UserName:     message.User.Name,
			Timestamp:    pgtype.Timestamptz{Time: message.Timestamp, Valid: true},
			PrivacyLevel: pgtype.Text{String: string(message.PrivacyLevel), Valid: true},
		}
		mastodonMeta, ok := message.RawMeta[ChannelTypeMastodon]
		if ok {
			id := mastodonMeta.(map[string]string)["status_id"]
			params.MastodonStatusID = pgtype.Text{String: id, Valid: true}
		}

		dbMessage, err := qtx.CreateChatgptMessage(ctx, params)
		if err != nil {
			slog.Error("Failed to save message to the database", "error", err)
			continue
		}

		// Associate the message with the thread
		if err := qtx.CreateChatgptThreadRel(ctx, db.CreateChatgptThreadRelParams{
			ThreadID:         dbThread.ID,
			ChatgptMessageID: dbMessage.ID,
			SequenceNum:      int32(i + 1),
		}); err != nil {
			slog.Error("Failed to create a chatgpt thread relationship", "error", err)
			continue
		}
		slog.Info("Created thread relationship", "threadID", dbThread.ID, "messageID", dbMessage.ID, "sequenceNum", i+1)
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}

	return dbThread.ID, nil
}

// ForkThread creates a new thread forked at specified message. The new thread shares reply tree with the original
// up to the given message, but the following messages will be kept separately.
// Returns the new thread ID.
func (t *Teobot) ForkThread(ctx context.Context, messageID uuid.UUID) (uuid.UUID, error) {
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)
	qtx := t.queries.WithTx(tx)

	// Identify the thread that the message belongs to
	orgRel, err := qtx.GetChatgptThreadRels(ctx, messageID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("get thread rel (messageID %s): %w", messageID, err)
	}
	if len(orgRel) == 0 {
		return uuid.Nil, fmt.Errorf("message %s does not belong to any threads", messageID)
	}
	orgThreadID := orgRel[0].ThreadID
	slog.Info("Forking thread", "threadID", orgThreadID)

	// Create a new thread to fork into
	thread, err := qtx.CreateChatgptThread(ctx, uuid.Must(uuid.NewV7()))
	if err != nil {
		return uuid.Nil, fmt.Errorf("forking thread: %w", err)
	}

	// Create thread rels to associate messages in reply tree to the new thread
	rows, err := qtx.GetFullChatgptMessagesByThreadId(ctx, orgThreadID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("get thread messages (threadID %s): %w", orgThreadID, err)
	}
	for i, row := range rows {
		params := db.CreateChatgptThreadRelParams{
			ThreadID:         thread.ID,
			ChatgptMessageID: row.ID,
			SequenceNum:      int32(i + 1),
		}
		if err := qtx.CreateChatgptThreadRel(ctx, params); err != nil {
			return uuid.Nil, fmt.Errorf("create thread rel: %w", err)
		}

		// Replies made after the given message are not included in the forked thread
		if row.ID == messageID {
			break
		}
	}

	// Commit
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return thread.ID, nil
}

// FindMessageByMastodonStatusID searches DB for the message entry corresponding to the given Mastodon status ID.
// Returns the parsed Message object, or nil if not found.
func (t *Teobot) FindMessageByMastodonStatusID(ctx context.Context, statusID string) (*Message, error) {
	dbMessage, err := t.queries.FindChatgptMessageByMastodonStatusId(ctx, pgtype.Text{String: statusID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	message, err := parseChatGptMessage(dbMessage)
	if err != nil {
		return nil, err
	}
	return &message, nil
}
