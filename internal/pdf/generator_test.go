package pdf

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/google/uuid"
	"github.com/jung-kurt/gofpdf"
)

func TestGeneratorRenderProducesPDF(t *testing.T) {
	generator := NewGenerator()
	draft := sampleDraft(12)

	data, err := generator.Render(draft)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Fatalf("expected PDF header, got %q", string(data[:4]))
	}
}

func TestGeneratorRenderHandlesLargeFactTable(t *testing.T) {
	generator := NewGenerator()
	draft := sampleDraft(200)

	data, err := generator.Render(draft)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if len(data) < 4000 {
		t.Fatalf("expected larger PDF for long table, got %d bytes", len(data))
	}
}

func TestFormatHelpers(t *testing.T) {
	if got := formatSummaryValue("amount_ore", int64(1234)); got != "12.34 SEK" {
		t.Fatalf("unexpected ore summary %q", got)
	}

	fact := domain.AccountingFact{
		SourceObjectType:  "charge",
		SourceObjectID:    "ch_123",
		SourceCurrency:    strPtr("EUR"),
		SourceAmountMinor: intPtr(1200),
	}
	if got := sourceObjectLabel(fact); got != "charge:ch_123" {
		t.Fatalf("unexpected source object label %q", got)
	}
	if got := sourceAmountLabel(fact); got != "1200 EUR" {
		t.Fatalf("unexpected source amount label %q", got)
	}
	if got := splitCellLines(newTestPDF(), 20, strings.Repeat("abc ", 20)); len(got) < 2 {
		t.Fatalf("expected wrapped lines, got %#v", got)
	}
}

func sampleDraft(factCount int) domain.JournalDraft {
	facts := make([]domain.AccountingFact, 0, factCount)
	for i := 0; i < factCount; i++ {
		amount := int64(1000 + i)
		facts = append(facts, domain.AccountingFact{
			ID:                         uuid.New(),
			SourceGroupID:              "charge:ch_123:sale",
			SourceObjectType:           "charge",
			SourceObjectID:             "ch_123",
			StripeEventID:              strPtr("evt_123"),
			StripeBalanceTransactionID: strPtr("bt_123"),
			FactType:                   "sale_revenue",
			PostingDate:                time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC),
			SourceCurrency:             strPtr("SEK"),
			SourceAmountMinor:          &amount,
			AmountSEKOre:               amount,
			BokioAccount:               3001,
			Direction:                  domain.DirectionCredit,
			ReviewReason:               strPtr("review only when needed"),
		})
	}

	return domain.JournalDraft{
		Title:       "Journal draft",
		Date:        time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC),
		GeneratedAt: time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC),
		Items: []domain.JournalItem{
			{Account: 1580, Debit: 10},
			{Account: 3001, Credit: 10},
		},
		Facts: facts,
		Summary: map[string]any{
			"fact_count":          factCount,
			"journal_checksum":    "checksum",
			"rounding_amount_ore": int64(0),
		},
		PostingRun: domain.PostingRun{
			ID:             uuid.New(),
			BokioCompanyID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		},
	}
}

func newTestPDF() *gofpdf.Fpdf {
	p := gofpdf.New("L", "mm", "A4", "")
	p.AddPage()
	p.SetFont("Arial", "", 7)
	return p
}

func strPtr(value string) *string {
	return &value
}

func intPtr(value int64) *int64 {
	return &value
}
