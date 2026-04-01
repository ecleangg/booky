package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (q *Queries) InsertWebhookEvent(ctx context.Context, evt domain.StripeWebhookEvent) (bool, error) {
	cmd, err := q.db.Exec(ctx, `
		INSERT INTO stripe_webhook_events (
			stripe_event_id, event_type, livemode, api_version, stripe_created_at, payload
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (stripe_event_id) DO NOTHING
	`, evt.ID, evt.EventType, evt.Livemode, nullIfEmpty(evt.APIVersion), evt.StripeCreatedAt, []byte(evt.Payload))
	if err != nil {
		return false, fmt.Errorf("insert webhook event: %w", err)
	}
	return cmd.RowsAffected() == 1, nil
}

func (q *Queries) GetWebhookEvent(ctx context.Context, eventID string) (domain.StripeWebhookEvent, error) {
	row := q.db.QueryRow(ctx, `
		SELECT stripe_event_id, event_type, livemode, api_version, stripe_created_at, received_at, payload, processed_at, processing_error
		FROM stripe_webhook_events
		WHERE stripe_event_id = $1
	`, eventID)

	var evt domain.StripeWebhookEvent
	var payload []byte
	var processedAt sql.NullTime
	var processingError sql.NullString
	if err := row.Scan(&evt.ID, &evt.EventType, &evt.Livemode, &evt.APIVersion, &evt.StripeCreatedAt, &evt.ReceivedAt, &payload, &processedAt, &processingError); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.StripeWebhookEvent{}, ErrNotFound
		}
		return domain.StripeWebhookEvent{}, fmt.Errorf("get webhook event: %w", err)
	}
	evt.Payload = payload
	if processedAt.Valid {
		evt.ProcessedAt = &processedAt.Time
	}
	if processingError.Valid {
		evt.ProcessingError = &processingError.String
	}
	return evt, nil
}

func (q *Queries) ResetWebhookForRetry(ctx context.Context, evt domain.StripeWebhookEvent) error {
	_, err := q.db.Exec(ctx, `
		UPDATE stripe_webhook_events
		SET event_type = $2,
			livemode = $3,
			api_version = $4,
			stripe_created_at = $5,
			payload = $6,
			processed_at = NULL,
			processing_error = NULL
		WHERE stripe_event_id = $1
	`, evt.ID, evt.EventType, evt.Livemode, nullIfEmpty(evt.APIVersion), evt.StripeCreatedAt, []byte(evt.Payload))
	if err != nil {
		return fmt.Errorf("reset webhook for retry: %w", err)
	}
	return nil
}

func (q *Queries) MarkWebhookProcessed(ctx context.Context, eventID string) error {
	_, err := q.db.Exec(ctx, `UPDATE stripe_webhook_events SET processed_at = now(), processing_error = NULL WHERE stripe_event_id = $1`, eventID)
	if err != nil {
		return fmt.Errorf("mark webhook processed: %w", err)
	}
	return nil
}

func (q *Queries) MarkWebhookFailed(ctx context.Context, eventID, reason string) error {
	_, err := q.db.Exec(ctx, `UPDATE stripe_webhook_events SET processing_error = $2 WHERE stripe_event_id = $1`, eventID, reason)
	if err != nil {
		return fmt.Errorf("mark webhook failed: %w", err)
	}
	return nil
}

func (q *Queries) UpsertObjectSnapshot(ctx context.Context, snapshot domain.ObjectSnapshot) error {
	_, err := q.db.Exec(ctx, `
		INSERT INTO stripe_object_snapshots (object_type, stripe_object_id, livemode, payload)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (object_type, stripe_object_id)
		DO UPDATE SET payload = EXCLUDED.payload, livemode = EXCLUDED.livemode, last_synced_at = now()
	`, snapshot.ObjectType, snapshot.ObjectID, snapshot.Livemode, []byte(snapshot.Payload))
	if err != nil {
		return fmt.Errorf("upsert object snapshot: %w", err)
	}
	return nil
}
