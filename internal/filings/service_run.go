package filings

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/store"
	"github.com/ecleangg/booky/internal/support"
)

func (s *Service) SyncWebhookEntries(ctx context.Context, snapshots []domain.ObjectSnapshot, facts []domain.AccountingFact) error {
	if !s.Enabled() {
		return nil
	}

	sourceGroups := filingSourceGroups(facts)
	ossEntries, psEntries, periods, err := s.BuildWebhookEntries(ctx, snapshots, facts)
	if err != nil {
		return err
	}

	return s.Repo.InTx(ctx, func(q *store.Queries) error {
		if err := q.DeleteOSSUnionEntriesBySourceGroups(ctx, sourceGroups); err != nil {
			return err
		}
		if err := q.DeletePeriodicSummaryEntriesBySourceGroups(ctx, sourceGroups); err != nil {
			return err
		}
		if len(ossEntries) > 0 {
			if err := q.UpsertOSSUnionEntries(ctx, ossEntries); err != nil {
				return err
			}
		}
		if len(psEntries) > 0 {
			if err := q.UpsertPeriodicSummaryEntries(ctx, psEntries); err != nil {
				return err
			}
		}
		if len(periods) > 0 {
			if err := q.UpsertFilingPeriods(ctx, periods); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) BackfillUpcoming(ctx context.Context, now time.Time) error {
	if !s.Enabled() {
		return nil
	}
	for _, candidate := range s.backfillCandidates(now, now.AddDate(0, 0, 60)) {
		if err := s.BackfillPeriod(ctx, candidate.Kind, candidate.Period); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) BackfillPeriod(ctx context.Context, kind, period string) error {
	if !s.Enabled() {
		return nil
	}
	start, end, err := periodDateRange(kind, period, s.Config)
	if err != nil {
		return err
	}

	facts, err := s.Repo.Queries().ListFilingRelevantFactsByDateRange(ctx, s.Config.Bokio.CompanyID, start, end)
	if err != nil {
		return err
	}
	if len(facts) == 0 {
		if kind == domain.FilingKindOSSUnion {
			return s.Repo.InTx(ctx, func(q *store.Queries) error {
				return q.UpsertFilingPeriods(ctx, []domain.FilingPeriod{s.periodForKind(kind, period)})
			})
		}
		return nil
	}

	source := newRepositorySource(s.Repo)
	ossEntries, psEntries, periods, err := s.buildEntries(ctx, facts, source)
	if err != nil {
		return err
	}

	targetGroups := filingSourceGroups(facts)
	filteredOSS := filterOSSPeriodEntries(ossEntries, kind, period)
	filteredPS := filterPSPeriodEntries(psEntries, kind, period)
	filteredPeriods := filterPeriods(periods, kind, period)
	if len(filteredPeriods) == 0 {
		filteredPeriods = []domain.FilingPeriod{s.periodForKind(kind, period)}
	}

	return s.Repo.InTx(ctx, func(q *store.Queries) error {
		switch kind {
		case domain.FilingKindOSSUnion:
			if err := q.DeleteOSSUnionEntriesBySourceGroups(ctx, targetGroups); err != nil {
				return err
			}
			if len(filteredOSS) > 0 {
				if err := q.UpsertOSSUnionEntries(ctx, filteredOSS); err != nil {
					return err
				}
			}
		case domain.FilingKindPeriodicSummary:
			if err := q.DeletePeriodicSummaryEntriesBySourceGroups(ctx, targetGroups); err != nil {
				return err
			}
			if len(filteredPS) > 0 {
				if err := q.UpsertPeriodicSummaryEntries(ctx, filteredPS); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unsupported filing kind %q", kind)
		}
		return q.UpsertFilingPeriods(ctx, filteredPeriods)
	})
}

func (s *Service) EvaluateDuePeriods(ctx context.Context, now time.Time) error {
	if !s.Enabled() {
		return nil
	}
	if err := s.ensureScheduledPeriods(ctx, now); err != nil {
		return err
	}

	periods, err := s.Repo.Queries().ListDueFilingPeriods(ctx, s.Config.Bokio.CompanyID, now)
	if err != nil {
		return err
	}
	var errs []error
	for _, period := range periods {
		if _, err := s.evaluatePeriod(ctx, period.Kind, period.Period, false, now); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *Service) RunPeriod(ctx context.Context, kind, period string) (*domain.FilingExport, error) {
	if !s.Enabled() {
		return nil, nil
	}
	if err := s.BackfillPeriod(ctx, kind, period); err != nil {
		return nil, err
	}
	return s.evaluatePeriod(ctx, kind, period, true, time.Now().In(support.LocationOrUTC(s.Config)))
}

func (s *Service) MarkSubmitted(ctx context.Context, kind, period string, submittedAt time.Time) error {
	if !s.Enabled() {
		return nil
	}
	return s.Repo.InTx(ctx, func(q *store.Queries) error {
		return q.MarkFilingPeriodSubmitted(ctx, s.Config.Bokio.CompanyID, kind, period, submittedAt.UTC())
	})
}

func (s *Service) GetStatus(ctx context.Context, kind, period string) (PeriodStatus, error) {
	if !s.Enabled() {
		return PeriodStatus{}, store.ErrNotFound
	}
	if err := s.ensurePeriod(ctx, kind, period); err != nil {
		return PeriodStatus{}, err
	}

	filingPeriod, err := s.Repo.Queries().GetFilingPeriod(ctx, s.Config.Bokio.CompanyID, kind, period)
	if err != nil {
		return PeriodStatus{}, err
	}

	ready, review, err := s.periodEntryCounts(ctx, kind, period)
	if err != nil {
		return PeriodStatus{}, err
	}

	status := PeriodStatus{
		Period:        filingPeriod,
		ReadyEntries:  ready,
		ReviewEntries: review,
	}
	if latest, err := s.Repo.Queries().GetLatestFilingExport(ctx, s.Config.Bokio.CompanyID, kind, period); err == nil {
		status.LatestExport = &latest
	} else if err != nil && err != store.ErrNotFound {
		return PeriodStatus{}, err
	}
	return status, nil
}

func (s *Service) evaluatePeriod(ctx context.Context, kind, period string, force bool, now time.Time) (*domain.FilingExport, error) {
	if err := s.ensurePeriod(ctx, kind, period); err != nil {
		return nil, err
	}

	filingPeriod, err := s.Repo.Queries().GetFilingPeriod(ctx, s.Config.Bokio.CompanyID, kind, period)
	if err != nil {
		return nil, err
	}
	if filingPeriod.SubmittedAt != nil {
		return nil, nil
	}
	if !force && now.Before(filingPeriod.FirstSendAt) {
		return nil, nil
	}

	var (
		reviewEntries int
		latest        domain.FilingExport
		hasLatest     bool
	)
	if current, err := s.Repo.Queries().GetLatestFilingExport(ctx, s.Config.Bokio.CompanyID, kind, period); err == nil {
		latest = current
		hasLatest = true
	} else if err != nil && err != store.ErrNotFound {
		return nil, err
	}

	switch kind {
	case domain.FilingKindOSSUnion:
		entries, err := s.Repo.Queries().ListOSSUnionEntriesByPeriod(ctx, s.Config.Bokio.CompanyID, period)
		if err != nil {
			return nil, s.failEvaluation(ctx, kind, period, now, err)
		}
		readyEntries := make([]domain.OSSUnionEntry, 0)
		for _, entry := range entries {
			if entry.ReviewState == domain.FilingReviewStateReady {
				readyEntries = append(readyEntries, entry)
			} else {
				reviewEntries++
			}
		}
		if len(readyEntries) == 0 {
			if filingPeriod.ZeroReminderSentAt != nil {
				return nil, s.Repo.InTx(ctx, func(q *store.Queries) error {
					return q.UpdateFilingPeriodEvaluation(ctx, s.Config.Bokio.CompanyID, kind, period, domain.FilingPeriodStatusUnchanged, now.UTC(), nil)
				})
			}
			return s.sendZeroReminder(ctx, filingPeriod, reviewEntries, hasLatest, latest, now)
		}
		rendered, err := RenderOSSUnion(s.Config, period, readyEntries)
		if err != nil {
			return nil, s.failEvaluation(ctx, kind, period, now, err)
		}
		return s.sendRenderedExport(ctx, filingPeriod, rendered, len(readyEntries), reviewEntries, hasLatest, latest, now)
	case domain.FilingKindPeriodicSummary:
		entries, err := s.Repo.Queries().ListPeriodicSummaryEntriesByPeriod(ctx, s.Config.Bokio.CompanyID, period)
		if err != nil {
			return nil, s.failEvaluation(ctx, kind, period, now, err)
		}
		readyEntries := make([]domain.PeriodicSummaryEntry, 0)
		for _, entry := range entries {
			if entry.ReviewState == domain.FilingReviewStateReady {
				readyEntries = append(readyEntries, entry)
			} else {
				reviewEntries++
			}
		}
		if len(readyEntries) == 0 {
			return nil, s.Repo.InTx(ctx, func(q *store.Queries) error {
				return q.UpdateFilingPeriodEvaluation(ctx, s.Config.Bokio.CompanyID, kind, period, domain.FilingPeriodStatusUnchanged, now.UTC(), nil)
			})
		}
		rendered, err := RenderPeriodicSummary(s.Config, period, readyEntries)
		if err != nil {
			return nil, s.failEvaluation(ctx, kind, period, now, err)
		}
		return s.sendRenderedExport(ctx, filingPeriod, rendered, len(readyEntries), reviewEntries, hasLatest, latest, now)
	default:
		return nil, fmt.Errorf("unsupported filing kind %q", kind)
	}
}

func (s *Service) ensureScheduledPeriods(ctx context.Context, now time.Time) error {
	periodMap := map[string]domain.FilingPeriod{}
	if s.Config.Filings.OSSUnion.Enabled {
		for _, period := range completedQuarterPeriods(now.In(support.LocationOrUTC(s.Config)), 8) {
			periodMap[periodKey(domain.FilingKindOSSUnion, period)] = s.periodForKind(domain.FilingKindOSSUnion, period)
		}
	}
	if s.Config.Filings.PeriodicSummary.Enabled {
		periods, err := s.Repo.Queries().ListEntryPeriods(ctx, s.Config.Bokio.CompanyID, domain.FilingKindPeriodicSummary)
		if err != nil {
			return err
		}
		for _, period := range periods {
			periodMap[periodKey(domain.FilingKindPeriodicSummary, period)] = s.periodForKind(domain.FilingKindPeriodicSummary, period)
		}
	}

	if len(periodMap) == 0 {
		return nil
	}
	periods := make([]domain.FilingPeriod, 0, len(periodMap))
	for _, period := range periodMap {
		periods = append(periods, period)
	}
	return s.Repo.InTx(ctx, func(q *store.Queries) error {
		return q.UpsertFilingPeriods(ctx, periods)
	})
}

func (s *Service) ensurePeriod(ctx context.Context, kind, period string) error {
	if _, err := s.Repo.Queries().GetFilingPeriod(ctx, s.Config.Bokio.CompanyID, kind, period); err == nil {
		return nil
	} else if err != store.ErrNotFound {
		return err
	}
	return s.Repo.InTx(ctx, func(q *store.Queries) error {
		return q.UpsertFilingPeriods(ctx, []domain.FilingPeriod{s.periodForKind(kind, period)})
	})
}

func (s *Service) periodForKind(kind, period string) domain.FilingPeriod {
	loc := support.LocationOrUTC(s.Config)
	deadline, err := filingDeadline(kind, period, loc)
	if err != nil {
		deadline = time.Time{}
	}
	firstSendAt, err := filingFirstSendAt(kind, period, s.Config)
	if err != nil {
		firstSendAt = time.Time{}
	}
	return domain.FilingPeriod{
		Kind:                 kind,
		Period:               period,
		BokioCompanyID:       s.Config.Bokio.CompanyID,
		DeadlineDate:         deadline,
		FirstSendAt:          firstSendAt,
		LastEvaluationStatus: domain.FilingPeriodStatusPending,
	}
}

func (s *Service) backfillCandidates(now, until time.Time) []domain.FilingPeriod {
	loc := support.LocationOrUTC(s.Config)
	now = now.In(loc)
	until = until.In(loc)

	var periods []domain.FilingPeriod
	if s.Config.Filings.OSSUnion.Enabled {
		for _, period := range completedQuarterPeriods(now, 6) {
			deadline, err := filingDeadline(domain.FilingKindOSSUnion, period, loc)
			if err == nil && !deadline.Before(now) && !deadline.After(until) {
				periods = append(periods, s.periodForKind(domain.FilingKindOSSUnion, period))
			}
		}
	}
	if s.Config.Filings.PeriodicSummary.Enabled {
		currentMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
		for cursor := currentMonthStart.AddDate(0, -6, 0); cursor.Before(currentMonthStart); cursor = cursor.AddDate(0, 1, 0) {
			period := monthPeriod(cursor)
			deadline, err := filingDeadline(domain.FilingKindPeriodicSummary, period, loc)
			if err == nil && !deadline.Before(now) && !deadline.After(until) {
				periods = append(periods, s.periodForKind(domain.FilingKindPeriodicSummary, period))
			}
		}
	}
	sort.Slice(periods, func(i, j int) bool {
		if periods[i].Kind == periods[j].Kind {
			return periods[i].Period < periods[j].Period
		}
		return periods[i].Kind < periods[j].Kind
	})
	return periods
}

func (s *Service) periodEntryCounts(ctx context.Context, kind, period string) (int, int, error) {
	switch kind {
	case domain.FilingKindOSSUnion:
		entries, err := s.Repo.Queries().ListOSSUnionEntriesByPeriod(ctx, s.Config.Bokio.CompanyID, period)
		if err != nil {
			return 0, 0, err
		}
		return countEntryStates(entries), countEntryReviewStates(entries), nil
	case domain.FilingKindPeriodicSummary:
		entries, err := s.Repo.Queries().ListPeriodicSummaryEntriesByPeriod(ctx, s.Config.Bokio.CompanyID, period)
		if err != nil {
			return 0, 0, err
		}
		return countPSEntryStates(entries), countPSReviewStates(entries), nil
	default:
		return 0, 0, fmt.Errorf("unsupported filing kind %q", kind)
	}
}

func nextExportVersion(hasLatest bool, latest domain.FilingExport) int {
	if !hasLatest {
		return 1
	}
	return latest.Version + 1
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func checksumBytes(content []byte) string {
	sum := sha1.Sum(content)
	return hex.EncodeToString(sum[:])
}

func filingLabel(kind string) string {
	switch kind {
	case domain.FilingKindOSSUnion:
		return "OSS union filing"
	case domain.FilingKindPeriodicSummary:
		return "Periodisk sammanstallning"
	default:
		return kind
	}
}

func countEntryStates(entries []domain.OSSUnionEntry) int {
	count := 0
	for _, entry := range entries {
		if entry.ReviewState == domain.FilingReviewStateReady {
			count++
		}
	}
	return count
}

func countEntryReviewStates(entries []domain.OSSUnionEntry) int {
	count := 0
	for _, entry := range entries {
		if entry.ReviewState == domain.FilingReviewStateReview {
			count++
		}
	}
	return count
}

func countPSEntryStates(entries []domain.PeriodicSummaryEntry) int {
	count := 0
	for _, entry := range entries {
		if entry.ReviewState == domain.FilingReviewStateReady {
			count++
		}
	}
	return count
}

func countPSReviewStates(entries []domain.PeriodicSummaryEntry) int {
	count := 0
	for _, entry := range entries {
		if entry.ReviewState == domain.FilingReviewStateReview {
			count++
		}
	}
	return count
}
