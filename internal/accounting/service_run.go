package accounting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ecleangg/booky/internal/bokio"
	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/store"
	"github.com/google/uuid"
)

func (s *Service) RunDailyClose(ctx context.Context, postingDate time.Time) error {
	postingDate = dateOnly(postingDate)
	unlock, err := s.Repo.AcquireAdvisoryLock(ctx, int32(postingDate.Year()), int32(postingDate.YearDay()))
	if err != nil {
		if errors.Is(err, store.ErrLockBusy) {
			return fmt.Errorf("daily close already running for %s", postingDate.Format("2006-01-02"))
		}
		return err
	}
	defer func() {
		_ = unlock(context.Background())
	}()

	if err := s.resumeUploadIfNeeded(ctx, postingDate); err != nil {
		return err
	}

	facts, err := s.Repo.Queries().ListPendingAccountingFacts(ctx, s.Config.Bokio.CompanyID, postingDate)
	if err != nil {
		s.notifyFailure(ctx, postingDate, err, nil)
		return err
	}
	if len(facts) == 0 {
		return nil
	}
	if err := validateFactsForPosting(facts, s.Config.Posting.AutoPostUnknownToOBS); err != nil {
		s.notifyFailure(ctx, postingDate, err, facts)
		return err
	}

	configSnapshot, err := s.Config.SnapshotJSON()
	if err != nil {
		return fmt.Errorf("snapshot config: %w", err)
	}
	run := domain.PostingRun{
		ID:             uuid.New(),
		BokioCompanyID: s.Config.Bokio.CompanyID,
		PostingDate:    postingDate,
		Timezone:       s.Config.App.Timezone,
		RunType:        domain.PostingRunTypeDailyClose,
		SequenceNo:     1,
		Status:         domain.PostingRunStatusStarted,
		ConfigSnapshot: configSnapshot,
		Summary:        json.RawMessage(`{"status":"started"}`),
		StartedAt:      time.Now().UTC(),
	}
	run, err = s.ensurePostingRun(ctx, run)
	if err != nil {
		s.notifyFailure(ctx, postingDate, err, facts)
		return err
	}

	if err := s.Bokio.ValidateAccounts(ctx, collectAccounts(facts, s.Config.Accounts.Rounding)); err != nil {
		_ = s.failRun(ctx, run.ID, err)
		s.notifyFailure(ctx, postingDate, err, facts)
		return err
	}
	if err := s.Bokio.EnsureFiscalYearOpen(ctx, postingDate); err != nil {
		_ = s.failRun(ctx, run.ID, err)
		s.notifyFailure(ctx, postingDate, err, facts)
		return err
	}

	items, roundingFact, summary, err := AggregateJournalItems(facts, s.Config.Accounts.Rounding, s.Config.Posting.RoundingToleranceOre)
	if err != nil {
		_ = s.failRun(ctx, run.ID, err)
		s.notifyFailure(ctx, postingDate, err, facts)
		return err
	}
	if roundingFact != nil {
		roundingFact.BokioCompanyID = s.Config.Bokio.CompanyID
		roundingFact.StripeAccountID = "system"
		roundingFact.SourceObjectType = "system"
		roundingFact.SourceObjectID = "rounding"
		roundingFact.Status = domain.FactStatusPending
		facts = append(facts, *roundingFact)
	}
	summary["fact_count"] = len(facts)

	title := fmt.Sprintf("Stripe dagsverifikat %s", postingDate.Format("2006-01-02"))
	checksum, err := store.ChecksumJSON(map[string]any{
		"title":        title,
		"posting_date": postingDate.Format("2006-01-02"),
		"items":        items,
		"facts":        facts,
	})
	if err != nil {
		_ = s.failRun(ctx, run.ID, err)
		s.notifyFailure(ctx, postingDate, err, facts)
		return fmt.Errorf("checksum journal evidence: %w", err)
	}
	summary["journal_checksum"] = checksum
	draft := domain.JournalDraft{
		Title:       title,
		Date:        postingDate,
		GeneratedAt: run.StartedAt,
		Items:       items,
		Facts:       facts,
		Summary:     summary,
		PostingRun:  run,
		Description: fmt.Sprintf("Stripe dagsverifikat %s underlag", postingDate.Format("2006-01-02")),
	}

	pdfBytes, err := s.PDF.Render(draft)
	if err != nil {
		_ = s.failRun(ctx, run.ID, err)
		s.notifyFailure(ctx, postingDate, err, facts)
		return fmt.Errorf("render pdf: %w", err)
	}

	ids := collectFactIDs(facts)
	journalResp, err := s.ensureRemoteJournal(ctx, title, postingDate, items)
	if err != nil {
		_ = s.failRun(ctx, run.ID, err)
		s.notifyFailure(ctx, postingDate, err, facts)
		return err
	}
	journalRecord := domain.BokioJournal{
		PostingRunID:        run.ID,
		BokioCompanyID:      s.Config.Bokio.CompanyID,
		BokioJournalEntryID: journalResp.ID,
		BokioJournalEntryNo: journalResp.JournalEntryNumber,
		BokioJournalTitle:   title,
		PostingDate:         postingDate,
		AttachmentChecksum:  checksum,
	}
	if err := s.Repo.InTx(ctx, func(q *store.Queries) error {
		if roundingFact != nil {
			if err := q.UpsertAccountingFacts(ctx, []domain.AccountingFact{*roundingFact}); err != nil {
				return err
			}
		}
		if err := q.AttachFactsToRun(ctx, run.ID, ids); err != nil {
			return err
		}
		if err := q.MarkFactsStatus(ctx, ids, domain.FactStatusBatched); err != nil {
			return err
		}
		if err := q.UpsertBokioJournal(ctx, journalRecord); err != nil {
			return err
		}
		return q.UpdatePostingRun(ctx, run.ID, domain.PostingRunStatusJournalCreated, mustJSON(summary), nil)
	}); err != nil {
		s.notifyFailure(ctx, postingDate, err, facts)
		return err
	}

	uploadID, err := s.ensureRemoteUpload(ctx, journalResp.ID, draft.Description, title+".pdf", pdfBytes)
	if err != nil {
		_ = s.failRun(ctx, run.ID, err)
		s.notifyFailure(ctx, postingDate, err, facts)
		return err
	}
	journalRecord.BokioUploadID = &uploadID

	if err := s.Repo.InTx(ctx, func(q *store.Queries) error {
		if err := q.UpsertBokioJournal(ctx, journalRecord); err != nil {
			return err
		}
		if err := q.MarkFactsStatus(ctx, ids, domain.FactStatusPosted); err != nil {
			return err
		}
		return q.UpdatePostingRun(ctx, run.ID, domain.PostingRunStatusCompleted, mustJSON(summary), nil)
	}); err != nil {
		s.notifyFailure(ctx, postingDate, err, facts)
		return err
	}
	s.notifyWarnings(ctx, postingDate, facts, roundingFact)

	s.Logger.InfoContext(ctx, "daily close completed", "posting_date", postingDate.Format("2006-01-02"), "journal_entry_id", journalResp.ID)
	return nil
}

