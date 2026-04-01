package notify

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ecleangg/booky/internal/config"
)

type ResendNotifier struct {
	baseURL       string
	apiKey        string
	from          string
	to            []string
	subjectPrefix string
	httpClient    *http.Client
}

func NewResendNotifier(cfg config.ResendConfig) *ResendNotifier {
	return &ResendNotifier{
		baseURL:       strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:        cfg.APIKey,
		from:          cfg.From,
		to:            cfg.To,
		subjectPrefix: cfg.SubjectPrefix,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (n *ResendNotifier) Send(ctx context.Context, notification Notification) error {
	subject := strings.TrimSpace(notification.Subject)
	if n.subjectPrefix != "" {
		subject = strings.TrimSpace(n.subjectPrefix + " " + subject)
	}
	body := buildBody(notification)
	recipients := n.to
	if len(notification.To) > 0 {
		recipients = notification.To
	}
	payload := map[string]any{
		"from":    n.from,
		"to":      recipients,
		"subject": subject,
		"text":    body,
	}
	if len(notification.Attachments) > 0 {
		attachments := make([]map[string]any, 0, len(notification.Attachments))
		for _, attachment := range notification.Attachments {
			if attachment.Filename == "" || len(attachment.Content) == 0 {
				continue
			}
			part := map[string]any{
				"filename": attachment.Filename,
				"content":  base64.StdEncoding.EncodeToString(attachment.Content),
			}
			if attachment.ContentType != "" {
				part["content_type"] = attachment.ContentType
			}
			attachments = append(attachments, part)
		}
		if len(attachments) > 0 {
			payload["attachments"] = attachments
		}
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal resend payload: %w", err)
	}
	endpoint, err := n.resolveURL("/emails")
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("create resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+n.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call resend api: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read resend response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("resend api returned %d: %s", resp.StatusCode, bytes.TrimSpace(bodyBytes))
	}
	return nil
}

func (n *ResendNotifier) resolveURL(endpoint string) (string, error) {
	base, err := url.Parse(n.baseURL)
	if err != nil {
		return "", fmt.Errorf("parse resend base url: %w", err)
	}
	ref, err := url.Parse(strings.TrimLeft(endpoint, "/"))
	if err != nil {
		return "", fmt.Errorf("parse resend endpoint: %w", err)
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/" + ref.Path
	return base.String(), nil
}
