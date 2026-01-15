package teobot

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osak/teobot/internal/db"
	"github.com/osak/teobot/internal/service/chatgpt"
)

type Teobot struct {
	chatGpt *chatgpt.ChatGpt
	queries *db.Queries
	pool    *pgxpool.Pool
	// user is an DB entity that represents the bot itself.
	user *User
}
