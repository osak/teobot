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
	db                 *sql.DB
	DataStoragePath    string
	MyAccountID        string
	LastNotificationID string
}

type state struct {
	LastNotificationID string `json:"lastNotificationId,omitempty"`
}

func NewMastodonTeobotFrontend(client *mastodon.Client, teobotService *service.TeobotService, historyService *history.HistoryService, textSplitService *textsplit.TextSplitService, sqldb *sql.DB, dataStoragePath string) (*MastodonTeobotFrontend, error) {
	account, err := client.VerifyCredentials()
	if err != nil {
		return nil, err
	}

	queries := db.New(sqldb)
	m := &MastodonTeobotFrontend{
		Logger:            slog.New(slog.NewTextHandler(os.Stdout, nil)).With("component", "mastodon-teobot-frontend"),
		Client:            client,
		TeobotService:     teobotService,
		ThreadRecollector: &service.MastodonThreadRecollector{Client: client},
		HistoryService:    historyService,
		TextSplitService:  textSplitService,
		db:                sqldb,
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

var ErrNoCurrentThread = errors.New("no current thread found")

func findThreadsContainingMessageId(ctx context.Context, queries *db.Queries, messageID uint64) (map[uint64][]db.ChatgptThreadsRel, error) {
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
	threads := make(map[uint64][]db.ChatgptThreadsRel)
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
func (m *MastodonTeobotFrontend) GetOrCreateCurrentThreadId(ctx context.Context, status *mastodon.Status) (uint64, error) {
	if status.InReplyToID == "" {
		// This is not a reply, so we need to create a new thread
		return m.createNewThread(ctx)
	}

	// Look up the status being replied in the database
	mes, err := m.queries.FindChatgptMessageByMastodonStatusId(ctx, sql.NullString{String: status.InReplyToID, Valid: true})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// There is a Mastodon reply tree but we don't know the history. Needs reconciliation.
			return m.ReconcileThread(ctx, status.ID)
		} else {
			return 0, fmt.Errorf("failed to find chatgpt message by mastodon status ID: %w", err)
		}
	}

	// List the existing threads containing the message ID
	threads, err := findThreadsContainingMessageId(ctx, m.queries, mes.ID)
	if err != nil {
		return 0, fmt.Errorf("failed to find threads containing status ID: %w", err)
	}
	if len(threads) == 0 {
		// Unlikely, but if we don't find any threads, we need to create a new one
		return m.createNewThread(ctx)
	}

	// Check if the message is the last one in any thread
	var anyThreadId uint64
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

func (m *MastodonTeobotFrontend) createNewThread(ctx context.Context) (uint64, error) {
	if err := m.queries.CreateChatgptThread(ctx); err != nil {
		return 0, fmt.Errorf("failed to create a new chatgpt thread: %w", err)
	}
	threadId, err := m.queries.GetLastInsertedChatgptThreadId(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get last inserted thread ID: %w", err)
	}
	return uint64(threadId), nil
}

func (m *MastodonTeobotFrontend) cloneThread(ctx context.Context, baseThread []db.ChatgptThreadsRel, messageId uint64) (uint64, error) {
	// Create a new thread
	newThreadId, err := m.createNewThread(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to create a new thread: %w", err)
	}
	m.Logger.Info("Cloning thread into", "newThreadID", newThreadId)

	tx, err := m.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()
	qtx := m.queries.WithTx(tx)

	// Clone the base thread up to the message ID
	for _, rel := range baseThread {
		// Create a new relationship in the new thread
		if err := qtx.CreateChatgptThreadRel(ctx, db.CreateChatgptThreadRelParams{
			ThreadID:         newThreadId,
			ChatgptMessageID: rel.ChatgptMessageID,
			SequenceNum:      rel.SequenceNum,
		}); err != nil {
			return 0, fmt.Errorf("failed to create a new thread relationship: %w", err)
		}
		m.Logger.Info("Cloned thread relationship", "threadID", newThreadId, "messageID", rel.ChatgptMessageID, "sequenceNum", rel.SequenceNum)
		if rel.ChatgptMessageID == messageId {
			break
		}
	}

	tx.Commit()
	return newThreadId, nil
}

// ReconcileThread reconciles the Mastodon reply tree for a given status ID.
// The thread is built from the reply tree and saved to the database.
// The resulting thread does NOT include the message specified by statusId.
func (m *MastodonTeobotFrontend) ReconcileThread(ctx context.Context, statusId string) (uint64, error) {
	m.Logger.Info("ReconcileThread", "statusId", statusId)

	// Get the Mastodon reply tree
	tree, err := m.Client.GetReplyTree(statusId)
	if err != nil {
		return 0, fmt.Errorf("failed to get reply tree for statusId=%s: %w", statusId, err)
	}

	tx, err := m.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

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
		params := db.CreateChatgptMessageParams{
			MessageType:      "chatgpt",
			JsonBody:         string(jsonBody),
			UserName:         message.Name,
			MastodonStatusID: sql.NullString{String: status.ID, Valid: true},
		}
		if err := qtx.CreateChatgptMessage(ctx, params); err != nil {
			m.Logger.Error("Failed to save message to the database", "error", err)
			continue
		}
		lastChatgptMessage, err := qtx.GetLastInsertedChatgptMessage(ctx)
		if err != nil {
			m.Logger.Error("Failed to get last inserted message", "error", err)
			continue
		}
		chatgptMessages = append(chatgptMessages, &lastChatgptMessage)
		m.Logger.Info("Saved message to the database", "statusID", status.ID, "messageID", lastChatgptMessage.ID)
	}

	// Create a new thread
	if err := qtx.CreateChatgptThread(ctx); err != nil {
		m.Logger.Error("Failed to create a chatgpt thread in the database", "error", err)
		return 0, err
	}
	threadID, err := qtx.GetLastInsertedChatgptThreadId(ctx)
	if err != nil {
		m.Logger.Error("Failed to get last inserted thread ID", "error", err)
		return 0, err
	}
	m.Logger.Info("Created thread", "threadID", threadID)

	// Create relationships between messages and the thread
	for i, message := range chatgptMessages {
		if err := qtx.CreateChatgptThreadRel(ctx, db.CreateChatgptThreadRelParams{
			ThreadID:         uint64(threadID),
			ChatgptMessageID: message.ID,
			SequenceNum:      int32(i + 1),
		}); err != nil {
			m.Logger.Error("Failed to create a chatgpt thread relationship", "error", err)
			continue
		}
		m.Logger.Info("Created thread relationship", "threadID", threadID, "messageID", message.ID, "sequenceNum", i+1)
	}

	tx.Commit()
	return uint64(threadID), nil
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

func (m *MastodonTeobotFrontend) BuildThreadHistory(acct string) (*history.ThreadHistory, error) {
	// Get all history files for this account
	threads, err := m.HistoryService.GetHistory(acct, 10)
	if err != nil {
		return nil, err
	}

	// Convert to Mastodon thread history
	var mastodonThreads [][]*mastodon.PartialStatus
	for _, thread := range threads {
		if containsDirectVisibility(thread) {
			continue
		}
		var mastodonThread []*mastodon.PartialStatus
		for _, status := range thread {
			mastodonThread = append(mastodonThread, &mastodon.PartialStatus{
				Account:   status.Account,
				Content:   mastodon.NormalizeStatusContent(status),
				CreatedAt: status.CreatedAt,
			})
		}
		mastodonThreads = append(mastodonThreads, mastodonThread)
	}

	return &history.ThreadHistory{Threads: mastodonThreads}, nil
}

func (m *MastodonTeobotFrontend) ReplyTo(status *mastodon.Status) (*service.ChatResponse, error) {
	// Get the thread context
	thread, err := m.ThreadRecollector.RecollectThread(status.ID)
	if err != nil {
		m.Logger.Warn("Failed to recollect thread", "error", err)
		thread = make([]*mastodon.Status, 0)
	}

	threadHistory, err := m.BuildThreadHistory(status.Account.Acct)
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
		"</threads>",
		status.Account.Acct,
		string(threadHistoryJSON))
	m.Logger.Info("Extra context", "extraContext", extraContext)

	// Build a new chat context
	ctx := m.TeobotService.NewChatContext(extraContext)
	for _, s := range thread {
		ctx.History = append(ctx.History, m.convertToMessage(s))
	}

	// Process the thread
	return m.TeobotService.Chat(ctx, *m.convertToMessage(status))
}

func (m *MastodonTeobotFrontend) ReplyToStatusId(id string) (*service.ChatResponse, error) {
	status, err := m.Client.GetStatus(id)
	if err != nil {
		return nil, err
	}
	return m.ReplyTo(status)
}

// Run goes through the newly arrived replies and repond to them
func (m *MastodonTeobotFrontend) Run() error {
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
		response, err := m.ReplyTo(reply.Status)
		if err != nil {
			return fmt.Errorf("failed to generate reply to the status: %w", err)
		}
		m.Logger.Info("Reply generated.", "result", response.Message.Content)

		sanitized := text.ReplaceAll(text.New(response.Message.Content), "@", "@ ")
		texts := []*text.Text{sanitized}
		if sanitized.Len() > 450 {
			texts, err = m.TextSplitService.SplitText(sanitized, 450)
			if err != nil {
				return fmt.Errorf("failed to split text: %w", err)
			}
		}

		inReplyToId := reply.Status.ID
		for _, text := range texts {
			body := fmt.Sprintf("@%s %s", reply.Status.Account.Acct, text)
			newStatus, err := m.Client.PostStatus(body, &mastodon.PostStatusOpt{
				ReplyToID:  inReplyToId,
				Visibility: reply.Status.Visibility,
			})
			if err != nil {
				return fmt.Errorf("failed to post status: %w", err)
			}
			inReplyToId = newStatus.ID
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
