package history

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"github.com/osak/teobot/internal/mastodon"
)

// ThreadHistory represents a collection of conversation threads
type ThreadHistory struct {
	Threads [][]*mastodon.PartialStatus `json:"threads"`
}

type envelope struct {
	Messages []*mastodon.Status `json:"messages"`
}

// HistoryService manages conversation history
type HistoryService struct {
	storagePath string
}

// NewHistoryService creates a new history service
func NewHistoryService(storagePath string) *HistoryService {
	return &HistoryService{
		storagePath: storagePath,
	}
}

// GetHistory retrieves conversation history for a given account
func (h *HistoryService) GetHistory(acct string, limit int) ([][]*mastodon.Status, error) {
	// Get all history files for this account
	pattern := filepath.Join(h.storagePath, "threads", acct, "*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list history files: %w", err)
	}

	// Sort files by name
	sort.Strings(files)
	slices.Reverse(files) // Reverse to get the most recent first

	// Limit the number of files to process
	if len(files) > limit {
		files = files[:limit]
	}

	// Read and parse each file
	var threads [][]*mastodon.Status
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue // Skip files that can't be read
		}

		var e envelope
		if err := json.Unmarshal(data, &e); err != nil {
			slog.Error("Failed to unmarshal thread", "file", file, "error", err)
			continue // Skip files that can't be parsed
		}

		threads = append(threads, e.Messages)
	}

	return threads, nil
}

// SaveHistory stores a conversation thread
func (h *HistoryService) SaveHistory(thread []*mastodon.Status) error {
	if len(thread) == 0 {
		return nil // Nothing to save
	}

	// Use the first status ID as the filename
	firstStatus := thread[0]
	filename := fmt.Sprintf("%s.json", firstStatus.ID)

	// Ensure the directory exists
	threadsDir := filepath.Join(h.storagePath, "threads")
	if err := os.MkdirAll(threadsDir, 0755); err != nil {
		return fmt.Errorf("failed to create threads directory: %w", err)
	}

	// Serialize the thread
	data, err := json.Marshal(thread)
	if err != nil {
		return fmt.Errorf("failed to marshal thread: %w", err)
	}

	// Write to file
	path := filepath.Join(threadsDir, filename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write thread file: %w", err)
	}

	return nil
}
