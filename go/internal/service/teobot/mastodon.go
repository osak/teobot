package teobot

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osak/teobot/internal/db"
	"github.com/osak/teobot/internal/history"
	"github.com/osak/teobot/internal/mastodon"
	"github.com/osak/teobot/internal/service"
	"github.com/osak/teobot/internal/text"
	"github.com/osak/teobot/internal/textsplit"
)

type MastodonTeobotFrontend struct {
	Logger             *slog.Logger
	Client             *mastodon.Client
	TeobotService      *service.TeobotService
	ThreadRecollector  *service.MastodonThreadRecollector
	HistoryService     *history.HistoryService
	TextSplitService   *textsplit.TextSplitService
	queries            *db.Queries
	pool               *pgxpool.Pool
	DataStoragePath    string
	MyAccountID        string
	LastNotificationID string
}

type state struct {
	LastNotificationID string `json:"lastNotificationId,omitempty"`
}

func NewMastodonTeobotFrontend(client *mastodon.Client, teobotService *service.TeobotService, historyService *history.HistoryService, textSplitService *textsplit.TextSplitService, pool *pgxpool.Pool, dataStoragePath string) (*MastodonTeobotFrontend, error) {
	account, err := client.VerifyCredentials()
	if err != nil {
		return nil, err
	}

	queries := db.New(pool)
	m := &MastodonTeobotFrontend{
		Logger:            slog.New(slog.NewTextHandler(os.Stdout, nil)).With("component", "mastodon-teobot-frontend"),
		Client:            client,
		TeobotService:     teobotService,
		ThreadRecollector: &service.MastodonThreadRecollector{Client: client},
		HistoryService:    historyService,
		TextSplitService:  textSplitService,
		pool:              pool,
		queries:           queries,
		DataStoragePath:   dataStoragePath,
		MyAccountID:       account.ID,
	}
	if err := m.LoadState(); err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}
	return m, nil
}

func (m *MastodonTeobotFrontend) SaveState() error {
	state := &state{
		LastNotificationID: m.LastNotificationID,
	}
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}
	return os.WriteFile(path.Join(m.DataStoragePath, "state.json"), data, 0644)
}

func (m *MastodonTeobotFrontend) LoadState() error {
	data, err := os.ReadFile(path.Join(m.DataStoragePath, "state.json"))
	if err != nil {
		return fmt.Errorf("failed to read state file: %w", err)
	}
	var state state
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}
	m.LastNotificationID = state.LastNotificationID
	return nil
}

func (m *MastodonTeobotFrontend) convertToMessage(status *mastodon.Status) *service.Message {
	message := &service.Message{
		Role:    "user",
		Content: mastodon.NormalizeStatusContent(status),
		Name:    status.Account.Acct,
	}
	if status.Account.ID == m.MyAccountID {
		message.Role = "assistant"
	}
	return message
}

func containsDirectVisibility(statuses []*mastodon.Status) bool {
	for _, s := range statuses {
		if s.Visibility == "direct" {
			return true
		}
	}
	return false
}

func getPrivacyLevel(status *mastodon.Status) string {
	if status.Visibility == "direct" || status.Visibility == "private" {
		return "private"
	}
	return "public"
}

var ErrNoCurrentThread = errors.New("no current thread found")

func findThreadsContainingMessageId(ctx context.Context, queries *db.Queries, messageID uuid.UUID) (map[uuid.UUID][]db.ChatgptThreadsRel, error) {
	// Get the thread ID from the message
	threadRel, err := queries.GetChatgptThreadRels(ctx, messageID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		} else {
			return nil, fmt.Errorf("failed to find chatgpt thread relationship: %w", err)
		}
	}

	// Group the rels by thread ID
	threads := make(map[uuid.UUID][]db.ChatgptThreadsRel)
	for _, rel := range threadRel {
		if _, ok := threads[rel.ThreadID]; ok {
			threads[rel.ThreadID] = append(threads[rel.ThreadID], rel)
		} else {
			threads[rel.ThreadID] = []db.ChatgptThreadsRel{rel}
		}
	}
	return threads, nil
}

