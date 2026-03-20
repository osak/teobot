package mastodon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osak/teobot/internal/db"
	"github.com/osak/teobot/internal/metrics"
	"github.com/osak/teobot/internal/service/teobot"
	utext "github.com/osak/teobot/internal/text"
	"github.com/osak/teobot/internal/textsplit"
)

type TeobotBinding struct {
	teobot             *teobot.Teobot
	mastodon           Client
	myAccountID        string
	lastNotificationID string
	dataStoragePath    string
	queries            *db.Queries
	pool               *pgxpool.Pool
}

type state struct {
	LastNotificationID string `json:"lastNotificationId,omitempty"`
}

func NewTeobotBinding(t *teobot.Teobot, mastodon Client, dataStoragePath string, queries *db.Queries, pool *pgxpool.Pool) (*TeobotBinding, error) {
	acct, err := mastodon.VerifyCredentials()
	if err != nil {
		return nil, err
	}
	tb := TeobotBinding{
		teobot:             t,
		mastodon:           mastodon,
		myAccountID:        acct.Acct,
		lastNotificationID: "",
		dataStoragePath:    dataStoragePath,
		queries:            queries,
		pool:               pool,
	}
	if err := tb.LoadState(); err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}
	return &tb, nil
}

func (t *TeobotBinding) SaveState() error {
	state := &state{
		LastNotificationID: t.lastNotificationID,
	}
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	return os.WriteFile(path.Join(t.dataStoragePath, "state.json"), data, 0644)
}

func (t *TeobotBinding) LoadState() error {
	data, err := os.ReadFile(path.Join(t.dataStoragePath, "state.json"))
	if err != nil {
		return fmt.Errorf("read state file: %w", err)
	}
	var state state
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("unmarshal state: %w", err)
	}
	t.lastNotificationID = state.LastNotificationID
	return nil
}

func getPrivacyLevel(status *Status) teobot.PrivacyLevel {
	if status.Visibility == "direct" || status.Visibility == "private" {
		return teobot.PrivacyLevelPrivate
	}
	return teobot.PrivacyLevelPublic
}

func (t *TeobotBinding) convertToMessage(status *Status, user *teobot.User) (*teobot.Message, error) {
	timestamp, err := time.Parse(time.RFC3339, status.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse status.CreatedAt `%s`: %w", status.CreatedAt, err)
	}
	return &teobot.Message{
		Text:         NormalizeStatusContent(status),
		PrivacyLevel: getPrivacyLevel(status),
		User:         user,
		Timestamp:    timestamp,
		RawMeta: map[teobot.ChannelType]any{
			teobot.ChannelTypeMastodon: map[string]string{
				"status_id": status.ID,
			},
		},
	}, nil
}

func (t *TeobotBinding) resolveUser(ctx context.Context, account *Account) (*teobot.User, error) {
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	qtx := t.queries.WithTx(tx)
	row, err := qtx.GetUserByMastodonAccountId(ctx, account.Acct)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			user := teobot.User{
				ID:   uuid.Must(uuid.NewV7()),
				Name: account.Acct,
			}
			if err := t.teobot.CreateUser(ctx, &user); err != nil {
				return nil, fmt.Errorf("create user: %w", err)
			}
			err := qtx.CreateMastodonUserMapping(ctx, db.CreateMastodonUserMappingParams{
				MastodonAccountID: account.Acct,
				UserID:            user.ID,
			})
			if err != nil {
				return nil, fmt.Errorf("create user mapping: %w", err)
			}
			return &user, nil
		}

		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	user := teobot.User{
		ID:   row.ID,
		Name: row.Name,
	}
	return &user, nil
}

// ReconcileThread reconciles the Mastodon reply tree for a given status ID.
// The thread is built from the reply tree and saved to the database.
// The resulting thread does NOT include the message specified by statusId.
func (t *TeobotBinding) ReconcileThread(ctx context.Context, statusId string) (uuid.UUID, error) {
	slog.Info("ReconcileThread", slog.String("statusId", statusId))

	// Get the Mastodon reply tree
	tree, err := t.mastodon.GetReplyTree(statusId)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to get reply tree for statusId=%s: %w", statusId, err)
	}

	// Build messages history from the thread
	var messages []teobot.Message
	for _, status := range tree.Ancestors {
		user, err := t.resolveUser(ctx, &status.Account)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to resolve user %v for status %v: %v", status.Account.ID, status.ID, err))
			continue
		}
		message, err := t.convertToMessage(status, user)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to convert status %s to Message: %v", status.ID, err))
			continue
		}
		messages = append(messages, *message)
	}

	thread := &teobot.Thread{
		Messages: messages,
	}
	threadID, err := t.teobot.ImportThread(ctx, thread)
	if err != nil {
		return uuid.Nil, err
	}
	return threadID, nil
}

