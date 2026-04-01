package notify

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ecleangg/booky/internal/config"
	"github.com/google/uuid"
)

func TestResendNotifierSendUsesNotificationOverrides(t *testing.T) {
	var received struct {
		From        string `json:"from"`
		To          []string
		Subject     string `json:"subject"`
		Text        string `json:"text"`
		Attachments []struct {
			Filename    string `json:"filename"`
			Content     string `json:"content"`
			ContentType string `json:"content_type"`
		} `json:"attachments"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/emails" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer resend-key" {
			t.Fatalf("unexpected auth header %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	notifier := NewResendNotifier(config.ResendConfig{
		APIKey:        "resend-key",
		From:          "bookkeeping@example.com",
		To:            []string{"finance@example.com"},
		BaseURL:       server.URL,
		SubjectPrefix: "[booky]",
	})

	err := notifier.Send(context.Background(), Notification{
		Severity:    SeverityWarning,
		Category:    CategoryDailyCloseWarning,
		CompanyID:   uuid.MustParse("11111111-1111-1111-1111-111111111111").String(),
		To:          []string{"override@example.com"},
		Subject:     "Daily close warning",
		SummaryLines: []string{"One warning"},
		Attachments: []Attachment{{
			Filename:    "draft.txt",
			ContentType: "text/plain",
			Content:     []byte("hello"),
		}},
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	if received.From != "bookkeeping@example.com" {
		t.Fatalf("unexpected from %q", received.From)
	}
	if len(received.To) != 1 || received.To[0] != "override@example.com" {
		t.Fatalf("unexpected recipients %#v", received.To)
	}
	if received.Subject != "[booky] Daily close warning" {
		t.Fatalf("unexpected subject %q", received.Subject)
	}
	if len(received.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(received.Attachments))
	}
	if decoded, _ := base64.StdEncoding.DecodeString(received.Attachments[0].Content); string(decoded) != "hello" {
		t.Fatalf("unexpected attachment content %q", string(decoded))
	}
}

func TestBuildBodyIncludesStructuredSections(t *testing.T) {
	postingDate := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	body := buildBody(Notification{
		Severity:     SeverityError,
		Category:     CategoryDailyCloseFailure,
		PostingDate:  &postingDate,
		CompanyID:    "company-123",
		Subject:      "Failure",
		SummaryLines: []string{"Summary line"},
		DetailLines:  []string{"Detail line"},
		ActionLines:  []string{"Action line"},
	})

	for _, wanted := range []string{
		"Failure",
		"Posting date: 2026-04-02",
		"Company ID: company-123",
		"Severity: error",
		"Category: daily_close_failure",
		"Summary",
		"- Summary line",
		"Details",
		"- Detail line",
		"How To Handle",
		"- Action line",
	} {
		if !strings.Contains(body, wanted) {
			t.Fatalf("body missing %q:\n%s", wanted, body)
		}
	}
}
