package alarm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client posts alarm events to Home Assistant (or compatible) endpoints.
type Client struct {
	baseURL   string
	token     string
	eventName string
	http      *http.Client
}

// Payload describes a slash-command request that should trigger an alarm.
type Payload struct {
	Message       string    `json:"message"`
	GuildID       string    `json:"guild_id,omitempty"`
	ChannelID     string    `json:"channel_id,omitempty"`
	ChannelName   string    `json:"channel_name,omitempty"`
	TriggeredBy   string    `json:"triggered_by,omitempty"`
	TriggeredByID string    `json:"triggered_by_id,omitempty"`
	TriggeredAt   time.Time `json:"triggered_at"`
}

const defaultTimeout = 5 * time.Second

// NewClient returns a ready-to-use alarm client.
func NewClient(baseURL, token, eventName string) (*Client, error) {
	baseURL = strings.TrimSpace(baseURL)
	token = strings.TrimSpace(token)
	eventName = strings.TrimSpace(eventName)
	if baseURL == "" {
		return nil, fmt.Errorf("baseURL is empty")
	}
	if token == "" {
		return nil, fmt.Errorf("token is empty")
	}
	if eventName == "" {
		return nil, fmt.Errorf("eventName is empty")
	}
	baseURL = strings.TrimRight(baseURL, "/")

	return &Client{
		baseURL:   baseURL,
		token:     token,
		eventName: eventName,
		http: &http.Client{
			Timeout: defaultTimeout,
		},
	}, nil
}

// Trigger posts the payload to /api/events/<event>.
func (c *Client) Trigger(ctx context.Context, payload Payload) error {
	if c == nil {
		return fmt.Errorf("alarm client is nil")
	}
	if payload.Message == "" {
		return fmt.Errorf("message is empty")
	}
	eventURL := fmt.Sprintf("%s/api/events/%s", c.baseURL, c.eventName)
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, eventURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("send alarm request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("alarm request failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return nil
}
