package testutil

import (
	"context"
	"sync"

	"github.com/ecleangg/booky/internal/notify"
)

type RecordingNotifier struct {
	mu            sync.Mutex
	Notifications []notify.Notification
}

func (n *RecordingNotifier) Send(_ context.Context, notification notify.Notification) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Notifications = append(n.Notifications, notification)
	return nil
}

func (n *RecordingNotifier) Snapshot() []notify.Notification {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := make([]notify.Notification, len(n.Notifications))
	copy(out, n.Notifications)
	return out
}
