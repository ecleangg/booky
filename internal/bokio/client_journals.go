package bokio

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/google/uuid"
)

func (c *Client) ValidateAccounts(ctx context.Context, accounts []int) error {
	if len(accounts) == 0 {
		return nil
	}
	endpoint := fmt.Sprintf("/companies/%s/chart-of-accounts", c.companyID)
	var result []struct {
		Account int `json:"account"`
	}
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &result); err != nil {
		return err
	}
	existing := map[int]struct{}{}
	for _, account := range result {
		existing[account.Account] = struct{}{}
	}
	missing := make([]int, 0)
	for _, account := range accounts {
		if _, ok := existing[account]; !ok {
			missing = append(missing, account)
		}
	}
	if len(missing) > 0 {
		sort.Ints(missing)
		return fmt.Errorf("bokio chart of accounts missing accounts: %v", missing)
	}
	return nil
}

func (c *Client) EnsureFiscalYearOpen(ctx context.Context, postingDate time.Time) error {
	endpoint := fmt.Sprintf("/companies/%s/fiscal-years?query=status==open", c.companyID)
	var response struct {
		Items []struct {
			StartDate string `json:"startDate"`
			EndDate   string `json:"endDate"`
			Status    string `json:"status"`
		} `json:"items"`
	}
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &response); err != nil {
		return err
	}
	for _, year := range response.Items {
		start, err := time.Parse("2006-01-02", year.StartDate)
		if err != nil {
			continue
		}
		end, err := time.Parse("2006-01-02", year.EndDate)
		if err != nil {
			continue
		}
		if !postingDate.Before(start) && !postingDate.After(end) && year.Status == "open" {
			return nil
		}
	}
	return fmt.Errorf("no open Bokio fiscal year covering %s", postingDate.Format("2006-01-02"))
}

func (c *Client) CreateJournalEntry(ctx context.Context, title string, postingDate time.Time, items []domain.JournalItem) (CreateJournalResponse, error) {
	body := map[string]any{
		"title": title,
		"date":  postingDate.Format("2006-01-02"),
		"items": items,
	}
	endpoint := fmt.Sprintf("/companies/%s/journal-entries", c.companyID)
	var response CreateJournalResponse
	if err := c.doJSON(ctx, http.MethodPost, endpoint, body, &response); err != nil {
		return CreateJournalResponse{}, err
	}
	if response.ID == uuid.Nil {
		return CreateJournalResponse{}, fmt.Errorf("bokio journal response missing id")
	}
	return response, nil
}

func (c *Client) FindMatchingJournalEntry(ctx context.Context, title string, postingDate time.Time, items []domain.JournalItem) (*CreateJournalResponse, error) {
	entries, err := c.listJournalEntries(ctx, fmt.Sprintf("date==%s", postingDate.Format("2006-01-02")), 100)
	if err != nil {
		return nil, err
	}
	expected := normalizeJournalItems(items)
	for _, entry := range entries {
		if entry.Title != title || entry.Date != postingDate.Format("2006-01-02") {
			continue
		}
		if sameJournalItems(expected, normalizeJournalEntryItems(entry.Items)) {
			return &CreateJournalResponse{ID: entry.ID, JournalEntryNumber: entry.JournalEntryNumber}, nil
		}
	}
	return nil, nil
}

func (c *Client) listJournalEntries(ctx context.Context, query string, pageSize int) ([]JournalEntry, error) {
	endpoint := fmt.Sprintf("/companies/%s/journal-entries?page=1&pageSize=%d", c.companyID, pageSize)
	if strings.TrimSpace(query) != "" {
		endpoint += "&query=" + url.QueryEscape(query)
	}
	var response struct {
		Items []JournalEntry `json:"items"`
	}
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &response); err != nil {
		return nil, err
	}
	return response.Items, nil
}
