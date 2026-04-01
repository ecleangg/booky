package bokio

import (
	"net/http"
	"strings"
	"time"

	"github.com/ecleangg/booky/internal/config"
	"github.com/google/uuid"
)

type Client struct {
	baseURL    string
	companyID  uuid.UUID
	token      string
	httpClient *http.Client
}

type CreateJournalResponse struct {
	ID                 uuid.UUID `json:"id"`
	JournalEntryNumber string    `json:"journalEntryNumber"`
}

type JournalEntry struct {
	ID                 uuid.UUID          `json:"id"`
	Title              string             `json:"title"`
	Date               string             `json:"date"`
	JournalEntryNumber string             `json:"journalEntryNumber"`
	Items              []JournalEntryItem `json:"items"`
}

type JournalEntryItem struct {
	Account int     `json:"account"`
	Debit   float64 `json:"debit"`
	Credit  float64 `json:"credit"`
}

type Upload struct {
	ID             uuid.UUID `json:"id"`
	Description    string    `json:"description"`
	JournalEntryID uuid.UUID `json:"journalEntryId"`
}

type CheckResult struct {
	CompanyID          uuid.UUID `json:"company_id"`
	CompanyName        string    `json:"company_name,omitempty"`
	CompanyInformation bool      `json:"company_information"`
	ChartOfAccounts    bool      `json:"chart_of_accounts"`
	FiscalYears        bool      `json:"fiscal_years"`
}

func NewClient(cfg config.BokioConfig) *Client {
	return &Client{
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		companyID:  cfg.CompanyID,
		token:      cfg.Token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}
