package stripe

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/notify"
)

func (s *Service) notifyWebhookFailure(ctx context.Context, evt Event, err error) {
	if s.Notify == nil || err == nil {
		return
	}
	if err := s.Notify.Send(ctx, notify.Notification{
		Severity:  notify.SeverityError,
		Category:  notify.CategoryWebhookIngestionError,
		CompanyID: s.Config.Bokio.CompanyID.String(),
		Subject:   fmt.Sprintf("Stripe webhook processing failed: %s", evt.Type),
		SummaryLines: []string{
			fmt.Sprintf("Webhook event %s could not be processed.", evt.ID),
			fmt.Sprintf("Error: %s", err.Error()),
			"Investigate promptly so the source transaction is not missed in bookkeeping.",
		},
		DetailLines: []string{
			fmt.Sprintf("Stripe event type: %s", evt.Type),
			fmt.Sprintf("Stripe account: %s", stripeAccountID(evt)),
		},
		ActionLines: []string{
			"Check the application logs for the full processing error and confirm the Stripe event payload is valid for the current code/config.",
			"If the failure is due to missing config or Bokio account mapping, correct config first and then resend the webhook or trigger a fresh Stripe update for the same object.",
			"Do not insert or edit bookkeeping facts directly in PostgreSQL as a normal recovery step.",
		},
	}); err != nil && s.Logger != nil {
		s.Logger.ErrorContext(ctx, "send webhook failure notification", "event_id", evt.ID, "error", err)
	}
}

func (s *Service) notifyWebhookReviewFacts(ctx context.Context, evt Event, facts []domain.AccountingFact) {
	if s.Notify == nil {
		return
	}
	details := webhookReviewLines(facts)
	if len(details) == 0 {
		return
	}
	if err := s.Notify.Send(ctx, notify.Notification{
		Severity:  notify.SeverityWarning,
		Category:  notify.CategoryWebhookReviewRequired,
		CompanyID: s.Config.Bokio.CompanyID.String(),
		Subject:   fmt.Sprintf("Stripe transaction needs accounting review: %s", evt.Type),
		SummaryLines: []string{
			fmt.Sprintf("Webhook event %s produced %d review concern(s).", evt.ID, len(details)),
			"Review these transactions before the next bookkeeping close so evidence and VAT treatment stay complete.",
		},
		DetailLines: details,
		ActionLines: []string{
			"Open the charge/refund/dispute/payout in Stripe and compare it with the review_reason in this alert.",
			"If the problem is missing VAT, country, or customer evidence, correct that in Stripe or your upstream integration and trigger a fresh Stripe update so booky rebuilds the facts.",
			"If the problem is missing Bokio account mapping, update the service config and rerun the accounting day after a fresh normalization event exists.",
			"Daily close is rerun through POST /admin/runs/daily-close?date=YYYY-MM-DD with Authorization: Bearer <BOOKY_ADMIN_TOKEN>.",
			"There is currently no separate approve-in-review UI; the supported workflow is to correct the source/config and let a new normalization replace the review facts.",
		},
	}); err != nil && s.Logger != nil {
		s.Logger.ErrorContext(ctx, "send webhook review notification", "event_id", evt.ID, "error", err)
	}
}

func webhookReviewLines(facts []domain.AccountingFact) []string {
	set := map[string]struct{}{}
	for _, fact := range facts {
		if fact.Status != domain.FactStatusNeedsReview {
			continue
		}
		line := fmt.Sprintf("Source group %s (%s) is marked needs_review", fact.SourceGroupID, fact.FactType)
		if fact.ReviewReason != nil && strings.TrimSpace(*fact.ReviewReason) != "" {
			line += ": " + strings.TrimSpace(*fact.ReviewReason)
		}
		set[line] = struct{}{}
	}
	lines := make([]string, 0, len(set))
	for line := range set {
		lines = append(lines, line)
	}
	sort.Strings(lines)
	return lines
}
