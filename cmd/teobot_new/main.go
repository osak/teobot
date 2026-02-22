package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

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
	// Configure global logger
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

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

	ctx := context.Background()
	t, err := teobot.New(ctx, chatGpt, queries, pool)
	if err != nil {
		return err
	}

	m := mastodon.NewClient(env.MastodonBaseURL, env.MastodonClientKey, env.MastodonClientSecret, env.MastodonAccessToken)
	tb, err := mastodon.NewTeobotBinding(t, m, env.TeokureStoragePath, queries, pool)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			slog.Info("Processing new replies...")
			if err := tb.Run(ctx); err != nil {
				slog.Error("Failed to process new replies", "error", err)
			}

			slog.Info("Done. Waiting for 30 seconds before next check...")

			// Sleep for 30 seconds
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(30 * time.Second):
				// Continue
			}
		}
	}
}

func main() {
	slog.SetLogLoggerLevel(slog.LevelDebug)
	if err := run(); err != nil {
		panic(err)
	}
}
