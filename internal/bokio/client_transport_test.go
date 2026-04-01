package bokio

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ecleangg/booky/internal/config"
	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/testutil"
	"github.com/google/uuid"
)

var testCompanyID = uuid.MustParse("453d7692-ff18-4d79-be6a-2519d9f6655d")

func TestValidateAccountsReportsMissingAccounts(t *testing.T) {
	fixture := testutil.LoadFixture(t, "bokio", "chart_of_accounts.json")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method %s", r.Method)
		}
		if r.URL.Path != "/v1/companies/"+testCompanyID.String()+"/chart-of-accounts" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer bokio-token" {
			t.Fatalf("unexpected auth header %q", got)
		}
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	err := client.ValidateAccounts(context.Background(), []int{1580, 3001, 9999})
	if err == nil {
		t.Fatal("expected ValidateAccounts to fail")
	}
	if !strings.Contains(err.Error(), "missing accounts: [9999]") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestCreateJournalEntrySendsExpectedPayload(t *testing.T) {
	fixture := testutil.LoadFixture(t, "bokio", "create_journal_response.json")
	postingDate := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		if r.URL.Path != "/v1/companies/"+testCompanyID.String()+"/journal-entries" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}

		var payload struct {
			Title string               `json:"title"`
			Date  string               `json:"date"`
			Items []domain.JournalItem `json:"items"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Title != "Stripe dagsverifikat 2026-04-02" {
			t.Fatalf("unexpected title %q", payload.Title)
		}
		if payload.Date != "2026-04-02" {
			t.Fatalf("unexpected date %q", payload.Date)
		}
		if len(payload.Items) != 2 {
			t.Fatalf("unexpected item count %d", len(payload.Items))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	response, err := client.CreateJournalEntry(context.Background(), "Stripe dagsverifikat 2026-04-02", postingDate, []domain.JournalItem{
		{Account: 1580, Debit: 10},
		{Account: 3001, Credit: 10},
	})
	if err != nil {
		t.Fatalf("CreateJournalEntry returned error: %v", err)
	}
	if response.ID != uuid.MustParse("11111111-1111-1111-1111-111111111111") {
		t.Fatalf("unexpected journal id %s", response.ID)
	}
	if response.JournalEntryNumber != "A-42" {
		t.Fatalf("unexpected journal number %q", response.JournalEntryNumber)
	}
}

func TestUploadJournalAttachmentSendsMultipartPayload(t *testing.T) {
	fixture := testutil.LoadFixture(t, "bokio", "upload_response.json")
	journalEntryID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		if r.URL.Path != "/v1/companies/"+testCompanyID.String()+"/uploads" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart form: %v", err)
		}
		if got := r.FormValue("journalEntryId"); got != journalEntryID.String() {
			t.Fatalf("unexpected journalEntryId %q", got)
		}
		if got := r.FormValue("description"); got != "Attachment description" {
			t.Fatalf("unexpected description %q", got)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("form file: %v", err)
		}
		defer file.Close()
		content, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("read uploaded file: %v", err)
		}
		if header.Filename != "journal.pdf" {
			t.Fatalf("unexpected filename %q", header.Filename)
		}
		if string(content) != "%PDF-sample" {
			t.Fatalf("unexpected upload content %q", string(content))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	uploadID, err := client.UploadJournalAttachment(context.Background(), journalEntryID, "Attachment description", "journal.pdf", []byte("%PDF-sample"))
	if err != nil {
		t.Fatalf("UploadJournalAttachment returned error: %v", err)
	}
	if uploadID != uuid.MustParse("22222222-2222-2222-2222-222222222222") {
		t.Fatalf("unexpected upload id %s", uploadID)
	}
}

func TestCheckAggregatesCompanyReadiness(t *testing.T) {
	companyInfo := testutil.LoadFixture(t, "bokio", "company_information.json")
	chart := testutil.LoadFixture(t, "bokio", "chart_of_accounts.json")
	fiscalYears := testutil.LoadFixture(t, "bokio", "fiscal_years.json")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/companies/" + testCompanyID.String() + "/company-information":
			_, _ = w.Write(companyInfo)
		case "/v1/companies/" + testCompanyID.String() + "/chart-of-accounts":
			_, _ = w.Write(chart)
		case "/v1/companies/" + testCompanyID.String() + "/fiscal-years":
			_, _ = w.Write(fiscalYears)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := client.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if result.CompanyID != testCompanyID {
		t.Fatalf("unexpected company id %s", result.CompanyID)
	}
	if result.CompanyName != "Demo AB" {
		t.Fatalf("unexpected company name %q", result.CompanyName)
	}
	if !result.CompanyInformation || !result.ChartOfAccounts || !result.FiscalYears {
		t.Fatalf("unexpected readiness result %#v", result)
	}
}

func TestEnsureFiscalYearOpenMatchesFixtureRange(t *testing.T) {
	fixture := testutil.LoadFixture(t, "bokio", "fiscal_years_open.json")
	postingDate := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/companies/"+testCompanyID.String()+"/fiscal-years" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.URL.RawQuery; got != "query=status==open" {
			t.Fatalf("unexpected query %q", got)
		}
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	if err := client.EnsureFiscalYearOpen(context.Background(), postingDate); err != nil {
		t.Fatalf("EnsureFiscalYearOpen returned error: %v", err)
	}
}

func newTestClient(serverURL string) *Client {
	return NewClient(config.BokioConfig{
		CompanyID: testCompanyID,
		Token:     "bokio-token",
		BaseURL:   serverURL + "/v1",
	})
}
