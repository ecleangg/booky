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
	"github.com/ecleangg/booky/internal/integrations"
	"github.com/ecleangg/booky/internal/notify"
)

type Scheduler struct {
	Config  config.Config
	Service *accounting.Service
	Filings *filings.Service
	Tenants *integrations.Service
	Logger  *slog.Logger
}

func NewScheduler(cfg config.Config, service *accounting.Service, filingsService *filings.Service, tenants *integrations.Service, logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{Config: cfg, Service: service, Filings: filingsService, Tenants: tenants, Logger: logger}
}

func (s *Scheduler) Run(ctx context.Context) error {
	if !s.Config.Posting.SchedulerEnabled && (s.Tenants == nil || !s.Tenants.Enabled()) {
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
		notifier := notify.NewNotifier(s.Config.Notifications)
		if notifier == nil && s.Service != nil {
			notifier = s.Service.Notify
		}
		s.Logger.Error("scheduler timezone error", "error", err)
		s.notifySchedulerIssue(ctx, s.Config, notifier, notify.SeverityCritical, notify.CategorySchedulerConfig, "Scheduler configuration error", []string{
			fmt.Sprintf("Scheduler could not resolve configured timezone: %s", err.Error()),
			"Automatic bookkeeping runs are not executing until this is corrected.",
		})
		return
	}
	now := time.Now().In(loc)
	if s.Tenants == nil || !s.Tenants.Enabled() {
		s.tryRunLegacy(ctx, now)
		return
	}
	s.tryRunTenants(ctx, now)
}

func (s *Scheduler) tryRunLegacy(ctx context.Context, now time.Time) {
	for _, postingDate := range postingDatesToRun(now, s.Config) {
		if err := s.Service.RunDailyClose(ctx, postingDate); err != nil && !shouldIgnoreScheduleError(err) {
			s.Logger.Error("scheduled daily close failed", "posting_date", postingDate.Format("2006-01-02"), "error", err)
			date := postingDate
			s.notifySchedulerIssue(ctx, s.Config, s.Service.Notify, notify.SeverityError, notify.CategorySchedulerFailure, fmt.Sprintf("Scheduled daily close failed for %s", postingDate.Format("2006-01-02")), []string{
				fmt.Sprintf("Scheduled run failed: %s", err.Error()),
				"Investigate promptly so bookkeeping is completed without delay.",
			}, &date)
		}
	}
	if s.Filings != nil && s.Filings.Enabled() {
		if err := s.Filings.EvaluateDuePeriods(ctx, now); err != nil && !shouldIgnoreScheduleError(err) {
			s.Logger.Error("scheduled filing evaluation failed", "error", err)
			s.notifySchedulerIssue(ctx, s.Config, s.Service.Notify, notify.SeverityError, notify.CategorySchedulerFailure, "Scheduled filing evaluation failed", []string{
				fmt.Sprintf("Automatic filing evaluation failed: %s", err.Error()),
				"Investigate promptly so filing drafts are sent before the reporting deadline.",
			})
		}
	}
}

func (s *Scheduler) tryRunTenants(ctx context.Context, now time.Time) {
	runtimes, err := s.Tenants.ListActiveRuntimes(ctx)
	if err != nil {
		notifier := notify.NewNotifier(s.Config.Notifications)
		if notifier == nil && s.Service != nil {
			notifier = s.Service.Notify
		}
		s.Logger.Error("list active runtimes failed", "error", err)
		s.notifySchedulerIssue(ctx, s.Config, notifier, notify.SeverityError, notify.CategorySchedulerFailure, "Scheduled runtime listing failed", []string{
			fmt.Sprintf("Failed to list active tenant runtimes: %s", err.Error()),
			"Investigate promptly so automatic bookkeeping continues for connected companies.",
		})
		return
	}

	for _, runtime := range runtimes {
		cfg := runtime.Apply(s.Config)
		notifier := notify.NewNotifier(cfg.Notifications)
		for _, postingDate := range postingDatesToRun(now, cfg) {
			if err := s.Service.RunDailyCloseForCompany(ctx, runtime.BokioCompanyID, postingDate); err != nil && !shouldIgnoreScheduleError(err) {
				s.Logger.Error("scheduled daily close failed", "company_id", runtime.BokioCompanyID, "posting_date", postingDate.Format("2006-01-02"), "error", err)
				date := postingDate
				s.notifySchedulerIssue(ctx, cfg, notifier, notify.SeverityError, notify.CategorySchedulerFailure, fmt.Sprintf("Scheduled daily close failed for %s", postingDate.Format("2006-01-02")), []string{
					fmt.Sprintf("Scheduled run failed: %s", err.Error()),
					"Investigate promptly so bookkeeping is completed without delay.",
				}, &date)
			}
		}
		if s.Filings != nil && cfg.Filings.Enabled {
			if err := s.Filings.EvaluateDuePeriodsForCompany(ctx, runtime.BokioCompanyID, now); err != nil && !shouldIgnoreScheduleError(err) {
				s.Logger.Error("scheduled filing evaluation failed", "company_id", runtime.BokioCompanyID, "error", err)
				s.notifySchedulerIssue(ctx, cfg, notifier, notify.SeverityError, notify.CategorySchedulerFailure, "Scheduled filing evaluation failed", []string{
					fmt.Sprintf("Automatic filing evaluation failed: %s", err.Error()),
					"Investigate promptly so filing drafts are sent before the reporting deadline.",
				})
			}
		}
	}
}

func postingDatesToRun(now time.Time, cfg config.Config) []time.Time {
	if !cfg.Posting.SchedulerEnabled {
		return nil
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)

	toRun := []time.Time{yesterday}
	hour, minute, err := cfg.CutoffHourMinute()
	if err == nil && (now.Hour() > hour || (now.Hour() == hour && now.Minute() >= minute)) {
		toRun = append(toRun, today)
	}
	return toRun
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

func (s *Scheduler) notifySchedulerIssue(ctx context.Context, cfg config.Config, notifier notify.Notifier, severity notify.Severity, category notify.Category, subject string, summary []string, postingDate ...*time.Time) {
	if notifier == nil {
		return
	}
	var date *time.Time
	if len(postingDate) > 0 {
		date = postingDate[0]
	}
	if err := notifier.Send(ctx, notify.Notification{
		Severity:     severity,
		Category:     category,
		PostingDate:  date,
		CompanyID:    cfg.Bokio.CompanyID.String(),
		Subject:      subject,
		SummaryLines: summary,
	}); err != nil && s.Logger != nil {
		s.Logger.Error("send scheduler notification", "category", category, "error", err)
	}
}
