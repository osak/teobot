package config

import (
	"os"
)

// Env holds all environment configuration
type Env struct {
	MastodonBaseURL      string
	MastodonClientKey    string
	MastodonClientSecret string
	MastodonAccessToken  string
	ChatGPTAPIKey        string
	TeokureStoragePath   string
	HistoryStoragePath   string
	DBConnectionString   string
	NewRelicLicenseKey   string
	NewRelicAppName      string
}

// LoadEnvFromOS loads environment variables from the OS
func LoadEnvFromOS() *Env {
	return &Env{
		MastodonBaseURL:      getEnv("MASTODON_BASE_URL", ""),
		MastodonClientKey:    getEnv("MASTODON_CLIENT_KEY", ""),
		MastodonClientSecret: getEnv("MASTODON_CLIENT_SECRET", ""),
		MastodonAccessToken:  getEnv("MASTODON_ACCESS_TOKEN", ""),
		ChatGPTAPIKey:        getEnv("CHAT_GPT_API_KEY", ""),
		TeokureStoragePath:   getEnv("TEOKURE_STORAGE_PATH", "data"),
		HistoryStoragePath:   getEnv("HISTORY_STORAGE_PATH", "tmp"),
		DBConnectionString:   getEnv("DB_CONNECTION_STRING", ""),
		NewRelicLicenseKey:   getEnv("NEW_RELIC_LICENSE_KEY", ""),
		NewRelicAppName:      getEnv("NEW_RELIC_APP_NAME", "teobot"),
	}
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
