package mastodon

import (
	"fmt"
	"log/slog"
)

type MockMastodonClient struct {
	AccountID string
}

func (m *MockMastodonClient) VerifyCredentials() (*Account, error) {
	return &Account{
		ID:          m.AccountID,
		Username:    "teobot",
		Acct:        "teobot@social.mikutter.hachune.net",
		DisplayName: "teokure robot",
	}, nil
}

func (m *MockMastodonClient) GetStatus(id string) (*Status, error) {
	return &Status{
		ID:      id,
		URL:     fmt.Sprintf("https://example.com/%s", id),
		Content: fmt.Sprintf("This is status %s", id),
		Account: Account{
			ID:          "dummy",
			Username:    "dummy-uname",
			Acct:        "dummy-acct",
			DisplayName: "Dummy account",
		},
	}, nil
}

func (m *MockMastodonClient) GetReplyTree(id string) (*Context, error) {
	return nil, nil
}

func (m *MockMastodonClient) PostStatus(content string, opt *PostStatusOpt) (*Status, error) {
	slog.Info("Mock: PostStatus", slog.String("content", content))
	return nil, nil
}

func (m *MockMastodonClient) GetAllNotifications(opt *GetAllNotificationsOpt) ([]*Notification, error) {
	slog.Info("Mock: GetAllNotifications")
	return nil, nil
}

func (m *MockMastodonClient) UploadImage(imageData []byte) (*MediaAttachment, error) {
	slog.Info("Mock: UploadImage")
	return nil, nil
}

func (m *MockMastodonClient) GetImage(id string) (*MediaAttachment, error) {
	slog.Info("Mock: GetImage", slog.String("id", id))
	return nil, nil
}