func (s *Service) ensurePostingRun(ctx context.Context, proposed domain.PostingRun) (domain.PostingRun, error) {
	existing, err := s.Repo.Queries().GetPostingRunByDate(ctx, proposed.BokioCompanyID, proposed.PostingDate, proposed.RunType)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return domain.PostingRun{}, err
		}
		if err := s.Repo.InTx(ctx, func(q *store.Queries) error {
			return q.CreatePostingRun(ctx, proposed)
		}); err != nil {
			if strings.Contains(err.Error(), "uq_posting_runs_daily_close") || strings.Contains(err.Error(), "duplicate key") {
				return domain.PostingRun{}, fmt.Errorf("posting run already exists for %s", proposed.PostingDate.Format("2006-01-02"))
			}
			return domain.PostingRun{}, err
		}
		return proposed, nil
	}

	switch existing.Status {
	case domain.PostingRunStatusCompleted:
		return domain.PostingRun{}, fmt.Errorf("posting run already completed for %s", proposed.PostingDate.Format("2006-01-02"))
	case domain.PostingRunStatusJournalCreated, domain.PostingRunStatusUploadCreated:
		return existing, nil
	case domain.PostingRunStatusFailed:
		if _, err := s.Repo.Queries().GetBokioJournalByRunID(ctx, existing.ID); err == nil {
			return existing, nil
		} else if !errors.Is(err, store.ErrNotFound) {
			return domain.PostingRun{}, err
		}
		if err := s.Repo.InTx(ctx, func(q *store.Queries) error {
			return q.ResetPostingRun(ctx, existing.ID, proposed.ConfigSnapshot, proposed.Summary)
		}); err != nil {
			return domain.PostingRun{}, err
		}
		existing.Status = domain.PostingRunStatusStarted
		existing.ConfigSnapshot = proposed.ConfigSnapshot
		existing.Summary = proposed.Summary
		existing.ErrorMessage = nil
		existing.FinishedAt = nil
		existing.StartedAt = time.Now().UTC()
		return existing, nil
	default:
		return domain.PostingRun{}, fmt.Errorf("posting run already exists for %s", proposed.PostingDate.Format("2006-01-02"))
	}
}

