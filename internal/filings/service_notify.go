package filings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/notify"
	"github.com/ecleangg/booky/internal/store"
	"github.com/ecleangg/booky/internal/support"
	"github.com/google/uuid"
)

func (s *Service) failEvaluation(ctx context.Context, kind, period string, now time.Time, cause error) error {
	updateErr := s.Repo.InTx(ctx, func(q *store.Queries) error {
		return q.UpdateFilingPeriodEvaluation(ctx, s.Config.Bokio.CompanyID, kind, period, domain.FilingPeriodStatusEvaluationFailed, now.UTC(), nil)
	})
	notifyErr := s.sendFailureNotification(ctx, kind, period, cause)
	return errors.Join(cause, updateErr, notifyErr)
}

func (s *Service) sendZeroReminder(ctx context.Context, filingPeriod domain.FilingPeriod, reviewEntries int, hasLatest bool, latest domain.FilingExport, now time.Time) (*domain.FilingExport, error) {
	checksum := "zero-reminder"
	if hasLatest && latest.Checksum == checksum {
		if err := s.Repo.InTx(ctx, func(q *store.Queries) error {
			return q.UpdateFilingPeriodEvaluation(ctx, s.Config.Bokio.CompanyID, filingPeriod.Kind, filingPeriod.Period, domain.FilingPeriodStatusUnchanged, now.UTC(), nil)
		}); err != nil {
			return nil, err
		}
		return nil, nil
	}

	summary := map[string]any{
		"kind":           filingPeriod.Kind,
		"period":         filingPeriod.Period,
		"ready_entries":  0,
		"review_entries": reviewEntries,
		"zero_return":    true,
	}
	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		return nil, err
	}

	export := domain.FilingExport{
		ID:             uuid.New(),
		Kind:           filingPeriod.Kind,
		Period:         filingPeriod.Period,
		BokioCompanyID: s.Config.Bokio.CompanyID,
		Version:        nextExportVersion(hasLatest, latest),
		Checksum:       checksum,
		Summary:        summaryJSON,
		EmailedAt:      timePtr(now.UTC()),
	}
	if err := s.sendZeroReminderNotification(ctx, filingPeriod, reviewEntries); err != nil {
		return nil, s.failEvaluation(ctx, filingPeriod.Kind, filingPeriod.Period, now, err)
	}
	if err := s.Repo.InTx(ctx, func(q *store.Queries) error {
		if err := q.CreateFilingExport(ctx, export); err != nil {
			return err
		}
		if hasLatest {
			if err := q.MarkFilingExportSuperseded(ctx, latest.ID, export.ID); err != nil {
				return err
			}
		}
		zeroSent := now.UTC()
		return q.UpdateFilingPeriodEvaluation(ctx, s.Config.Bokio.CompanyID, filingPeriod.Kind, filingPeriod.Period, domain.FilingPeriodStatusNoDataReminder, now.UTC(), &zeroSent)
	}); err != nil {
		return nil, err
	}
	return &export, nil
}

func (s *Service) sendRenderedExport(ctx context.Context, filingPeriod domain.FilingPeriod, rendered RenderedFile, readyEntries, reviewEntries int, hasLatest bool, latest domain.FilingExport, now time.Time) (*domain.FilingExport, error) {
	checksum := checksumBytes(rendered.Content)
	if hasLatest && latest.Checksum == checksum {
		if err := s.Repo.InTx(ctx, func(q *store.Queries) error {
			return q.UpdateFilingPeriodEvaluation(ctx, s.Config.Bokio.CompanyID, filingPeriod.Kind, filingPeriod.Period, domain.FilingPeriodStatusUnchanged, now.UTC(), nil)
		}); err != nil {
			return nil, err
		}
		return nil, nil
	}

	version := nextExportVersion(hasLatest, latest)
	summary := map[string]any{
		"kind":           filingPeriod.Kind,
		"period":         filingPeriod.Period,
		"ready_entries":  readyEntries,
		"review_entries": reviewEntries,
		"filename":       rendered.Filename,
		"checksum":       checksum,
		"version":        version,
	}
	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		return nil, err
	}

	export := domain.FilingExport{
		ID:             uuid.New(),
		Kind:           filingPeriod.Kind,
		Period:         filingPeriod.Period,
		BokioCompanyID: s.Config.Bokio.CompanyID,
		Version:        version,
		Checksum:       checksum,
		Filename:       &rendered.Filename,
		Content:        rendered.Content,
		Summary:        summaryJSON,
		EmailedAt:      timePtr(now.UTC()),
	}
	if err := s.sendDraftNotification(ctx, filingPeriod, rendered, readyEntries, reviewEntries, version); err != nil {
		return nil, s.failEvaluation(ctx, filingPeriod.Kind, filingPeriod.Period, now, err)
	}
	if err := s.Repo.InTx(ctx, func(q *store.Queries) error {
		if err := q.CreateFilingExport(ctx, export); err != nil {
			return err
		}
		if hasLatest {
			if err := q.MarkFilingExportSuperseded(ctx, latest.ID, export.ID); err != nil {
				return err
			}
		}
		return q.UpdateFilingPeriodEvaluation(ctx, s.Config.Bokio.CompanyID, filingPeriod.Kind, filingPeriod.Period, domain.FilingPeriodStatusExported, now.UTC(), nil)
	}); err != nil {
		return nil, err
	}
	return &export, nil
}

