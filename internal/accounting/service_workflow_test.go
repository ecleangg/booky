package accounting

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ecleangg/booky/internal/bokio"
	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/pdf"
	"github.com/ecleangg/booky/internal/testutil"
	"github.com/google/uuid"
)

func TestRunDailyCloseReturnsEarlyWithoutFacts(t *testing.T) {
	repo, _ := testutil.NewTestRepository(t)
	cfg := testutil.TestConfig()
	service := NewService(cfg, repo, bokio.NewClient(cfg.Bokio), nil, pdf.NewGenerator(), accountingTestLogger())

	postingDate := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	if err := service.RunDailyClose(context.Background(), postingDate); err != nil {
		t.Fatalf("RunDailyClose returned error: %v", err)
	}

	_, err := repo.Queries().GetPostingRunByDate(context.Background(), cfg.Bokio.CompanyID, postingDate, domain.PostingRunTypeDailyClose)
	if err == nil {
		t.Fatal("expected no posting run to be created")
	}
}

func TestRunDailyCloseCompletesPostingWorkflow(t *testing.T) {
	repo, _ := testutil.NewTestRepository(t)
	chartFixture := testutil.LoadFixture(t, "bokio", "chart_of_accounts.json")
	openFiscalYearFixture := testutil.LoadFixture(t, "bokio", "fiscal_years_open.json")
	journalFixture := testutil.LoadFixture(t, "bokio", "create_journal_response.json")
	uploadFixture := testutil.LoadFixture(t, "bokio", "upload_response.json")

	cfg := testutil.TestConfig()
	postingDate := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/chart-of-accounts"):
			_, _ = w.Write(chartFixture)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/fiscal-years") && r.URL.Query().Get("query") == "status==open":
			_, _ = w.Write(openFiscalYearFixture)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/journal-entries"):
			_, _ = w.Write([]byte(`{"items":[]}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/journal-entries"):
			_, _ = w.Write(journalFixture)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/uploads"):
			_, _ = w.Write([]byte(`{"items":[]}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/uploads"):
			_, _ = w.Write(uploadFixture)
		default:
			t.Fatalf("unexpected bokio request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	cfg.Bokio.BaseURL = server.URL + "/v1"

	service := NewService(cfg, repo, bokio.NewClient(cfg.Bokio), nil, pdf.NewGenerator(), accountingTestLogger())
	if err := repo.Queries().UpsertAccountingFacts(context.Background(), []domain.AccountingFact{
		accountingFact(cfg.Bokio.CompanyID, "charge:ch_123:sale", "sale_receivable", 1580, domain.DirectionDebit, 11900, postingDate),
		accountingFact(cfg.Bokio.CompanyID, "charge:ch_123:sale", "sale_revenue", 3001, domain.DirectionCredit, 11900, postingDate),
	}); err != nil {
		t.Fatalf("UpsertAccountingFacts returned error: %v", err)
	}

	if err := service.RunDailyClose(context.Background(), postingDate); err != nil {
		t.Fatalf("RunDailyClose returned error: %v", err)
	}

	run, err := repo.Queries().GetPostingRunByDate(context.Background(), cfg.Bokio.CompanyID, postingDate, domain.PostingRunTypeDailyClose)
	if err != nil {
		t.Fatalf("GetPostingRunByDate returned error: %v", err)
	}
	if run.Status != domain.PostingRunStatusCompleted {
		t.Fatalf("unexpected posting run status %q", run.Status)
	}

	journal, err := repo.Queries().GetBokioJournalByRunID(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("GetBokioJournalByRunID returned error: %v", err)
	}
	if journal.BokioUploadID == nil {
		t.Fatal("expected upload id to be stored")
	}

	pending, err := repo.Queries().ListPendingAccountingFacts(context.Background(), cfg.Bokio.CompanyID, postingDate)
	if err != nil {
		t.Fatalf("ListPendingAccountingFacts returned error: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected no pending facts, got %d", len(pending))
	}

	runFacts, err := repo.Queries().ListFactsByRun(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("ListFactsByRun returned error: %v", err)
	}
	if len(runFacts) != 2 {
		t.Fatalf("expected 2 facts attached to run, got %d", len(runFacts))
	}
	for _, fact := range runFacts {
		if fact.Status != domain.FactStatusPosted {
			t.Fatalf("unexpected fact status %q", fact.Status)
		}
	}
}

func accountingFact(companyID uuid.UUID, sourceGroupID, factType string, account int, direction string, amountSEK int64, postingDate time.Time) domain.AccountingFact {
	currency := "SEK"
	sourceAmount := amountSEK
	return domain.AccountingFact{
		ID:                uuid.New(),
		BokioCompanyID:    companyID,
		StripeAccountID:   "acct_123",
		SourceGroupID:     sourceGroupID,
		SourceObjectType:  "charge",
		SourceObjectID:    "ch_123",
		FactType:          factType,
		PostingDate:       postingDate,
		SourceCurrency:    &currency,
		SourceAmountMinor: &sourceAmount,
		AmountSEKOre:      amountSEK,
		BokioAccount:      account,
		Direction:         direction,
		Status:            domain.FactStatusPending,
		Payload:           json.RawMessage(`{"fact":"sample"}`),
	}
}

func accountingTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