// GetOrCreateCurrentThread retrieves the current thread for a given status ID, or create one if not exist in DB.
// "Current thread" means the thread that contains the given status ID as the last message.
func (m *MastodonTeobotFrontend) GetOrCreateCurrentThreadId(ctx context.Context, status *mastodon.Status) (uuid.UUID, error) {
	if status.InReplyToID == "" {
		// This is not a reply, so we need to create a new thread
		return m.createNewThread(ctx)
	}

	// Look up the status being replied in the database
	mes, err := m.queries.FindChatgptMessageByMastodonStatusId(ctx, pgtype.Text{String: status.InReplyToID, Valid: true})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// There is a Mastodon reply tree but we don't know the history. Needs reconciliation.
			return m.ReconcileThread(ctx, status.ID)
		} else {
			return uuid.Nil, fmt.Errorf("failed to find chatgpt message by mastodon status ID: %w", err)
		}
	}

	// List the existing threads containing the message ID
	threads, err := findThreadsContainingMessageId(ctx, m.queries, mes.ID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to find threads containing status ID: %w", err)
	}
	if len(threads) == 0 {
		// Unlikely, but if we don't find any threads, we need to create a new one
		return m.createNewThread(ctx)
	}

	// Check if the message is the last one in any thread
	var anyThreadId uuid.UUID
	for threadId, rels := range threads {
		if rels[len(rels)-1].ChatgptMessageID == mes.ID {
			// This is the current thread
			return threadId, nil
		}
		anyThreadId = threadId
	}

	// Now, a thread is being forked from existing one.
	// We need to create a new thread by duplicating the existing one up to `mes`.
	baseThread := threads[anyThreadId]
	return m.cloneThread(ctx, baseThread, mes.ID)
}

func (m *MastodonTeobotFrontend) createNewThread(ctx context.Context) (uuid.UUID, error) {
	thread, err := m.queries.CreateChatgptThread(ctx, uuid.Must(uuid.NewV7()))
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to create a new chatgpt thread: %w", err)
	}
	return thread.ID, nil
}

func (m *MastodonTeobotFrontend) cloneThread(ctx context.Context, baseThread []db.ChatgptThreadsRel, messageId uuid.UUID) (uuid.UUID, error) {
	// Create a new thread
	newThreadId, err := m.createNewThread(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to create a new thread: %w", err)
	}
	m.Logger.Info("Cloning thread into", "newThreadID", newThreadId)

	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	qtx := m.queries.WithTx(tx)

	// Clone the base thread up to the message ID
	for _, rel := range baseThread {
		// Create a new relationship in the new thread
		if err := qtx.CreateChatgptThreadRel(ctx, db.CreateChatgptThreadRelParams{
			ThreadID:         newThreadId,
			ChatgptMessageID: rel.ChatgptMessageID,
			SequenceNum:      rel.SequenceNum,
		}); err != nil {
			return uuid.Nil, fmt.Errorf("failed to create a new thread relationship: %w", err)
		}
		m.Logger.Info("Cloned thread relationship", "threadID", newThreadId, "messageID", rel.ChatgptMessageID, "sequenceNum", rel.SequenceNum)
		if rel.ChatgptMessageID == messageId {
			break
		}
	}

	tx.Commit(ctx)
	return newThreadId, nil
}

