package pdf

import (
	"fmt"
	"strings"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/jung-kurt/gofpdf"
)

func writeWrapped(p *gofpdf.Fpdf, text string) {
	p.MultiCell(0, 4, strings.TrimSpace(text), "", "", false)
	if p.GetY() > 280 {
		p.AddPage()
	}
}

func renderFactsTable(p *gofpdf.Fpdf, facts []domain.AccountingFact) {
	headers := []string{"Group", "Fact", "Event", "Balance Tx", "Source", "Acct", "Dir", "Date", "Source Amt", "SEK", "Market", "VAT", "Review"}
	widths := []float64{32, 24, 28, 28, 26, 12, 10, 17, 23, 16, 13, 15, 33}
	lineHeight := 3.8

	drawFactsHeader(p, headers, widths, lineHeight)
	p.SetFont("Arial", "", 7)
	for _, fact := range facts {
		cells := []string{
			fact.SourceGroupID,
			fact.FactType,
			valueOrEmpty(fact.StripeEventID),
			valueOrEmpty(fact.StripeBalanceTransactionID),
			sourceObjectLabel(fact),
			fmt.Sprintf("%d", fact.BokioAccount),
			fact.Direction,
			fact.PostingDate.Format("2006-01-02"),
			sourceAmountLabel(fact),
			formatSEK(fact.AmountSEKOre),
			valueOrEmpty(fact.MarketCode),
			valueOrEmpty(fact.VATTreatment),
			valueOrEmpty(fact.ReviewReason),
		}
		renderWrappedRow(p, headers, widths, cells, lineHeight)
	}
}

func drawFactsHeader(p *gofpdf.Fpdf, headers []string, widths []float64, rowHeight float64) {
	p.SetFont("Arial", "B", 8)
	p.SetFillColor(230, 230, 230)
	for i, header := range headers {
		p.CellFormat(widths[i], rowHeight+1, header, "1", 0, "L", true, 0, "")
	}
	p.Ln(-1)
}

func ensureTableRowPage(p *gofpdf.Fpdf, rowHeight float64, headers []string, widths []float64, lineHeight float64) {
	if p.GetY()+rowHeight > 190 {
		p.AddPage()
		drawFactsHeader(p, headers, widths, lineHeight)
		p.SetFont("Arial", "", 7)
	}
}

func renderWrappedRow(p *gofpdf.Fpdf, headers []string, widths []float64, cells []string, lineHeight float64) {
	maxLines := 1
	wrapped := make([][]string, len(cells))
	for i, cell := range cells {
		lines := splitCellLines(p, widths[i], cell)
		wrapped[i] = lines
		if len(lines) > maxLines {
			maxLines = len(lines)
		}
	}

	rowHeight := float64(maxLines) * lineHeight
	ensureTableRowPage(p, rowHeight, headers, widths, lineHeight)

	x := p.GetX()
	y := p.GetY()
	for i, lines := range wrapped {
		cellText := strings.Join(lines, "\n")
		p.Rect(x, y, widths[i], rowHeight, "D")
		p.SetXY(x, y)
		p.MultiCell(widths[i], lineHeight, cellText, "", "L", false)
		x += widths[i]
		p.SetXY(x, y)
	}
	p.SetX(10)
	p.SetY(y + rowHeight)
}

func splitCellLines(p *gofpdf.Fpdf, width float64, text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return []string{""}
	}
	lines := p.SplitLines([]byte(text), width)
	if len(lines) == 0 {
		return []string{text}
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, string(line))
	}
	return out
}
