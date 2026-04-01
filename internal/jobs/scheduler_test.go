package jobs

import (
	"context"
	"log/slog"
	"testing"

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