func (s *Service) resumeUploadIfNeeded(ctx context.Context, postingDate time.Time) error {
	run, err := s.Repo.Queries().GetPostingRunByDate(ctx, s.Config.Bokio.CompanyID, postingDate, domain.PostingRunTypeDailyClose)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return err
	}
	if run.Status != domain.PostingRunStatusJournalCreated && run.Status != domain.PostingRunStatusUploadCreated && run.Status != domain.PostingRunStatusFailed {
		return nil
	}

	journal, err := s.Repo.Queries().GetBokioJournalByRunID(ctx, run.ID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return err
	}
	if journal.BokioUploadID != nil {
		return nil
	}

	facts, err := s.Repo.Queries().ListFactsByRun(ctx, run.ID)
	if err != nil {
		return err
	}
	items, _, summary, err := AggregateJournalItems(facts, s.Config.Accounts.Rounding, s.Config.Posting.RoundingToleranceOre)
	if err != nil {
		return err
	}
	summary["fact_count"] = len(facts)
	checksum, err := store.ChecksumJSON(map[string]any{
		"title":        journal.BokioJournalTitle,
		"posting_date": postingDate.Format("2006-01-02"),
		"items":        items,
		"facts":        facts,
	})
	if err != nil {
		return err
	}
	summary["journal_checksum"] = checksum
	draft := domain.JournalDraft{
		Title:       journal.BokioJournalTitle,
		Date:        postingDate,
		GeneratedAt: run.StartedAt,
		Items:       items,
		Facts:       facts,
		Summary:     summary,
		PostingRun:  run,
		Description: fmt.Sprintf("%s underlag", journal.BokioJournalTitle),
	}
	pdfBytes, err := s.PDF.Render(draft)
	if err != nil {
		return err
	}
	uploadID, err := s.ensureRemoteUpload(ctx, journal.BokioJournalEntryID, draft.Description, draft.Title+".pdf", pdfBytes)
	if err != nil {
		return err
	}
	journal.BokioUploadID = &uploadID
	ids := collectFactIDs(facts)
	return s.Repo.InTx(ctx, func(q *store.Queries) error {
		if err := q.UpsertBokioJournal(ctx, journal); err != nil {
			return err
		}
		if err := q.MarkFactsStatus(ctx, ids, domain.FactStatusPosted); err != nil {
			return err
		}
		return q.UpdatePostingRun(ctx, run.ID, domain.PostingRunStatusCompleted, mustJSON(summary), nil)
	})
}

func (s *Service) ensureRemoteJournal(ctx context.Context, title string, postingDate time.Time, items []domain.JournalItem) (bokio.CreateJournalResponse, error) {
	match, err := s.Bokio.FindMatchingJournalEntry(ctx, title, postingDate, items)
	if err != nil {
		return bokio.CreateJournalResponse{}, err
	}
	if match != nil {
		return *match, nil
	}
	return s.Bokio.CreateJournalEntry(ctx, title, postingDate, items)
}

func (s *Service) ensureRemoteUpload(ctx context.Context, journalEntryID uuid.UUID, description, filename string, pdfBytes []byte) (uuid.UUID, error) {
	existing, err := s.Bokio.FindUploadByJournalEntryAndDescription(ctx, journalEntryID, description)
	if err != nil {
		return uuid.Nil, err
	}
	if existing != nil {
		return *existing, nil
	}
	return s.Bokio.UploadJournalAttachment(ctx, journalEntryID, description, filename, pdfBytes)
}

func validateFactsForPosting(facts []domain.AccountingFact, autoPostUnknown bool) error {
	reviewGroups := map[string]struct{}{}
	for _, fact := range facts {
		if fact.Status != domain.FactStatusNeedsReview {
			continue
		}
		reviewGroups[fact.SourceGroupID] = struct{}{}
		if fact.AmountSEKOre == 0 {
			reason := "needs review"
			if fact.ReviewReason != nil && *fact.ReviewReason != "" {
				reason = *fact.ReviewReason
			}
			return fmt.Errorf("fact %s cannot be posted with zero SEK amount: %s", fact.SourceGroupID, reason)
		}
	}
	if len(reviewGroups) > 0 && !autoPostUnknown {
		groups := make([]string, 0, len(reviewGroups))
		for group := range reviewGroups {
			groups = append(groups, group)
		}
		sort.Strings(groups)
		return fmt.Errorf("posting blocked because %d source group(s) need review and posting.auto_post_unknown_to_obs is false: %s", len(groups), strings.Join(groups, ", "))
	}
	return nil
}

func (s *Service) failRun(ctx context.Context, runID uuid.UUID, err error) error {
	msg := err.Error()
	return s.Repo.Queries().UpdatePostingRun(ctx, runID, domain.PostingRunStatusFailed, nil, &msg)
}

func collectAccounts(facts []domain.AccountingFact, extraAccounts ...int) []int {
	seen := map[int]struct{}{}
	accounts := make([]int, 0, len(facts))
	for _, fact := range facts {
		if _, ok := seen[fact.BokioAccount]; ok {
			continue
		}
		seen[fact.BokioAccount] = struct{}{}
		accounts = append(accounts, fact.BokioAccount)
	}
	for _, account := range extraAccounts {
		if account == 0 {
			continue
		}
		if _, ok := seen[account]; ok {
			continue
		}
		seen[account] = struct{}{}
		accounts = append(accounts, account)
	}
	return accounts
}

func collectFactIDs(facts []domain.AccountingFact) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(facts))
	for _, fact := range facts {
		ids = append(ids, fact.ID)
	}
	return ids
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