func (t *TeobotBinding) postReply(ctx context.Context, response *teobot.TalkResponse, replyTo *Status) (*Status, error) {
	// Remove <responseMeta>...</responseMeta> section from the response
	// NOTE: Current code assumes that this section will only appear in the end of the response.
	content := response.Message.Text
	if idx := strings.LastIndex(content, "<responseMeta>"); idx >= 0 {
		// Extract meta before stripping, so we can emit metrics
		endIdx := strings.LastIndex(content, "</responseMeta>")
		if endIdx > idx {
			metaStr := content[idx+len("<responseMeta>") : endIdx]
			var meta map[string]any
			if err := json.Unmarshal([]byte(strings.TrimSpace(metaStr)), &meta); err == nil {
				if s, ok := meta["seriousness"].(string); ok {
					switch strings.ToUpper(s) {
					case "LOW":
						metrics.RecordCount1("teobot/seriousness/LOW")
					case "MED":
						metrics.RecordCount1("teobot/seriousness/MED")
					case "HIGH":
						metrics.RecordCount1("teobot/seriousness/HIGH")
					default:
						metrics.RecordCount1("teobot/seriousness/UNKNOWN")
					}
				}
			} else {
				// Non-fatal, but record an error metric
				metrics.NoticeError(ctx, fmt.Errorf("failed to parse responseMeta: %w", err))
			}
		}
		content = content[:idx]
	}

	// Sanitize response message to avoid accidentally mention random people
	sanitized := utext.ReplaceAll(utext.New(content), "@", "@ ")

	// Split utext into chunks to fit to character limit of Mastodon
	texts := []*utext.Text{sanitized}
	if sanitized.Len() > 450 {
		ts := textsplit.NewTextSplitService(nil)
		newTexts, err := ts.SplitText(sanitized, 450)
		if err != nil {
			return nil, fmt.Errorf("split text: %w", err)
		}
		texts = newTexts
	}

	// Post messages in the reply chain
	var lastStatus *Status
	inReplyToId := replyTo.ID
	for _, text := range texts {
		body := fmt.Sprintf("@%s %s", replyTo.Account.Acct, text)
		newStatus, err := t.mastodon.PostStatus(body, &PostStatusOpt{
			ReplyToID:  inReplyToId,
			Visibility: replyTo.Visibility,
		})
		if err != nil {
			wrapped := fmt.Errorf("post status: %w", err)
			metrics.NoticeError(ctx, wrapped)
			return nil, wrapped
		}

		inReplyToId = newStatus.ID
		lastStatus = newStatus
	}

	return lastStatus, nil
}

func (t *TeobotBinding) GenerateResponse(ctx context.Context, status *Status) (*teobot.TalkResponse, error) {
	// If `status` is a reply to any existing Mastodon thread, we use that fact for reconstructing the conversaion thread
	// for the bot.
	// If the status is very beginning of a conversation, replyToMessageID will remain null.
	var replyToMessageID uuid.UUID
	if status.InReplyToID != "" {
		slog.Debug(fmt.Sprintf("InReplyToID: %s", status.InReplyToID))
		// Find the teobot Message to reply to
		replyToMessage, err := t.teobot.FindMessageByMastodonStatusID(ctx, status.InReplyToID)
		if err != nil {
			return nil, err
		}
		if replyToMessage == nil {
			// The message is not a part of any known conversation threads - needs reconciliation before proceed.
			_, err := t.ReconcileThread(ctx, status.ID)
			if err != nil {
				return nil, fmt.Errorf("reconcile %s: %w", status.InReplyToID, err)
			}

			// Populate replyToMessage again. This time the message must have been stored in the DB.
			replyToMessage, err = t.teobot.FindMessageByMastodonStatusID(ctx, status.InReplyToID)
			if err != nil {
				return nil, err
			}
			if replyToMessage == nil {
				return nil, fmt.Errorf("post reconciliation")
			}
		}
		replyToMessageID = replyToMessage.ID
	}

	user, err := t.resolveUser(ctx, &status.Account)
	if err != nil {
		return nil, err
	}
	slog.Info(fmt.Sprintf("resolved user: %v", *user))

	// Convert the posted status to teobot Message
	message, err := t.convertToMessage(status, user)
	if err != nil {
		return nil, err
	}

	// Generate reply
	res, err := t.teobot.Talk(ctx, replyToMessageID, message)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (t *TeobotBinding) ReplyAndPost(ctx context.Context, status *Status) error {
	res, err := t.GenerateResponse(ctx, status)
	if err != nil {
		return err
	}

	// Post the reply message to Mastodon
	lastStatus, err := t.postReply(ctx, res, status)
	if err != nil {
		return err
	}

	if err := t.teobot.RecordMastodonStatusID(ctx, res.Message.ID, lastStatus.ID); err != nil {
		return err
	}

	return nil
}

// Run goes through the newly arrived replies and respond to them
func (t *TeobotBinding) Run(ctx context.Context) error {
	// Get the latest replies
	replies, err := t.mastodon.GetAllNotifications(&GetAllNotificationsOpt{
		SinceID: t.lastNotificationID,
		Types:   []string{"mention"},
	})
	if err != nil {
		return fmt.Errorf("failed to get notifications: %w", err)
	}

	// Process each reply
	for _, reply := range replies {
		slog.Info("Processing reply", slog.String("reply", reply.ID), slog.String("status", reply.Status.Content))
		err := t.ReplyAndPost(ctx, reply.Status)
		if err != nil {
			slog.Error("Failed to reply to the status", "error", err)
			continue
		}

		// Update the last notification ID
		if t.lastNotificationID < reply.ID {
			t.lastNotificationID = reply.ID
		}
		if err := t.SaveState(); err != nil {
			return fmt.Errorf("save state: %w", err)
		}
	}

	return nil
}