// ReconcileThread reconciles the Mastodon reply tree for a given status ID.
// The thread is built from the reply tree and saved to the database.
// The resulting thread does NOT include the message specified by statusId.
func (m *MastodonTeobotFrontend) ReconcileThread(ctx context.Context, statusId string) (uuid.UUID, error) {
	m.Logger.Info("ReconcileThread", "statusId", statusId)

	// Get the Mastodon reply tree
	tree, err := m.Client.GetReplyTree(statusId)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to get reply tree for statusId=%s: %w", statusId, err)
	}

	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := m.queries.WithTx(tx)

	// Build Chatgpt messages history
	var chatgptMessages []*db.ChatgptMessage
	for _, status := range tree.Ancestors {
		message := m.convertToMessage(status)
		jsonBody, err := json.Marshal(message)
		if err != nil {
			m.Logger.Error("Failed to marshal message", "error", err)
			continue
		}
		posted, err := time.Parse(time.RFC3339, status.CreatedAt)
		if err != nil {
			m.Logger.Error("Failed to parse status createdAt", "error", err)
			continue
		}
		params := db.CreateChatgptMessageParams{
			ID:               uuid.Must(uuid.NewV7()),
			MessageType:      "mastodon_status",
			JsonBody:         jsonBody,
			UserName:         message.Name,
			MastodonStatusID: pgtype.Text{String: status.ID, Valid: true},
			Timestamp:        pgtype.Timestamptz{Time: posted, Valid: true},
			PrivacyLevel:     pgtype.Text{String: getPrivacyLevel(status), Valid: true},
		}
		chatgptMessage, err := qtx.CreateChatgptMessage(ctx, params)
		if err != nil {
			m.Logger.Error("Failed to save message to the database", "error", err)
			continue
		}
		chatgptMessages = append(chatgptMessages, &chatgptMessage)
		m.Logger.Info("Saved message to the database", "statusID", status.ID, "messageID", chatgptMessage.ID)
	}

	// Create a new thread
	thread, err := qtx.CreateChatgptThread(ctx, uuid.Must(uuid.NewV7()))
	if err != nil {
		m.Logger.Error("Failed to create a chatgpt thread in the database", "error", err)
		return uuid.Nil, err
	}
	m.Logger.Info("Created thread", "threadID", thread.ID)

	// Create relationships between messages and the thread
	for i, message := range chatgptMessages {
		if err := qtx.CreateChatgptThreadRel(ctx, db.CreateChatgptThreadRelParams{
			ThreadID:         thread.ID,
			ChatgptMessageID: message.ID,
			SequenceNum:      int32(i + 1),
		}); err != nil {
			m.Logger.Error("Failed to create a chatgpt thread relationship", "error", err)
			continue
		}
		m.Logger.Info("Created thread relationship", "threadID", thread.ID, "messageID", message.ID, "sequenceNum", i+1)
	}

	tx.Commit(ctx)
	return thread.ID, nil
}

func (m *MastodonTeobotFrontend) RestoreThread(ctx context.Context, threadId uuid.UUID) ([]service.Message, error) {
	// Get the thread messages from the database
	messages, err := m.queries.GetChatgptMessagesByThreadId(ctx, threadId)
	if err != nil {
		return nil, fmt.Errorf("failed to get chatgpt thread messages: %w", err)
	}
	// Convert to service.Message
	var result []service.Message
	for _, message := range messages {
		var msg service.Message
		if err := json.Unmarshal(message.JsonBody, &msg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal message: %w", err)
		}
		result = append(result, msg)
	}
	return result, nil
}

func (m *MastodonTeobotFrontend) ProcessReply(ctx context.Context, statusId string) error {
	status, err := m.Client.GetStatus(statusId)
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}
	threadId, err := m.GetOrCreateCurrentThreadId(ctx, status)
	if err != nil {
		return fmt.Errorf("failed to get or create current thread: %w", err)
	}
	m.Logger.Info("Processing reply", "statusId", status.ID, "threadId", threadId)
	return nil
}

func (m *MastodonTeobotFrontend) BuildThreadHistory(ctx context.Context, acct string) ([][]service.Message, error) {
	// Grab the recent thread IDs based on 100 recent recorded messages from the user
	threadIds, err := m.queries.GetRecentThreadIdsByUserName(ctx, db.GetRecentThreadIdsByUserNameParams{UserName: acct, Limit: 100})
	if err != nil {
		return nil, fmt.Errorf("failed to get recent thread IDs: %w", err)
	}

	// Limit the number of threads to 10
	if len(threadIds) > 10 {
		threadIds = threadIds[:10]
	}

	// Get the thread messages from the database
	chatHistories := make([][]service.Message, len(threadIds))
	for i, threadId := range threadIds {
		messages, err := m.queries.GetChatgptMessagesByThreadId(ctx, threadId)
		if err != nil {
			return nil, fmt.Errorf("failed to get chatgpt thread messages: %w", err)
		}
		// Convert to service.Message
		history := make([]service.Message, len(messages))
		for j, message := range messages {
			var msg service.Message
			if err := json.Unmarshal(message.JsonBody, &msg); err != nil {
				return nil, fmt.Errorf("failed to unmarshal message: %w", err)
			}
			history[j] = msg
		}
		chatHistories[i] = history
	}

	return chatHistories, nil
}

