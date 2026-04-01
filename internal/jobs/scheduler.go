package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ecleangg/booky/internal/accounting"
	"github.com/ecleangg/booky/internal/config"
	"github.com/ecleangg/booky/internal/filings"
	"github.com/ecleangg/booky/internal/notify"
)

type Scheduler struct {
	Config  config.Config
	Service *accounting.Service
	Filings *filings.Service
	Logger  *slog.Logger
}

func NewScheduler(cfg config.Config, service *accounting.Service, filingsService *filings.Service, logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{Config: cfg, Service: service, Filings: filingsService, Logger: logger}
}

func (s *Scheduler) Run(ctx context.Context) error {
	if !s.Config.Posting.SchedulerEnabled {
		return nil
	}

	ticker := time.NewTicker(s.Config.SchedulerInterval())
	defer ticker.Stop()

	s.tryRun(ctx)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			s.tryRun(ctx)
		}
	}
}

func (s *Scheduler) tryRun(ctx context.Context) {
	loc, err := s.Config.Location()
	if err != nil {
		s.Logger.Error("scheduler timezone error", "error", err)
		s.notifySchedulerIssue(ctx, notify.SeverityCritical, notify.CategorySchedulerConfig, "Scheduler configuration error", []string{
			fmt.Sprintf("Scheduler could not resolve configured timezone: %s", err.Error()),
			"Automatic bookkeeping runs are not executing until this is corrected.",
		})
		return
	}
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	yesterday := today.AddDate(0, 0, -1)

	toRun := []time.Time{yesterday}
	hour, minute, err := s.Config.CutoffHourMinute()
	if err == nil && (now.Hour() > hour || (now.Hour() == hour && now.Minute() >= minute)) {
		toRun = append(toRun, today)
	}

	for _, postingDate := range toRun {
		if err := s.Service.RunDailyClose(ctx, postingDate); err != nil && !shouldIgnoreScheduleError(err) {
			s.Logger.Error("scheduled daily close failed", "posting_date", postingDate.Format("2006-01-02"), "error", err)
			date := postingDate
			s.notifySchedulerIssue(ctx, notify.SeverityError, notify.CategorySchedulerFailure, fmt.Sprintf("Scheduled daily close failed for %s", postingDate.Format("2006-01-02")), []string{
				fmt.Sprintf("Scheduled run failed: %s", err.Error()),
				"Investigate promptly so bookkeeping is completed without delay.",
			}, &date)
		}
	}
	if s.Filings != nil && s.Filings.Enabled() {
		if err := s.Filings.EvaluateDuePeriods(ctx, now); err != nil && !shouldIgnoreScheduleError(err) {
			s.Logger.Error("scheduled filing evaluation failed", "error", err)
			s.notifySchedulerIssue(ctx, notify.SeverityError, notify.CategorySchedulerFailure, "Scheduled filing evaluation failed", []string{
				fmt.Sprintf("Automatic filing evaluation failed: %s", err.Error()),
				"Investigate promptly so filing drafts are sent before the reporting deadline.",
			})
		}
	}
}

func shouldIgnoreScheduleError(err error) bool {
	if err == nil {
		return true
	}
	return errors.Is(err, context.Canceled) ||
		strings.Contains(err.Error(), "already exists") ||
		strings.Contains(err.Error(), "already running") ||
		strings.Contains(err.Error(), "already completed")
}

func (s *Scheduler) notifySchedulerIssue(ctx context.Context, severity notify.Severity, category notify.Category, subject string, summary []string, postingDate ...*time.Time) {
	if s.Service == nil || s.Service.Notify == nil {
		return
	}
	var date *time.Time
	if len(postingDate) > 0 {
		date = postingDate[0]
	}
	if err := s.Service.Notify.Send(ctx, notify.Notification{
		Severity:     severity,
		Category:     category,
		PostingDate:  date,
		CompanyID:    s.Config.Bokio.CompanyID.String(),
		Subject:      subject,
		SummaryLines: summary,
	}); err != nil && s.Logger != nil {
		s.Logger.Error("send scheduler notification", "category", category, "error", err)
	}
}
