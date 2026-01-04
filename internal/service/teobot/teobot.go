package teobot

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osak/teobot/internal/db"
	"github.com/osak/teobot/internal/service"
)

type Teobot struct {
	chatGpt *service.ChatGPT
	queries *db.Queries
	pool    *pgxpool.Pool
}