func (s *Service) sendDraftNotification(ctx context.Context, filingPeriod domain.FilingPeriod, rendered RenderedFile, readyEntries, reviewEntries, version int) error {
	if s.Notify == nil {
		return nil
	}
	subject := fmt.Sprintf("%s draft ready for %s", filingLabel(filingPeriod.Kind), filingPeriod.Period)
	if version > 1 {
		subject = fmt.Sprintf("%s revision %d ready for %s", filingLabel(filingPeriod.Kind), version, filingPeriod.Period)
	}
	return s.Notify.Send(ctx, notify.Notification{
		Severity:  notify.SeverityInfo,
		Category:  notify.CategoryFilingDraft,
		CompanyID: s.Config.Bokio.CompanyID.String(),
		To:        append([]string(nil), s.Config.Filings.EmailTo...),
		Subject:   subject,
		SummaryLines: []string{
			fmt.Sprintf("Period: %s", filingPeriod.Period),
			fmt.Sprintf("Deadline: %s", filingPeriod.DeadlineDate.In(support.LocationOrUTC(s.Config)).Format("2006-01-02")),
			fmt.Sprintf("Ready entries: %d", readyEntries),
			fmt.Sprintf("Review entries excluded from the file: %d", reviewEntries),
		},
		DetailLines: []string{
			fmt.Sprintf("Attached file: %s", rendered.Filename),
			fmt.Sprintf("Draft version: %d", version),
		},
		ActionLines: []string{
			"Review the attachment before filing it with Skatteverket.",
			"Use POST /admin/filings/mark-submitted?kind=...&period=... after the filing has been submitted so no further reminders are sent for the same period.",
		},
		Attachments: []notify.Attachment{{
			Filename:    rendered.Filename,
			ContentType: "text/plain",
			Content:     rendered.Content,
		}},
	})
}

func (s *Service) sendZeroReminderNotification(ctx context.Context, filingPeriod domain.FilingPeriod, reviewEntries int) error {
	if s.Notify == nil {
		return nil
	}
	return s.Notify.Send(ctx, notify.Notification{
		Severity:  notify.SeverityInfo,
		Category:  notify.CategoryFilingZeroReminder,
		CompanyID: s.Config.Bokio.CompanyID.String(),
		To:        append([]string(nil), s.Config.Filings.EmailTo...),
		Subject:   fmt.Sprintf("OSS nil return reminder for %s", filingPeriod.Period),
		SummaryLines: []string{
			fmt.Sprintf("No OSS-ready sales were found for %s, but a nil OSS return is still due.", filingPeriod.Period),
			fmt.Sprintf("Deadline: %s", filingPeriod.DeadlineDate.In(support.LocationOrUTC(s.Config)).Format("2006-01-02")),
			fmt.Sprintf("Review entries excluded from export: %d", reviewEntries),
		},
		ActionLines: []string{
			"File the nil OSS return in Skatteverket's e-service before the deadline.",
			"Use POST /admin/filings/mark-submitted?kind=oss_union&period=... after submitting it.",
		},
	})
}

func (s *Service) sendFailureNotification(ctx context.Context, kind, period string, cause error) error {
	if s.Notify == nil || cause == nil {
		return nil
	}
	return s.Notify.Send(ctx, notify.Notification{
		Severity:  notify.SeverityError,
		Category:  notify.CategoryFilingFailure,
		CompanyID: s.Config.Bokio.CompanyID.String(),
		To:        append([]string(nil), s.Config.Filings.EmailTo...),
		Subject:   fmt.Sprintf("%s generation failed for %s", filingLabel(kind), period),
		SummaryLines: []string{
			fmt.Sprintf("Automatic filing generation failed for %s %s.", filingLabel(kind), period),
			fmt.Sprintf("Error: %s", cause.Error()),
		},
		ActionLines: []string{
			"Check the application logs and the stored Stripe evidence for the affected period.",
			"Rerun the export through POST /admin/runs/filing?kind=...&period=... after correcting the issue.",
		},
	})
}
