package teobot

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osak/teobot/internal/db"
)

type Repository struct {
	queries *db.Queries
	pool    *pgxpool.Pool
	user    User
}

type txKey struct{}

func NewRepository(queries *db.Queries, pool *pgxpool.Pool, teobotUser User) Repository {
	return Repository{
		queries: queries,
		pool:    pool,
		user:    teobotUser,
	}
}

func txFromContext(ctx context.Context) (pgx.Tx, bool) {
	val, ok := ctx.Value(txKey{}).(pgx.Tx)
	return val, ok
}

func (r Repository) WithTransactionInLevel(ctx context.Context, isoLevel pgx.TxIsoLevel, fn func(ctx context.Context) error) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: isoLevel})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	newCtx := context.WithValue(ctx, txKey{}, tx)
	if err = fn(newCtx); err != nil {
		return err
	}
	return tx.Commit(ctx)
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

func (r Repository) LoadMessageByMastodonStatusID(ctx context.Context, statusID string) (*Message, error) {
	dbMessage, err := r.queries.FindChatgptMessageByMastodonStatusId(ctx, pgtype.Text{String: statusID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.hydrateChatGptMessage(ctx, dbMessage)
}

func (r Repository) CreateNewChatgptThread(ctx context.Context) (*db.ChatgptThread, error) {
	tx, ok := txFromContext(ctx)
	if !ok {
		var thread *db.ChatgptThread
		err := r.WithTransactionInLevel(ctx, pgx.Serializable, func(ctx context.Context) error {
			var err error
			thread, err = r.CreateNewChatgptThread(ctx)
			return err
		})
		return thread, err
	}
	qtx := r.queries.WithTx(tx)
	thread, err := qtx.CreateChatgptThread(ctx, uuid.Must(uuid.NewV7()))
	if err != nil {
		return nil, err
	}
	return &thread, err
}

func (r Repository) SaveMessages(ctx context.Context, threadID uuid.UUID, messages ...Message) error {
	tx, ok := txFromContext(ctx)
	if !ok {
		return r.WithTransactionInLevel(ctx, pgx.Serializable, func(ctx context.Context) error {
			return r.SaveMessages(ctx, threadID, messages...)
		})
	}
	qtx := r.queries.WithTx(tx)

	maxSeqNum, err := qtx.GetMaxSequenceNum(ctx, threadID)
	if err != nil {
		return err
	}
	for i, message := range messages {
		seqNum := int(maxSeqNum) + i + 1
		createMessageParams, err := r.buildCreateChatgptMessageParams(message)
		if err != nil {
			return fmt.Errorf("convert message %d: %w", i, err)
		}

		dbMessage, err := qtx.CreateChatgptMessage(ctx, *createMessageParams)
		if err != nil {
			return fmt.Errorf("insert message %d: %w", i, err)
		}
		err = qtx.CreateChatgptThreadRel(ctx, db.CreateChatgptThreadRelParams{
			ThreadID:         threadID,
			ChatgptMessageID: dbMessage.ID,
			SequenceNum:      int32(seqNum),
		})
		if err != nil {
			return fmt.Errorf("insert thread rel %d: %w", i, err)
		}

		// Save images
		for i, imageUrl := range message.ImageUrls {
			imageID := uuid.Must(uuid.NewV7())
			err := qtx.CreateImage(ctx, db.CreateImageParams{
				ID:  imageID,
				Url: imageUrl,
			})
			if err != nil {
				return fmt.Errorf("insert image %d: %w", i, err)
			}
			err = qtx.CreateChatGptMessageImageRel(ctx, db.CreateChatGptMessageImageRelParams{
				ChatgptMessageID: dbMessage.ID,
				ImageID:          imageID,
				Position:         int32(i),
			})
			if err != nil {
				return fmt.Errorf("insert image rel %d: %w", i, err)
			}
		}
	}
	return nil
}

func (r Repository) buildCreateChatgptMessageParams(message Message) (*db.CreateChatgptMessageParams, error) {
	id := message.ID
	if id == uuid.Nil {
		id = uuid.Must(uuid.NewV7())
	}
	messageType := "user_status"
	if message.User.Name == r.user.Name {
		messageType = "ai_response"
	}
	blob := dbMessageBlob{
		Name:    message.User.Name,
		Role:    messageType,
		Content: message.Text,
	}
	jsonBody, err := json.Marshal(blob, jsontext.EscapeForHTML(false))
	if err != nil {
		return nil, err
	}
	var mastodonStatusID string
	if meta, ok := message.RawMeta[ChannelTypeMastodon]; ok {
		if metaMap, ok := meta.(map[string]string); ok {
			mastodonStatusID = metaMap["status_id"]
		}
	}

	return &db.CreateChatgptMessageParams{
		ID:               id,
		MessageType:      messageType,
		JsonBody:         jsonBody,
		UserName:         message.User.Name,
		MastodonStatusID: pgtype.Text{String: mastodonStatusID, Valid: mastodonStatusID != ""},
		Timestamp:        pgtype.Timestamptz{Time: message.Timestamp, Valid: true},
		PrivacyLevel:     pgtype.Text{String: string(message.PrivacyLevel), Valid: true},
	}, nil
}

func (r Repository) hydrateChatGptMessage(ctx context.Context, cgMessage db.ChatgptMessage) (*Message, error) {
	arr := []db.ChatgptMessage{cgMessage}
	res, err := r.hydrateChatGptMessages(ctx, arr)
	if err != nil {
		return nil, err
	}
	return &res[0], nil
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
