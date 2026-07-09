package app

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

type NotificationClient struct {
	webhookURL string
	secret     string
	portalURL  string
	launchURL  string
	httpClient *http.Client
}

type NotificationEvent struct {
	EventType  string                  `json:"event_type"`
	OccurredAt time.Time               `json:"occurred_at"`
	Submission NotificationSubmission  `json:"submission"`
	Recipients []NotificationRecipient `json:"recipients"`
	DocuSeal   map[string]any          `json:"docuseal,omitempty"`
	Metadata   map[string]string       `json:"metadata,omitempty"`
}

func NewNotificationClient(cfg Config) *NotificationClient {
	return &NotificationClient{
		webhookURL: cfg.NotificationWebhookURL,
		secret:     cfg.NotificationWebhookSecret,
		portalURL:  cfg.FrontendURL,
		launchURL:  cfg.MoodleOTASignLaunchURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *NotificationClient) Send(ctx context.Context, event NotificationEvent) error {
	if c.webhookURL == "" {
		return nil
	}
	if event.Metadata == nil {
		event.Metadata = map[string]string{}
	}
	event.Metadata["portal_url"] = c.portalURL
	if c.launchURL != "" {
		event.Metadata["launch_url"] = c.launchURL
	}

	body, err := json.Marshal(event)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.secret != "" {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		req.Header.Set("X-OTA-Signature", timestamp+"."+signNotificationBody(c.secret, timestamp, body))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("notification webhook failed: status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func signNotificationBody(secret string, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
