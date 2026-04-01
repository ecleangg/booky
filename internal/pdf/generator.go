package pdf

import (
	"bytes"
	"fmt"
	"sort"
	"time"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/jung-kurt/gofpdf"
)

type Generator struct{}

func NewGenerator() *Generator {
	return &Generator{}
}

func (g *Generator) Render(draft domain.JournalDraft) ([]byte, error) {
	p := gofpdf.New("L", "mm", "A4", "")
	p.SetMargins(10, 10, 10)
	p.SetAutoPageBreak(true, 10)
	p.AddPage()
	p.SetFont("Arial", "B", 14)
	p.Cell(0, 8, draft.Title)
	p.Ln(10)

	p.SetFont("Arial", "", 10)
	writeLine(p, fmt.Sprintf("Bokio company: %s", draft.PostingRun.BokioCompanyID))
	writeLine(p, fmt.Sprintf("Posting date: %s", draft.Date.Format("2006-01-02")))
	generatedAt := draft.GeneratedAt
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	writeLine(p, fmt.Sprintf("Generated at: %s", generatedAt.UTC().Format(time.RFC3339)))
	writeLine(p, fmt.Sprintf("Run ID: %s", draft.PostingRun.ID))
	if checksum := summaryString(draft.Summary, "journal_checksum"); checksum != "" {
		writeLine(p, fmt.Sprintf("Journal checksum: %s", checksum))
	}
	p.Ln(4)

	p.SetFont("Arial", "B", 11)
	writeLine(p, "Journal summary")
	p.SetFont("Arial", "", 9)
	for _, item := range draft.Items {
		writeLine(p, fmt.Sprintf("Account %d  Debit %.2f  Credit %.2f", item.Account, item.Debit, item.Credit))
	}
	p.Ln(4)

	p.SetFont("Arial", "B", 11)
	writeLine(p, "Summary")
	p.SetFont("Arial", "", 9)
	keys := make([]string, 0, len(draft.Summary))
	for key := range draft.Summary {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		writeLine(p, fmt.Sprintf("%s: %s", key, formatSummaryValue(key, draft.Summary[key])))
	}
	p.Ln(4)

	p.SetFont("Arial", "B", 11)
	writeLine(p, "Included facts")
	renderFactsTable(p, draft.Facts)

	var buf bytes.Buffer
	if err := p.Output(&buf); err != nil {
		return nil, fmt.Errorf("render pdf output: %w", err)
	}
	return buf.Bytes(), nil
}

func writeLine(p *gofpdf.Fpdf, text string) {
	p.CellFormat(0, 5, text, "", 1, "", false, 0, "")
}
