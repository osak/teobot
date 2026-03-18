package main

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/osak/teobot/internal/config"
	"github.com/osak/teobot/internal/mastodon"
)

type app struct {
	m mastodon.Client
}

func (a *app) doGetStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	id := fs.Arg(0)
	status, err := a.m.GetStatus(id)
	if err != nil {
		return err
	}
	json.MarshalWrite(os.Stdout, status, jsontext.WithIndent("  "))
	return nil
}

func main() {
	// Configure global logger
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))
	env := config.LoadEnvFromOS()

	m := mastodon.NewClient(env.MastodonBaseURL, env.MastodonClientKey, env.MastodonClientSecret, env.MastodonAccessToken)
	app := app{m}

	cmd := os.Args[1]
	subArgs := os.Args[2:]
	var err error
	switch cmd {
	case "status":
		err = app.doGetStatus(subArgs)
	default:
		slog.Error(fmt.Sprintf("Unknown command %q", cmd))
	}
	if err != nil {
		slog.Error("Error", "error", err)
	}
}
