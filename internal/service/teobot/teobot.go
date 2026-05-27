package teobot

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osak/teobot/internal/db"
	"github.com/osak/teobot/internal/service/chatgpt"
)

type Teobot struct {
	chatGpt    *chatgpt.ChatGpt
	queries    *db.Queries
	pool       *pgxpool.Pool
	repository *Repository
	// user is an DB entity that represents the bot itself.
	user *User
}

func New(ctx context.Context, chatGpt *chatgpt.ChatGpt, queries *db.Queries, pool *pgxpool.Pool) (*Teobot, error) {
	// TODO: Rewrite to avoid depending on Mastodon
	row, err := queries.GetUserByMastodonAccountId(ctx, "teobot")
	if err != nil {
		return nil, fmt.Errorf("find teobot user: %w", err)
	}
	user := User{
		ID:   row.ID,
		Name: row.Name,
	}
	repository := NewRepository(queries, pool, user)
	return &Teobot{
		chatGpt:    chatGpt,
		queries:    queries,
		pool:       pool,
		user:       &user,
		repository: &repository,
	}, nil
}

func (t *Teobot) CreateUser(ctx context.Context, user *User) error {
	return t.queries.CreateUser(ctx, db.CreateUserParams{
		ID:   user.ID,
		Name: user.Name,
	})
}