func (m *MastodonTeobotFrontend) SaveMessageInThread(ctx context.Context, message service.Message, messageType string, threadId uuid.UUID, privacyLevel string, statusId string) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	qtx := m.queries.WithTx(tx)

	seqNum, err := qtx.GetMaxSequenceNum(ctx, threadId)
	if err != nil {
		return fmt.Errorf("failed to get sequence number: %w", err)
	}

	mastodonStatusID := pgtype.Text{}
	if statusId != "" {
		mastodonStatusID = pgtype.Text{String: statusId, Valid: true}
	}

	jsonBody, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	chatgptMessage, err := qtx.CreateChatgptMessage(ctx, db.CreateChatgptMessageParams{
		ID:               uuid.Must(uuid.NewV7()),
		MessageType:      messageType,
		JsonBody:         jsonBody,
		UserName:         message.Name,
		MastodonStatusID: mastodonStatusID,
		Timestamp:        pgtype.Timestamptz{Time: time.Now(), Valid: true},
		PrivacyLevel:     pgtype.Text{String: privacyLevel, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to save chatgpt message: %w", err)
	}

	if err := qtx.CreateChatgptThreadRel(ctx, db.CreateChatgptThreadRelParams{
		ThreadID:         threadId,
		ChatgptMessageID: chatgptMessage.ID,
		SequenceNum:      seqNum + 1,
	}); err != nil {
		return fmt.Errorf("failed to create chatgpt thread relationship: %w", err)
	}

	tx.Commit(ctx)
	return nil
}

type ReplyToResult struct {
	ThreadId        uuid.UUID
	UserMessage     service.Message
	RepsonseMessage service.Message
}

func (m *MastodonTeobotFrontend) ReplyTo(ctx context.Context, status *mastodon.Status) (*ReplyToResult, error) {
	threadHistory, err := m.BuildThreadHistory(ctx, status.Account.Acct)
	if err != nil {
		return nil, fmt.Errorf("failed to build thread history: %w", err)
	}
	threadHistoryJSON, err := json.Marshal(threadHistory)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal thread history: %w", err)
	}

	extraContext := fmt.Sprintf(""+
		"あなたが最近 %s と交わした会話スレッドは以下の通りです。\n"+
		"<threads>\n"+
		"%s\n"+
		"</threads>\n",
		status.Account.Acct,
		string(threadHistoryJSON))

	recentMessagesRaw, err := m.queries.GetRecentChatgptMessages(ctx, 50)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent chatgpt messages: %w", err)
	}
	recentMessages := make([]service.Message, len(recentMessagesRaw))
	for i, message := range recentMessagesRaw {
		var msg service.Message
		if err := json.Unmarshal(message.JsonBody, &msg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal message: %w", err)
		}
		recentMessages[i] = msg
	}
	recentMessagesJSON, err := json.Marshal(recentMessages)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal recent messages: %w", err)
	}
	extraContext += fmt.Sprintf(""+
		"\nあなたが直近他のユーザーと交わしたやりとりは以下の通りです。\n"+
		"<recent_messages>\n"+
		"%s\n"+
		"</recent_messages>\n",
		string(recentMessagesJSON))
	m.Logger.Info("Extra context", "extraContext", extraContext)

	// Build a new chat context
	threadId, err := m.GetOrCreateCurrentThreadId(ctx, status)
	if err != nil {
		return nil, fmt.Errorf("failed to get thread ID: %w", err)
	}
	prevMessages, err := m.RestoreThread(ctx, threadId)
	if err != nil {
		return nil, fmt.Errorf("failed to restore thread: %w", err)
	}
	chatCtx := m.TeobotService.NewChatContext(extraContext)
	for _, s := range prevMessages {
		chatCtx.History = append(chatCtx.History, s)
	}
	m.Logger.Info("Chat context", "history", chatCtx.History)

	// Process the thread
	userMessage := m.convertToMessage(status)
	response, err := m.TeobotService.Chat(chatCtx, *userMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to process chat: %w", err)
	}

	result := &ReplyToResult{
		ThreadId:        threadId,
		UserMessage:     *userMessage,
		RepsonseMessage: response.Message,
	}
	return result, nil
}

