package filings

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/store"
)

type entrySource interface {
	Snapshot(ctx context.Context, objectType, objectID string) (json.RawMessage, error)
	ParentChargeForRefund(ctx context.Context, refundID string, stripeEventID *string) (json.RawMessage, error)
}

type memorySource struct {
	snapshots map[string]json.RawMessage
	charges   []json.RawMessage
}

func newMemorySource(snapshots []domain.ObjectSnapshot, _ map[string]json.RawMessage) entrySource {
	source := &memorySource{
		snapshots: make(map[string]json.RawMessage, len(snapshots)),
		charges:   make([]json.RawMessage, 0),
	}
	for _, snapshot := range snapshots {
		source.snapshots[snapshotKey(snapshot.ObjectType, snapshot.ObjectID)] = snapshot.Payload
		if snapshot.ObjectType == "charge" {
			source.charges = append(source.charges, snapshot.Payload)
		}
	}
	return source
}

func (s *memorySource) Snapshot(_ context.Context, objectType, objectID string) (json.RawMessage, error) {
	raw, ok := s.snapshots[snapshotKey(objectType, objectID)]
	if !ok {
		return nil, store.ErrNotFound
	}
	return raw, nil
}

func (s *memorySource) ParentChargeForRefund(_ context.Context, refundID string, _ *string) (json.RawMessage, error) {
	for _, raw := range s.charges {
		charge, err := decodeCharge(raw)
		if err != nil {
			continue
		}
		for _, refund := range charge.Refunds.Data {
			if refund.ID == refundID {
				return raw, nil
			}
		}
	}
	return nil, store.ErrNotFound
}

type repositorySource struct {
	repo *store.Repository
}

func newRepositorySource(repo *store.Repository) entrySource {
	return &repositorySource{repo: repo}
}

func (s *repositorySource) Snapshot(ctx context.Context, objectType, objectID string) (json.RawMessage, error) {
	snapshot, err := s.repo.Queries().GetObjectSnapshot(ctx, objectType, objectID)
	if err != nil {
		return nil, err
	}
	return snapshot.Payload, nil
}

func (s *repositorySource) ParentChargeForRefund(ctx context.Context, refundID string, stripeEventID *string) (json.RawMessage, error) {
	if stripeEventID == nil || strings.TrimSpace(*stripeEventID) == "" {
		return nil, store.ErrNotFound
	}
	evt, err := s.repo.Queries().GetWebhookEvent(ctx, strings.TrimSpace(*stripeEventID))
	if err != nil {
		return nil, err
	}
	var envelope stripeEventEnvelope
	if err := json.Unmarshal(evt.Payload, &envelope); err != nil {
		return nil, fmt.Errorf("decode webhook event %s: %w", evt.ID, err)
	}
	charge, err := decodeCharge(envelope.Data.Object)
	if err != nil {
		return nil, err
	}
	for _, refund := range charge.Refunds.Data {
		if refund.ID == refundID {
			return envelope.Data.Object, nil
		}
	}
	return nil, store.ErrNotFound
}
