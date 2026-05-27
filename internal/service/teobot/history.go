package teobot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/osak/teobot/internal/db"
)

var (
	ErrNoThread = errors.New("no ongoing thread")
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
		slog.Debug("Loading thread", "threadId", threadId)
		messages, err := t.repository.LoadMessagesInThread(ctx, threadId)
		if err != nil {
			return nil, fmt.Errorf("load thread %v: %w", threadId, err)
		}

		thread := Thread{
			ID:       threadId,
			Messages: messages,
		}
		threads = append(threads, &thread)
	}
	return threads, nil
}

func (t *Teobot) loadRecentMessages(ctx context.Context) ([]Message, error) {
	return t.repository.LoadRecentMessages(ctx, 50)
}

// ImportThread stores given thread in the DB.
// Returns newly created thread id.
func (t *Teobot) ImportThread(ctx context.Context, thread *Thread) (uuid.UUID, error) {
	if thread.ID != uuid.Nil {
		return uuid.Nil, fmt.Errorf("thread id must not be set, got %s", thread.ID)
	}

	var threadID uuid.UUID
	err := t.repository.WithTransactionInLevel(ctx, pgx.Serializable, func(ctx context.Context) error {
		dbThread, err := t.repository.CreateNewChatgptThread(ctx)
		if err != nil {
			return fmt.Errorf("create thread: %w", err)
		}
		threadID = dbThread.ID

		// Save messages
		if err := t.repository.SaveMessages(ctx, threadID, thread.Messages...); err != nil {
			return fmt.Errorf("save messages: %w", err)
		}
		return nil
	})
	if err != nil {
		return uuid.Nil, err
	}

	return threadID, nil
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
	return t.repository.LoadMessageByMastodonStatusID(ctx, statusID)
}

// FindOngoingThreadIDByMessageID finds the id of ongoing thread that has ended with the given message.
// If there is no such thread, returns ErrNoThread.
func (t *Teobot) FindOngoingThreadIDByMessageID(ctx context.Context, messageID uuid.UUID) (uuid.UUID, error) {
	threadRels, err := t.queries.GetChatgptThreadRels(ctx, messageID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("get thread rels: %w", err)
	}

	// Find a thread ending with the given message
	for _, threadRel := range threadRels {
		threadID := threadRel.ThreadID
		messages, err := t.queries.GetFullChatgptMessagesByThreadId(ctx, threadID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("get thread messages (id=%s): %w", threadID, err)
		}
		if messages[len(messages)-1].ID == messageID {
			return threadID, nil
		}
	}

	return uuid.Nil, ErrNoThread
}

// RecordMastodonStatusID updates the DB to record in which Mastodon status the teobot response has been posted.
// Ideally it's the binding's responsibility to maintain the association between mastodon status ID and teobot message ID,
// but the DB has already been designed so the teobot message table directly keeps tracking of mastodon status...
func (t *Teobot) RecordMastodonStatusID(ctx context.Context, messageID uuid.UUID, mastodonStatusID string) error {
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := t.queries.WithTx(tx)

	err = qtx.UpdateMastodonStatusId(ctx, db.UpdateMastodonStatusIdParams{
		ID:               messageID,
		MastodonStatusID: pgtype.Text{String: mastodonStatusID, Valid: true},
	})
	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}