func (m *MastodonTeobotFrontend) ReplyToStatusId(ctx context.Context, id string) (*ReplyToResult, error) {
	status, err := m.Client.GetStatus(id)
	if err != nil {
		return nil, err
	}
	return m.ReplyTo(ctx, status)
}

func (m *MastodonTeobotFrontend) ReplyAndPost(ctx context.Context, status *mastodon.Status) error {
	result, err := m.ReplyTo(ctx, status)
	if err != nil {
		return fmt.Errorf("failed to generate reply to the status: %w", err)
	}
	m.Logger.Info("Reply generated.", "result", result.RepsonseMessage.Content)

	sanitized := text.ReplaceAll(text.New(result.RepsonseMessage.Content), "@", "@ ")
	texts := []*text.Text{sanitized}
	if sanitized.Len() > 450 {
		texts, err = m.TextSplitService.SplitText(sanitized, 450)
		if err != nil {
			return fmt.Errorf("failed to split text: %w", err)
		}
	}

	privacyLevel := getPrivacyLevel(status)
	if err := m.SaveMessageInThread(ctx, result.UserMessage, "user_status", result.ThreadId, privacyLevel, status.ID); err != nil {
		return fmt.Errorf("failed to save user message: %w", err)
	}

	inReplyToId := status.ID
	for i, text := range texts {
		body := fmt.Sprintf("@%s %s", status.Account.Acct, text)
		newStatus, err := m.Client.PostStatus(body, &mastodon.PostStatusOpt{
			ReplyToID:  inReplyToId,
			Visibility: status.Visibility,
		})
		if err != nil {
			return fmt.Errorf("failed to post status: %w", err)
		}

		if i == 0 {
			if err := m.SaveMessageInThread(ctx, result.RepsonseMessage, "ai_response", result.ThreadId, privacyLevel, newStatus.ID); err != nil {
				return fmt.Errorf("failed to save assistant message: %w", err)
			}
		} else {
			// Insert pseudo message to the database for looking up the thread
			pseudoMessage := m.convertToMessage(newStatus)
			if err := m.SaveMessageInThread(ctx, *pseudoMessage, "pseudo_message", result.ThreadId, privacyLevel, newStatus.ID); err != nil {
				return fmt.Errorf("failed to save pseudo message: %w", err)
			}
		}

		inReplyToId = newStatus.ID
	}

	return nil
}

// Run goes through the newly arrived replies and repond to them
func (m *MastodonTeobotFrontend) Run(ctx context.Context) error {
	// Get the latest replies
	replies, err := m.Client.GetAllNotifications(&mastodon.GetAllNotificationsOpt{
		SinceID: m.LastNotificationID,
		Types:   []string{"mention"},
	})
	if err != nil {
		return fmt.Errorf("failed to get notifications: %w", err)
	}

	// Process each reply
	for _, reply := range replies {
		m.Logger.Info("Processing reply", "reply", reply.ID, "status", reply.Status.Content)
		err := m.ReplyAndPost(ctx, reply.Status)
		if err != nil {
			m.Logger.Error("Failed to reply to the status", "error", err)
			continue
		}

		// Update the last notification ID
		if m.LastNotificationID < reply.ID {
			m.LastNotificationID = reply.ID
		}
		if err := m.SaveState(); err != nil {
			return fmt.Errorf("failed to save state: %w", err)
		}
	}

	return nil
}
