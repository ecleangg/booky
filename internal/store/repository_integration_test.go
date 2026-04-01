package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/store"
	"github.com/ecleangg/booky/internal/testutil"
	"github.com/google/uuid"
)

func TestQueriesWebhookLifecycleAndSnapshots(t *testing.T) {
	repo, _ := testutil.NewTestRepository(t)
	q := repo.Queries()
	ctx := context.Background()

	event := domain.StripeWebhookEvent{
		ID:              "evt_123",
		EventType:       "charge.succeeded",
		Livemode:        false,
		APIVersion:      "2025-01-27.acacia",
		StripeCreatedAt: time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC),
		Payload:         json.RawMessage(`{"id":"evt_123","type":"charge.succeeded"}`),
	}

	inserted, err := q.InsertWebhookEvent(ctx, event)
	if err != nil {
		t.Fatalf("InsertWebhookEvent returned error: %v", err)
	}
	if !inserted {
		t.Fatal("expected first insert to succeed")
	}

	inserted, err = q.InsertWebhookEvent(ctx, event)
	if err != nil {
		t.Fatalf("duplicate InsertWebhookEvent returned error: %v", err)
	}
	if inserted {
		t.Fatal("expected duplicate insert to be ignored")
	}

	if err := q.MarkWebhookFailed(ctx, event.ID, "boom"); err != nil {
		t.Fatalf("MarkWebhookFailed returned error: %v", err)
	}
	stored, err := q.GetWebhookEvent(ctx, event.ID)
	if err != nil {
		t.Fatalf("GetWebhookEvent returned error: %v", err)
	}
	if stored.ProcessingError == nil || *stored.ProcessingError != "boom" {
		t.Fatalf("unexpected processing error %#v", stored.ProcessingError)
	}

	event.APIVersion = "2026-01-01"
	event.Payload = json.RawMessage(`{"id":"evt_123","retry":true}`)
	if err := q.ResetWebhookForRetry(ctx, event); err != nil {
		t.Fatalf("ResetWebhookForRetry returned error: %v", err)
	}
	stored, err = q.GetWebhookEvent(ctx, event.ID)
	if err != nil {
		t.Fatalf("GetWebhookEvent after reset returned error: %v", err)
	}
	if stored.APIVersion != "2026-01-01" {
		t.Fatalf("unexpected api version %q", stored.APIVersion)
	}
	if stored.ProcessedAt != nil || stored.ProcessingError != nil {
		t.Fatalf("expected reset webhook state, got %#v", stored)
	}

	if err := q.MarkWebhookProcessed(ctx, event.ID); err != nil {
		t.Fatalf("MarkWebhookProcessed returned error: %v", err)
	}
	stored, err = q.GetWebhookEvent(ctx, event.ID)
	if err != nil {
		t.Fatalf("GetWebhookEvent after process returned error: %v", err)
	}
	if stored.ProcessedAt == nil {
		t.Fatal("expected processed_at to be set")
	}

	snapshot := domain.ObjectSnapshot{
		ObjectType: "charge",
		ObjectID:   "ch_123",
		Livemode:   false,
		Payload:    json.RawMessage(`{"id":"ch_123","version":1}`),
	}
	if err := q.UpsertObjectSnapshot(ctx, snapshot); err != nil {
		t.Fatalf("UpsertObjectSnapshot returned error: %v", err)
	}
	snapshot.Payload = json.RawMessage(`{"id":"ch_123","version":2}`)
	if err := q.UpsertObjectSnapshot(ctx, snapshot); err != nil {
		t.Fatalf("UpsertObjectSnapshot second write returned error: %v", err)
	}
	storedSnapshot, err := q.GetObjectSnapshot(ctx, "charge", "ch_123")
	if err != nil {
		t.Fatalf("GetObjectSnapshot returned error: %v", err)
	}
	if !strings.Contains(string(storedSnapshot.Payload), `"version": 2`) && !strings.Contains(string(storedSnapshot.Payload), `"version":2`) {
		t.Fatalf("unexpected snapshot payload %s", string(storedSnapshot.Payload))
	}

	_, err = q.GetObjectSnapshot(ctx, "charge", "missing")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestQueriesAccountingPostingLifecycle(t *testing.T) {
	repo, _ := testutil.NewTestRepository(t)
	q := repo.Queries()
	ctx := context.Background()
	companyID := testutil.TestConfig().Bokio.CompanyID
	postingDate := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)

	facts := []domain.AccountingFact{
		sampleFact(companyID, uuid.New(), "charge:ch_123:sale", "sale_receivable", 1580, domain.DirectionDebit, 11900, postingDate),
		sampleFact(companyID, uuid.New(), "charge:ch_123:sale", "sale_revenue", 3001, domain.DirectionCredit, 11900, postingDate),
	}
	if err := q.UpsertAccountingFacts(ctx, facts); err != nil {
		t.Fatalf("UpsertAccountingFacts returned error: %v", err)
	}

	pending, err := q.ListPendingAccountingFacts(ctx, companyID, postingDate)
	if err != nil {
		t.Fatalf("ListPendingAccountingFacts returned error: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending facts, got %d", len(pending))
	}

	run := domain.PostingRun{
		ID:             uuid.New(),
		BokioCompanyID: companyID,
		PostingDate:    postingDate,
		Timezone:       "Europe/Stockholm",
		RunType:        domain.PostingRunTypeDailyClose,
		SequenceNo:     1,
		Status:         domain.PostingRunStatusStarted,
		ConfigSnapshot: json.RawMessage(`{"env":"test"}`),
		Summary:        json.RawMessage(`{"status":"started"}`),
	}
	if err := q.CreatePostingRun(ctx, run); err != nil {
		t.Fatalf("CreatePostingRun returned error: %v", err)
	}
	storedRun, err := q.GetPostingRunByDate(ctx, companyID, postingDate, domain.PostingRunTypeDailyClose)
	if err != nil {
		t.Fatalf("GetPostingRunByDate returned error: %v", err)
	}
	if storedRun.Status != domain.PostingRunStatusStarted {
		t.Fatalf("unexpected run status %q", storedRun.Status)
	}

	factIDs := []uuid.UUID{facts[0].ID, facts[1].ID}
	if err := q.AttachFactsToRun(ctx, run.ID, factIDs); err != nil {
		t.Fatalf("AttachFactsToRun returned error: %v", err)
	}
	if err := q.MarkFactsStatus(ctx, factIDs, domain.FactStatusBatched); err != nil {
		t.Fatalf("MarkFactsStatus returned error: %v", err)
	}
	runFacts, err := q.ListFactsByRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("ListFactsByRun returned error: %v", err)
	}
	if len(runFacts) != 2 {
		t.Fatalf("expected 2 run facts, got %d", len(runFacts))
	}
	for _, fact := range runFacts {
		if fact.Status != domain.FactStatusBatched {
			t.Fatalf("unexpected fact status %q", fact.Status)
		}
	}

	if err := q.UpdatePostingRun(ctx, run.ID, domain.PostingRunStatusJournalCreated, json.RawMessage(`{"status":"journal_created"}`), nil); err != nil {
		t.Fatalf("UpdatePostingRun returned error: %v", err)
	}

	uploadID := uuid.New()
	journal := domain.BokioJournal{
		PostingRunID:        run.ID,
		BokioCompanyID:      companyID,
		BokioJournalEntryID: uuid.New(),
		BokioJournalEntryNo: "A-42",
		BokioUploadID:       &uploadID,
		BokioJournalTitle:   "Stripe dagsverifikat 2026-04-02",
		PostingDate:         postingDate,
		AttachmentChecksum:  "checksum",
	}
	if err := q.UpsertBokioJournal(ctx, journal); err != nil {
		t.Fatalf("UpsertBokioJournal returned error: %v", err)
	}
	storedJournal, err := q.GetBokioJournalByRunID(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetBokioJournalByRunID returned error: %v", err)
	}
	if storedJournal.BokioUploadID == nil || *storedJournal.BokioUploadID != uploadID {
		t.Fatalf("unexpected stored upload id %#v", storedJournal.BokioUploadID)
	}

	if err := q.MarkFactsStatus(ctx, factIDs, domain.FactStatusPosted); err != nil {
		t.Fatalf("MarkFactsStatus posted returned error: %v", err)
	}
	pending, err = q.ListPendingAccountingFacts(ctx, companyID, postingDate)
	if err != nil {
		t.Fatalf("ListPendingAccountingFacts after post returned error: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected no pending facts, got %d", len(pending))
	}

	replacement := []domain.AccountingFact{
		sampleFact(companyID, uuid.New(), "charge:ch_123:sale", "sale_receivable", 1580, domain.DirectionDebit, 11900, postingDate),
	}
	if err := q.UpsertAccountingFacts(ctx, replacement); err == nil {
		t.Fatal("expected upsert to fail for posted source group")
	}

	_, err = q.GetPostingRunByDate(ctx, companyID, postingDate.AddDate(0, 0, 1), domain.PostingRunTypeDailyClose)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestQueriesFilingLifecycle(t *testing.T) {
	repo, _ := testutil.NewTestRepository(t)
	q := repo.Queries()
	ctx := context.Background()
	companyID := testutil.TestConfig().Bokio.CompanyID
	postingDate := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	period := "2026-Q1"
	now := time.Date(2026, 4, 24, 9, 0, 0, 0, time.UTC)

	facts := []domain.AccountingFact{
		sampleFact(companyID, uuid.New(), "charge:ch_456:sale", "sale_revenue", 3001, domain.DirectionCredit, 7900, postingDate),
		sampleFact(companyID, uuid.New(), "payout:po_1", "payout", 1920, domain.DirectionDebit, 7900, postingDate),
	}
	if err := q.UpsertAccountingFacts(ctx, facts); err != nil {
		t.Fatalf("UpsertAccountingFacts returned error: %v", err)
	}
	relevantFacts, err := q.ListFilingRelevantFactsByDateRange(ctx, companyID, postingDate.AddDate(0, 0, -1), postingDate.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("ListFilingRelevantFactsByDateRange returned error: %v", err)
	}
	if len(relevantFacts) != 1 || relevantFacts[0].SourceGroupID != "charge:ch_456:sale" {
		t.Fatalf("unexpected relevant facts %#v", relevantFacts)
	}

	ossEntry := domain.OSSUnionEntry{
		ID:                    uuid.New(),
		BokioCompanyID:        companyID,
		SourceGroupID:         "charge:ch_456:sale",
		SourceObjectType:      "charge",
		SourceObjectID:        "ch_456",
		OriginalSupplyPeriod:  period,
		FilingPeriod:          period,
		ConsumptionCountry:    "DE",
		OriginCountry:         "SE",
		OriginIdentifier:      "SE556000016701",
		SaleType:              "SERVICES",
		VATRateBasisPoints:    1900,
		TaxableAmountEURCents: 10000,
		VATAmountEURCents:     1900,
		ReviewState:           domain.FilingReviewStateReady,
		Payload:               json.RawMessage(`{"entry":"oss"}`),
	}
	if err := q.UpsertOSSUnionEntries(ctx, []domain.OSSUnionEntry{ossEntry}); err != nil {
		t.Fatalf("UpsertOSSUnionEntries returned error: %v", err)
	}
	psEntry := domain.PeriodicSummaryEntry{
		ID:                uuid.New(),
		BokioCompanyID:    companyID,
		SourceGroupID:     "charge:ch_789:sale",
		SourceObjectType:  "charge",
		SourceObjectID:    "ch_789",
		FilingPeriod:      "2026-03",
		BuyerVATNumber:    "DE123456789",
		RowType:           "services",
		AmountSEKOre:      7900,
		ExportedAmountSEK: 79,
		ReviewState:       domain.FilingReviewStateReview,
		ReviewReason:      ptr("missing validation"),
		Payload:           json.RawMessage(`{"entry":"ps"}`),
	}
	if err := q.UpsertPeriodicSummaryEntries(ctx, []domain.PeriodicSummaryEntry{psEntry}); err != nil {
		t.Fatalf("UpsertPeriodicSummaryEntries returned error: %v", err)
	}

	ossEntries, err := q.ListOSSUnionEntriesByPeriod(ctx, companyID, period)
	if err != nil {
		t.Fatalf("ListOSSUnionEntriesByPeriod returned error: %v", err)
	}
	if len(ossEntries) != 1 {
		t.Fatalf("expected 1 OSS entry, got %d", len(ossEntries))
	}
	psEntries, err := q.ListPeriodicSummaryEntriesByPeriod(ctx, companyID, "2026-03")
	if err != nil {
		t.Fatalf("ListPeriodicSummaryEntriesByPeriod returned error: %v", err)
	}
	if len(psEntries) != 1 {
		t.Fatalf("expected 1 PS entry, got %d", len(psEntries))
	}

	filingPeriod := domain.FilingPeriod{
		Kind:                 domain.FilingKindOSSUnion,
		Period:               period,
		BokioCompanyID:       companyID,
		DeadlineDate:         time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC),
		FirstSendAt:          time.Date(2026, 4, 23, 9, 0, 0, 0, time.UTC),
		LastEvaluationStatus: domain.FilingPeriodStatusPending,
	}
	if err := q.UpsertFilingPeriods(ctx, []domain.FilingPeriod{filingPeriod}); err != nil {
		t.Fatalf("UpsertFilingPeriods returned error: %v", err)
	}
	storedPeriod, err := q.GetFilingPeriod(ctx, companyID, domain.FilingKindOSSUnion, period)
	if err != nil {
		t.Fatalf("GetFilingPeriod returned error: %v", err)
	}
	if storedPeriod.LastEvaluationStatus != domain.FilingPeriodStatusPending {
		t.Fatalf("unexpected filing status %q", storedPeriod.LastEvaluationStatus)
	}

	duePeriods, err := q.ListDueFilingPeriods(ctx, companyID, now)
	if err != nil {
		t.Fatalf("ListDueFilingPeriods returned error: %v", err)
	}
	if len(duePeriods) != 1 {
		t.Fatalf("expected 1 due period, got %d", len(duePeriods))
	}

	zeroReminderSentAt := now
	if err := q.UpdateFilingPeriodEvaluation(ctx, companyID, domain.FilingKindOSSUnion, period, domain.FilingPeriodStatusExported, now, &zeroReminderSentAt); err != nil {
		t.Fatalf("UpdateFilingPeriodEvaluation returned error: %v", err)
	}
	storedPeriod, err = q.GetFilingPeriod(ctx, companyID, domain.FilingKindOSSUnion, period)
	if err != nil {
		t.Fatalf("GetFilingPeriod after update returned error: %v", err)
	}
	if storedPeriod.LastEvaluatedAt == nil || storedPeriod.ZeroReminderSentAt == nil {
		t.Fatalf("expected evaluation timestamps, got %#v", storedPeriod)
	}

	filename1 := "oss_union_2026-Q1_v1.txt"
	export1 := domain.FilingExport{
		ID:             uuid.New(),
		Kind:           domain.FilingKindOSSUnion,
		Period:         period,
		BokioCompanyID: companyID,
		Version:        1,
		Checksum:       "sum-1",
		Filename:       &filename1,
		Content:        []byte("version1"),
		Summary:        json.RawMessage(`{"entries":1}`),
	}
	if err := q.CreateFilingExport(ctx, export1); err != nil {
		t.Fatalf("CreateFilingExport returned error: %v", err)
	}
	filename2 := "oss_union_2026-Q1_v2.txt"
	export2 := domain.FilingExport{
		ID:             uuid.New(),
		Kind:           domain.FilingKindOSSUnion,
		Period:         period,
		BokioCompanyID: companyID,
		Version:        2,
		Checksum:       "sum-2",
		Filename:       &filename2,
		Content:        []byte("version2"),
		Summary:        json.RawMessage(`{"entries":1}`),
	}
	if err := q.CreateFilingExport(ctx, export2); err != nil {
		t.Fatalf("CreateFilingExport second version returned error: %v", err)
	}
	if err := q.MarkFilingExportSuperseded(ctx, export1.ID, export2.ID); err != nil {
		t.Fatalf("MarkFilingExportSuperseded returned error: %v", err)
	}

	latestExport, err := q.GetLatestFilingExport(ctx, companyID, domain.FilingKindOSSUnion, period)
	if err != nil {
		t.Fatalf("GetLatestFilingExport returned error: %v", err)
	}
	if latestExport.Version != 2 {
		t.Fatalf("unexpected latest export version %d", latestExport.Version)
	}

	exports, err := q.ListFilingExports(ctx, companyID, domain.FilingKindOSSUnion, period)
	if err != nil {
		t.Fatalf("ListFilingExports returned error: %v", err)
	}
	if len(exports) != 2 {
		t.Fatalf("expected 2 exports, got %d", len(exports))
	}
	if exports[1].SupersededBy == nil || *exports[1].SupersededBy != export2.ID {
		t.Fatalf("expected first export to be superseded, got %#v", exports[1].SupersededBy)
	}

	entryPeriods, err := q.ListEntryPeriods(ctx, companyID, domain.FilingKindOSSUnion)
	if err != nil {
		t.Fatalf("ListEntryPeriods returned error: %v", err)
	}
	if len(entryPeriods) != 1 || entryPeriods[0] != period {
		t.Fatalf("unexpected entry periods %#v", entryPeriods)
	}

	if err := q.MarkFilingPeriodSubmitted(ctx, companyID, domain.FilingKindOSSUnion, period, now); err != nil {
		t.Fatalf("MarkFilingPeriodSubmitted returned error: %v", err)
	}
	duePeriods, err = q.ListDueFilingPeriods(ctx, companyID, now.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("ListDueFilingPeriods after submit returned error: %v", err)
	}
	if len(duePeriods) != 0 {
		t.Fatalf("expected no due periods after submission, got %d", len(duePeriods))
	}

	if err := q.DeleteOSSUnionEntriesBySourceGroups(ctx, []string{ossEntry.SourceGroupID}); err != nil {
		t.Fatalf("DeleteOSSUnionEntriesBySourceGroups returned error: %v", err)
	}
	if err := q.DeletePeriodicSummaryEntriesBySourceGroups(ctx, []string{psEntry.SourceGroupID}); err != nil {
		t.Fatalf("DeletePeriodicSummaryEntriesBySourceGroups returned error: %v", err)
	}

	_, err = q.GetLatestFilingExport(ctx, companyID, domain.FilingKindPeriodicSummary, "missing")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRepositoryAcquireAdvisoryLock(t *testing.T) {
	repo, _ := testutil.NewTestRepository(t)
	ctx := context.Background()

	unlock, err := repo.AcquireAdvisoryLock(ctx, 2026, 92)
	if err != nil {
		t.Fatalf("AcquireAdvisoryLock returned error: %v", err)
	}

	_, err = repo.AcquireAdvisoryLock(ctx, 2026, 92)
	if !errors.Is(err, store.ErrLockBusy) {
		t.Fatalf("expected ErrLockBusy, got %v", err)
	}

	if err := unlock(context.Background()); err != nil {
		t.Fatalf("unlock returned error: %v", err)
	}

	reacquired, err := repo.AcquireAdvisoryLock(ctx, 2026, 92)
	if err != nil {
		t.Fatalf("reacquire returned error: %v", err)
	}
	if err := reacquired(context.Background()); err != nil {
		t.Fatalf("reacquired unlock returned error: %v", err)
	}
}

func sampleFact(companyID uuid.UUID, id uuid.UUID, sourceGroupID, factType string, account int, direction string, amountSEK int64, postingDate time.Time) domain.AccountingFact {
	sourceCurrency := "SEK"
	sourceAmount := amountSEK
	return domain.AccountingFact{
		ID:                id,
		BokioCompanyID:    companyID,
		StripeAccountID:   "acct_123",
		SourceGroupID:     sourceGroupID,
		SourceObjectType:  "charge",
		SourceObjectID:    "obj_123",
		FactType:          factType,
		PostingDate:       postingDate,
		SourceCurrency:    &sourceCurrency,
		SourceAmountMinor: &sourceAmount,
		AmountSEKOre:      amountSEK,
		BokioAccount:      account,
		Direction:         direction,
		Status:            domain.FactStatusPending,
		Payload:           json.RawMessage(`{"fact":"sample"}`),
	}
}

func ptr(value string) *string {
	return &value
}
