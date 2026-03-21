package main

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osak/teobot/internal/config"
	"github.com/osak/teobot/internal/db"
	"github.com/osak/teobot/internal/mastodon"
	"github.com/osak/teobot/internal/service/chatgpt"
	"github.com/osak/teobot/internal/service/teobot"
	pgxUUID "github.com/vgarvardt/pgx-google-uuid/v5"
)

type app struct {
	teobot        *teobot.Teobot
	mastodon      mastodon.Client
	teobotBinding *mastodon.TeobotBinding
}

func NewApp() (*app, error) {
	env := config.LoadEnvFromOS()
	chatGpt := chatgpt.New(env.ChatGPTAPIKey)

	pgxConfig, err := pgxpool.ParseConfig(env.DBConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database connection string: %w", err)
	}
	pgxConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		pgxUUID.Register(conn.TypeMap())
		return nil
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), pgxConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}
	queries := db.New(pool)

	ctx := context.Background()
	t, err := teobot.New(ctx, chatGpt, queries, pool)
	if err != nil {
		return nil, err
	}

	m := mastodon.NewClient(env.MastodonBaseURL, env.MastodonClientKey, env.MastodonClientSecret, env.MastodonAccessToken)
	tb, err := mastodon.NewTeobotBinding(t, m, env.TeokureStoragePath, queries, pool)
	if err != nil {
		return nil, err
	}

	return &app{
		teobot:        t,
		mastodon:      m,
		teobotBinding: tb,
	}, nil
}

func (app *app) runServer() error {
	ctx := context.Background()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			slog.Info("Processing new replies...")
			if err := app.teobotBinding.Run(ctx); err != nil {
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

func (app *app) doTalk(args []string) error {
	fs := flag.NewFlagSet("talk", flag.ContinueOnError)
	replyToStr := fs.String("reply-to", "", "Reply to message ID")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var message teobot.Message
	if err := json.Unmarshal([]byte(fs.Arg(0)), &message); err != nil {
		return err
	}

	slog.Info("Reply to", "replyToStr", *replyToStr)
	var replyTo uuid.UUID
	if *replyToStr != "" {
		replyTo = uuid.MustParse(*replyToStr)
	}

	resp, err := app.teobot.Talk(context.Background(), replyTo, &message)
	if err != nil {
		return err
	}

	return json.MarshalWrite(os.Stdout, resp, jsontext.WithIndent("  "))
}

func main() {
	// Configure global logger
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	app, err := NewApp()
	if err != nil {
		panic(err)
	}

	cmd := os.Args[1]
	subArgs := os.Args[2:]

	switch cmd {
	case "talk":
		err = app.doTalk(subArgs)
	case "server":
		err = app.runServer()
	default:
		slog.Error(fmt.Sprintf("Unknown command: %q", cmd))
	}
	if err != nil {
		panic(err)
	}
}
