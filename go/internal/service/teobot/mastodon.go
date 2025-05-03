package teobot

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path"

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
	DataStoragePath    string
	MyAccountID        string
	LastNotificationID string
}

type state struct {
	LastNotificationID string `json:"lastNotificationId,omitempty"`
}

func NewMastodonTeobotFrontend(client *mastodon.Client, teobotService *service.TeobotService, historyService *history.HistoryService, textSplitService *textsplit.TextSplitService, dataStoragePath string) (*MastodonTeobotFrontend, error) {
	account, err := client.VerifyCredentials()
	if err != nil {
		return nil, err
	}

	m := &MastodonTeobotFrontend{
		Logger:            slog.New(slog.NewTextHandler(os.Stdout, nil)).With("component", "mastodon-teobot-frontend"),
		Client:            client,
		TeobotService:     teobotService,
		ThreadRecollector: &service.MastodonThreadRecollector{Client: client},
		HistoryService:    historyService,
		TextSplitService:  textSplitService,
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
