package jobs

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/ecleangg/booky/internal/accounting"
	"github.com/ecleangg/booky/internal/config"
	"github.com/ecleangg/booky/internal/notify"
	"github.com/google/uuid"
)

type recordingNotifier struct {
	notifications []notify.Notification
}

func (n *recordingNotifier) Send(_ context.Context, notification notify.Notification) error {
	n.notifications = append(n.notifications, notification)
	return nil
}

func TestSchedulerNotifiesOnTimezoneConfigurationError(t *testing.T) {
	notifier := &recordingNotifier{}
	service := &accounting.Service{Notify: notifier}
	scheduler := &Scheduler{
		Config: config.Config{
			App:   config.AppConfig{Timezone: "Bad/Timezone"},
			Bokio: config.BokioConfig{CompanyID: uuid.New()},
		},
		Service: service,
		Logger:  slog.Default(),
	}

	scheduler.tryRun(context.Background())

	if len(notifier.notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifier.notifications))
	}
	got := notifier.notifications[0]
	if got.Category != notify.CategorySchedulerConfig {
		t.Fatalf("expected scheduler config category, got %q", got.Category)
	}
	if got.Severity != notify.SeverityCritical {
		t.Fatalf("expected critical severity, got %q", got.Severity)
	}
}

func TestPostingDatesToRunBeforeCutoffOnlyIncludesYesterday(t *testing.T) {
	cfg := config.Config{
		Posting: config.PostingConfig{
			SchedulerEnabled:      true,
			CutoffTime:            "23:59",
			SchedulerPollInterval: "1m",
		},
	}
	now := time.Date(2026, time.April, 2, 10, 0, 0, 0, time.UTC)

	dates := postingDatesToRun(now, cfg)

	if len(dates) != 1 {
		t.Fatalf("expected 1 posting date, got %d", len(dates))
	}
	if got := dates[0].Format("2006-01-02"); got != "2026-04-01" {
		t.Fatalf("expected yesterday to be scheduled, got %s", got)
	}
}

func TestPostingDatesToRunAfterCutoffIncludesToday(t *testing.T) {
	cfg := config.Config{
		Posting: config.PostingConfig{
			SchedulerEnabled:      true,
			CutoffTime:            "09:00",
			SchedulerPollInterval: "1m",
		},
	}
	now := time.Date(2026, time.April, 2, 10, 0, 0, 0, time.UTC)

	dates := postingDatesToRun(now, cfg)

	if len(dates) != 2 {
		t.Fatalf("expected 2 posting dates, got %d", len(dates))
	}
	if got := dates[0].Format("2006-01-02"); got != "2026-04-01" {
		t.Fatalf("expected yesterday first, got %s", got)
	}
	if got := dates[1].Format("2006-01-02"); got != "2026-04-02" {
		t.Fatalf("expected today second, got %s", got)
	}
}

func TestPostingDatesToRunSkipsDisabledScheduler(t *testing.T) {
	cfg := config.Config{
		Posting: config.PostingConfig{
			SchedulerEnabled:      false,
			CutoffTime:            "09:00",
			SchedulerPollInterval: "1m",
		},
	}

	dates := postingDatesToRun(time.Date(2026, time.April, 2, 10, 0, 0, 0, time.UTC), cfg)
	if len(dates) != 0 {
		t.Fatalf("expected no posting dates, got %d", len(dates))
	}
}
