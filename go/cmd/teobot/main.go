package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/osak/teobot/internal/api"
	"github.com/osak/teobot/internal/config"
	"github.com/osak/teobot/internal/history"
	"github.com/osak/teobot/internal/mastodon"
	"github.com/osak/teobot/internal/service"
	"github.com/osak/teobot/internal/service/teobot"
	"github.com/osak/teobot/internal/textsplit"
	pgxUUID "github.com/vgarvardt/pgx-google-uuid/v5"
)

const (
	historyCharsLimit = 1000
)

// State stores persistent program state
type State struct {
	LastNotificationID string `json:"lastNotificationId,omitempty"`
}

// TeokureCli implements the main CLI application
type TeokureCli struct {
	logger                 *slog.Logger
	mastodonTeobotFrontend *teobot.MastodonTeobotFrontend
}

// NewTeokureCli creates a new instance of the CLI application
func NewTeokureCli(env *config.Env) (*TeokureCli, error) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil)).With("component", "teokure-cli")
	chatGPT := service.NewChatGPT(env.ChatGPTAPIKey)
	jmaAPI := api.NewJmaApi()
	dalleAPI := service.NewDallE(env.ChatGPTAPIKey)

	teobotService := service.NewTeobotService(chatGPT, jmaAPI, dalleAPI)
	mastodonClient := mastodon.NewClient(env.MastodonBaseURL, env.MastodonClientKey, env.MastodonClientSecret, env.MastodonAccessToken)
	textSplitService := textsplit.NewTextSplitService(chatGPT)
	historyService := history.NewHistoryService(env.HistoryStoragePath)

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

	mastodonTeobotFrontend, err := teobot.NewMastodonTeobotFrontend(mastodonClient, teobotService, historyService, textSplitService, pool, env.TeokureStoragePath)
	if err != nil {
		return nil, err
	}

	return &TeokureCli{
		logger:                 logger,
		mastodonTeobotFrontend: mastodonTeobotFrontend,
	}, nil
}

// RunCommand runs a specific command
func (t *TeokureCli) RunCommand(commandStr string) error {
	parts := strings.SplitN(commandStr, " ", 2)
	command := parts[0]
	rest := ""
	if len(parts) > 1 {
		rest = parts[1]
	}

	switch command {
	case "chat":
		parts := strings.SplitN(rest, " ", 2)
		acct := parts[0]
		body := parts[1]
		status := mastodon.Status{
			ID:      "12345",
			Content: body,
			Account: mastodon.Account{
				ID:   "12345",
				Acct: acct,
			},
		}
		ctx := context.Background()
		res, err := t.mastodonTeobotFrontend.ReplyTo(ctx, &status)
		if err != nil {
			return err
		}
		t.logger.Info("Reply", "result", res)
	case "reply_to":
		ctx := context.Background()
		res, err := t.mastodonTeobotFrontend.ReplyToStatusId(ctx, rest)
		if err != nil {
			return err
		}
		t.logger.Info("Reply", "result", res)
	case "history":
		parts := strings.Split(rest, " ")
		acct := parts[0]
		// Ignoring limitStr for now

		ctx := context.Background()
		history, err := t.mastodonTeobotFrontend.BuildThreadHistory(ctx, acct)
		if err != nil {
			return fmt.Errorf("failed to build thread history: %w", err)
		}

		historyJSON, err := json.Marshal(history)
		if err != nil {
			return fmt.Errorf("failed to marshal history: %w", err)
		}

		t.logger.Info("Thread history", "account", acct, "history", string(historyJSON))

	case "thread":
		parts := strings.Split(rest, " ")
		statusID := parts[0]

		ctx := context.Background()
		if err := t.mastodonTeobotFrontend.ProcessReply(ctx, statusID); err != nil {
			return fmt.Errorf("failed to get thread: %w", err)
		}

	default:
		t.logger.Error("Unknown command", "command", command)
	}

	return nil
}

// RunREPL runs an interactive command loop
func (t *TeokureCli) RunREPL() error {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Print("> ")
	for scanner.Scan() {
		command := scanner.Text()
		if err := t.RunCommand(command); err != nil {
			t.logger.Error("Command failed", "error", err)
		}
		fmt.Print("> ")
	}

	return scanner.Err()
}

// RunServer runs the application in server mode
func (t *TeokureCli) RunServer(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			t.logger.Info("Processing new replies...")
			if err := t.mastodonTeobotFrontend.Run(ctx); err != nil {
				t.logger.Error("Failed to process new replies", "error", err)
			}

			t.logger.Info("Done. Waiting for 30 seconds before next check...")

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
	// Configure global logger
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// Load .env file if present
	if err := godotenv.Load(); err != nil {
		slog.Info("No .env file found")
	}

	// Load environment variables
	env := config.LoadEnvFromOS()

	cli, err := NewTeokureCli(env)
	if err != nil {
		slog.Error("Failed to create Teokure CLI", "error", err)
		os.Exit(1)
	}

	// Check if we should run in server mode
	if len(os.Args) >= 2 && os.Args[1] == "server" {
		slog.Info("Running in server mode")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Handle graceful shutdown
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-sigs
			slog.Info("Received signal, shutting down...")
			cancel()
		}()

		if err := cli.RunServer(ctx); err != nil && err != context.Canceled {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	} else {
		if err := cli.RunREPL(); err != nil {
			slog.Error("REPL error", "error", err)
			os.Exit(1)
		}
	}
}
