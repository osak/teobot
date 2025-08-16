// Package api provides implementations for various external API integrations
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// JsonApiCustom defines customization options for the JSON API client
type JsonApiCustom struct {
	Headers     func() map[string]string
	CheckStatus func(code int) bool
	HandleError func(response *http.Response) error
}

// JsonApi is a client for making HTTP requests to JSON APIs
type JsonApi struct {
	baseURL string
	custom  JsonApiCustom
	client  *http.Client
}

// NewJsonApi creates a new JSON API client
func NewJsonApi(baseURL string, custom JsonApiCustom) *JsonApi {
	return &JsonApi{
		baseURL: baseURL,
		custom:  custom,
		client:  &http.Client{},
	}
}

// Get performs a GET request to the specified path
func (j *JsonApi) Get(path string, v interface{}) error {
	return j.doCall(http.MethodGet, path, nil, v)
}

// Post performs a POST request to the specified path with the given body
func (j *JsonApi) Post(path string, body interface{}, v interface{}) error {
	return j.doCall(http.MethodPost, path, body, v)
}

// doCall performs the HTTP request
func (j *JsonApi) doCall(method, path string, body interface{}, v interface{}) error {
	url := j.baseURL + path

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	j.setHeaders(req)

	resp, err := j.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if !j.checkStatus(resp.StatusCode) {
		return j.handleError(resp)
	}

	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// setHeaders sets the headers for the request
func (j *JsonApi) setHeaders(req *http.Request) {
	// Set default Content-Type header
	req.Header.Set("Content-Type", "application/json")

	// Add custom headers if provided
	if j.custom.Headers != nil {
		for key, value := range j.custom.Headers() {
			req.Header.Set(key, value)
		}
	}
}

// checkStatus checks if the HTTP status code indicates success
func (j *JsonApi) checkStatus(code int) bool {
	if j.custom.CheckStatus != nil {
		return j.custom.CheckStatus(code)
	}
	return code >= 200 && code < 300
}

// handleError handles an error response
func (j *JsonApi) handleError(resp *http.Response) error {
	if j.custom.HandleError != nil {
		return j.custom.HandleError(resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("API error (status: %d), failed to read response body: %w", resp.StatusCode, err)
	}

	return fmt.Errorf("API error (status: %d): %s", resp.StatusCode, body)
}
