package mastodon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Client handles communication with the Mastodon API
type Client struct {
	baseURL      string
	clientKey    string
	clientSecret string
	accessToken  string
	httpClient   *http.Client
	logger       *slog.Logger
}

// NewClient creates a new Mastodon API client
func NewClient(baseURL, clientKey, clientSecret, accessToken string) *Client {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil)).With("component", "mastodon")

	return &Client{
		baseURL:      baseURL,
		clientKey:    clientKey,
		clientSecret: clientSecret,
		accessToken:  accessToken,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		logger:       logger,
	}
}

// Account represents a Mastodon user account
type Account struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	Acct        string `json:"acct"`
	DisplayName string `json:"display_name"`
}

// Status represents a Mastodon post
type Status struct {
	ID                 string  `json:"id"`
	URL                string  `json:"url"`
	InReplyToID        string  `json:"in_reply_to_id"`
	InReplyToAccountID string  `json:"in_reply_to_account_id"`
	Content            string  `json:"content"`
	Account            Account `json:"account"`
	Visibility         string  `json:"visibility"`
	CreatedAt          string  `json:"created_at"`
}

// PartialStatus contains a subset of Status fields for history
type PartialStatus struct {
	Account    Account `json:"account,omitempty"`
	Content    string  `json:"content,omitempty"`
	CreatedAt  string  `json:"created_at,omitempty"`
	Visibility string  `json:"visibility,omitempty"`
}

// NotificationType represents the type of a notification
type NotificationType string

// Context represents a thread of statuses
type Context struct {
	Ancestors   []*Status `json:"ancestors"`
	Descendants []*Status `json:"descendants"`
}

// Notification represents a Mastodon notification
type Notification struct {
	ID      string  `json:"id"`
	Type    string  `json:"type"`
	Account Account `json:"account"`
	Status  *Status `json:"status,omitempty"`
}

// MediaAttachment represents uploaded media
type MediaAttachment struct {
	ID     string `json:"id"`
	URL    string `json:"url,omitempty"`
	Status string `json:"status,omitempty"`
}

// PostStatusOpt contains options for posting a status
type PostStatusOpt struct {
	ReplyToID  string
	MediaIDs   []string
	Visibility string
	Sensitive  bool
}

// GetAllNotificationsOpt contains options for fetching notifications
type GetAllNotificationsOpt struct {
	MaxID   string
	SinceID string
	Types   []string
}

// VerifyCredentials fetches the current user's account information
func (c *Client) VerifyCredentials() (*Account, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/accounts/verify_credentials", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var account Account
	if err := json.NewDecoder(resp.Body).Decode(&account); err != nil {
		return nil, err
	}

	return &account, nil
}

// GetStatus fetches a status by ID
func (c *Client) GetStatus(id string) (*Status, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/statuses/"+id, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var status Status
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}

	return &status, nil
}

// GetReplyTree fetches the conversation thread for a status
func (c *Client) GetReplyTree(id string) (*Context, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/statuses/"+id+"/context", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var context Context
	if err := json.NewDecoder(resp.Body).Decode(&context); err != nil {
		return nil, err
	}

	return &context, nil
}

// PostStatus creates a new status
func (c *Client) PostStatus(content string, opt *PostStatusOpt) (*Status, error) {
	data := map[string]interface{}{
		"status": content,
	}

	if opt != nil {
		if opt.ReplyToID != "" {
			data["in_reply_to_id"] = opt.ReplyToID
		}

		if len(opt.MediaIDs) > 0 {
			data["media_ids"] = opt.MediaIDs
		}

		if opt.Visibility != "" {
			data["visibility"] = opt.Visibility
		}

		if opt.Sensitive {
			data["sensitive"] = true
		}
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.baseURL+"/api/v1/statuses", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+c.accessToken)

	if c.logger != nil {
		c.logger.Debug("Posting status", "content_length", len(content), "has_media", len(opt.MediaIDs) > 0)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var status Status
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}

	return &status, nil
}

// GetAllNotifications fetches notifications with optional filters
func (c *Client) GetAllNotifications(opt *GetAllNotificationsOpt) ([]*Notification, error) {
	// Build query parameters
	params := url.Values{}

	if opt != nil {
		if opt.MaxID != "" {
			params.Add("max_id", opt.MaxID)
		}

		if opt.SinceID != "" {
			params.Add("since_id", opt.SinceID)
		}

		if len(opt.Types) > 0 {
			for _, t := range opt.Types {
				params.Add("types[]", t)
			}
		}
	}

	// Construct URL with parameters
	apiURL := c.baseURL + "/api/v1/notifications"
	if len(params) > 0 {
		apiURL += "?" + params.Encode()
	}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var notifications []*Notification
	if err := json.NewDecoder(resp.Body).Decode(&notifications); err != nil {
		return nil, err
	}

	return notifications, nil
}

// UploadImage uploads an image to the Mastodon media API
func (c *Client) UploadImage(imageData []byte) (*MediaAttachment, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "image.png")
	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(part, bytes.NewReader(imageData)); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.baseURL+"/api/v2/media", body)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", writer.FormDataContentType())
	req.Header.Add("Authorization", "Bearer "+c.accessToken)

	if c.logger != nil {
		c.logger.Debug("Uploading image", "size", len(imageData))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read and parse response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var media MediaAttachment
	if err := json.Unmarshal(respBody, &media); err != nil {
		return nil, err
	}

	// Set status based on response code
	if resp.StatusCode == http.StatusOK {
		media.Status = "uploaded"
	} else if resp.StatusCode == http.StatusAccepted {
		media.Status = "uploading"
	} else {
		media.Status = "error"
		if c.logger != nil {
			c.logger.Error("Failed to upload image", "status_code", resp.StatusCode)
		}
		return &media, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return &media, nil
}

// GetImage gets information about an uploaded media attachment
func (c *Client) GetImage(id string) (*MediaAttachment, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/media/"+id, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read and parse response
	var media MediaAttachment
	if err := json.NewDecoder(resp.Body).Decode(&media); err != nil {
		return nil, err
	}

	// Set status based on response code
	if resp.StatusCode == http.StatusOK {
		media.Status = "uploaded"
	} else if resp.StatusCode == http.StatusPartialContent {
		media.Status = "uploading"
	} else {
		media.Status = "error"
		return &media, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return &media, nil
}

// NormalizeStatusContent strips HTML tags and normalizes content
func NormalizeStatusContent(status *Status) string {
	content := status.Content

	// Simple HTML tag removal - in a real implementation you'd want a proper HTML parser
	content = stripHTMLTags(content)

	// Decode HTML entities
	content = unescapeHTML(content)

	return content
}

// Helper function to strip HTML tags - basic implementation
func stripHTMLTags(s string) string {
	var result strings.Builder
	inTag := false

	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// Helper function to unescape HTML entities - basic implementation
func unescapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	return s
}
