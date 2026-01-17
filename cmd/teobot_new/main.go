package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osak/teobot/internal/config"
	"github.com/osak/teobot/internal/db"
	"github.com/osak/teobot/internal/mastodon"
	"github.com/osak/teobot/internal/service/chatgpt"
	"github.com/osak/teobot/internal/service/teobot"
	pgxUUID "github.com/vgarvardt/pgx-google-uuid/v5"
)

func run() error {
	env := config.LoadEnvFromOS()
	chatGpt := chatgpt.New(env.ChatGPTAPIKey)

	pgxConfig, err := pgxpool.ParseConfig(env.DBConnectionString)
	if err != nil {
		return fmt.Errorf("failed to parse database connection string: %w", err)
	}
	pgxConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		pgxUUID.Register(conn.TypeMap())
		return nil
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), pgxConfig)
	if err != nil {
		return fmt.Errorf("failed to create connection pool: %w", err)
	}
	queries := db.New(pool)

	t := teobot.New(chatGpt, queries, pool)
	m := mastodon.NewClient(env.MastodonBaseURL, env.MastodonClientKey, env.MastodonClientSecret, env.MastodonAccessToken)
	tb, err := mastodon.NewTeobotBinding(t, m)
	if err != nil {
		return err
	}

	status, err := m.GetStatus(os.Args[1])
	if err != nil {
		return err
	}

	res, err := tb.GenerateResponse(context.Background(), status)
	if err != nil {
		return fmt.Errorf("failed to start talk with teobot: %w", err)
	}

	slog.Info(fmt.Sprintf("res: %s", res.Message.Text))
	return nil
}

func main() {
	slog.SetLogLoggerLevel(slog.LevelDebug)
	if err := run(); err != nil {
		panic(err)
	}
}
