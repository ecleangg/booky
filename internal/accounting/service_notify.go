package accounting

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ecleangg/booky/internal/config"
	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/notify"
)

func (s *Service) notifyFailure(ctx context.Context, postingDate time.Time, err error, facts []domain.AccountingFact) {
	if s.Notify == nil || err == nil {
		return
	}
	category := notify.CategoryDailyCloseFailure
	subject := fmt.Sprintf("Daily close failed for %s", postingDate.Format("2006-01-02"))
	summary := []string{
		fmt.Sprintf("Daily close failed with error: %s", err.Error()),
		fmt.Sprintf("Pending fact count: %d", len(facts)),
	}
	details := complianceConcernLines(facts, nil, s.Config.Accounts.FallbackOBS)
	if hasManualReviewBlock(err) {
		category = notify.CategoryManualReviewRequired
		subject = fmt.Sprintf("Manual review required before posting %s", postingDate.Format("2006-01-02"))
		summary = append(summary, "Posting is blocked until the flagged transactions are reviewed and corrected.")
	}
	actions := notificationActionLines(s.Config, category, postingDate, facts)
	if err := s.Notify.Send(ctx, notify.Notification{
		Severity:     notify.SeverityError,
		Category:     category,
		PostingDate:  &postingDate,
		CompanyID:    s.Config.Bokio.CompanyID.String(),
		Subject:      subject,
		SummaryLines: summary,
		DetailLines:  details,
		ActionLines:  actions,
	}); err != nil {
		s.Logger.ErrorContext(ctx, "send failure notification", "posting_date", postingDate.Format("2006-01-02"), "error", err)
	}
}

func (s *Service) notifyWarnings(ctx context.Context, postingDate time.Time, facts []domain.AccountingFact, roundingFact *domain.AccountingFact) {
	if s.Notify == nil {
		return
	}
	details := complianceConcernLines(facts, roundingFact, s.Config.Accounts.FallbackOBS)
	if len(details) == 0 {
		return
	}
	if err := s.Notify.Send(ctx, notify.Notification{
		Severity:    notify.SeverityWarning,
		Category:    notify.CategoryDailyCloseWarning,
		PostingDate: &postingDate,
		CompanyID:   s.Config.Bokio.CompanyID.String(),
		Subject:     fmt.Sprintf("Daily close warnings for %s", postingDate.Format("2006-01-02")),
		SummaryLines: []string{
			fmt.Sprintf("Daily close completed with %d warning(s)", len(details)),
			fmt.Sprintf("Posted fact count: %d", len(facts)),
		},
		DetailLines: details,
		ActionLines: notificationActionLines(s.Config, notify.CategoryDailyCloseWarning, postingDate, facts),
	}); err != nil {
		s.Logger.ErrorContext(ctx, "send warning notification", "posting_date", postingDate.Format("2006-01-02"), "error", err)
	}
}

func complianceConcernLines(facts []domain.AccountingFact, roundingFact *domain.AccountingFact, fallbackOBS int) []string {
	set := map[string]struct{}{}
	for _, fact := range facts {
		if fallbackOBS != 0 && fact.BokioAccount == fallbackOBS {
			set[fmt.Sprintf("Fact %s posted to fallback OBS account and should be reviewed against supporting evidence", fact.SourceGroupID)] = struct{}{}
		}
		if fact.ReviewReason != nil && *fact.ReviewReason != "" {
			set[fmt.Sprintf("Fact %s review note: %s", fact.SourceGroupID, *fact.ReviewReason)] = struct{}{}
		}
		if fact.Status == domain.FactStatusNeedsReview {
			set[fmt.Sprintf("Fact %s remains in needs_review status and requires manual accounting follow-up", fact.SourceGroupID)] = struct{}{}
		}
	}
	if roundingFact != nil {
		set[fmt.Sprintf("Rounding line added for %.2f SEK; confirm the source evidence supports the final booked amount", float64(roundingFact.AmountSEKOre)/100.0)] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for line := range set {
		out = append(out, line)
	}
	sort.Strings(out)
	return out
}

func hasManualReviewBlock(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "need review") || strings.Contains(msg, "needs review") || strings.Contains(msg, "cannot be posted with zero sek amount")
}

func notificationActionLines(cfg config.Config, category notify.Category, postingDate time.Time, facts []domain.AccountingFact) []string {
	runCmd := fmt.Sprintf("Re-run daily close after correction: POST /admin/runs/daily-close?date=%s with Authorization: Bearer <BOOKY_ADMIN_TOKEN>.", postingDate.Format("2006-01-02"))
	common := []string{
		"Do not edit accounting_facts rows manually in PostgreSQL as a normal workflow.",
		"Treat Stripe/customer metadata, account mappings, or a Bokio-side correction journal as the real correction points.",
	}

	switch category {
	case notify.CategoryManualReviewRequired:
		return append(common,
			"Read the review_reason lines in this alert and identify whether the problem is missing Stripe evidence, missing account mapping, or a zero-SEK conversion gap.",
			"If the issue is missing Stripe/customer/VAT evidence, correct the source of truth in Stripe or your upstream integration so a fresh webhook can rebuild the facts.",
			"If the issue is a missing account mapping, update config/booky.yaml or the deployed config so the referenced Bokio account exists and is mapped correctly.",
			"If the economic event is already correctly booked manually in Bokio, post a correction there and then make sure booky will not try to auto-post the same source group again without updated source evidence.",
			"After correcting the source/config, trigger a fresh Stripe event for the affected object if possible, for example by updating the charge metadata so Stripe emits charge.updated.",
			runCmd,
		)
	case notify.CategoryDailyCloseWarning:
		return append(common,
			"Review any fallback OBS postings and rounding lines before relying on the day as fully finalised bookkeeping evidence.",
			"If a line is acceptable as posted, document why. If not, correct the underlying Stripe/config evidence and re-run the day after reversing the journal through the normal correction flow.",
			runCmd,
		)
	case notify.CategoryDailyCloseFailure:
		return append(common,
			"Fix the blocking error first, then rerun the same accounting day.",
			runCmd,
		)
	default:
		return common
	}
}
