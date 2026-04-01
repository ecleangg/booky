package filings

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ecleangg/booky/internal/config"
	"github.com/ecleangg/booky/internal/domain"
)

type HTTPRateClient struct {
	cfg        config.FilingFXConfig
	httpClient *http.Client

	mu           sync.Mutex
	ecbRates     map[string]float64
	monthlyCache map[string]domain.FXRate
}

func NewHTTPRateClient(cfg config.FilingFXConfig) *HTTPRateClient {
	return &HTTPRateClient{
		cfg:          cfg,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		ecbRates:     map[string]float64{},
		monthlyCache: map[string]domain.FXRate{},
	}
}

func (c *HTTPRateClient) OSSPeriodEndEURSEK(ctx context.Context, period string) (domain.FXRate, error) {
	if err := c.loadECB(ctx); err != nil {
		return domain.FXRate{}, err
	}
	end, err := quarterPeriodEnd(period, time.UTC)
	if err != nil {
		return domain.FXRate{}, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	for offset := 0; offset <= 7; offset++ {
		observed := end.AddDate(0, 0, offset)
		key := observed.Format("2006-01-02")
		if rate, ok := c.ecbRates[key]; ok {
			return domain.FXRate{
				Provider:      c.cfg.OSSProvider,
				BaseCurrency:  "EUR",
				QuoteCurrency: "SEK",
				Period:        period,
				Rate:          rate,
				ObservedAt:    observed,
			}, nil
		}
	}
	return domain.FXRate{}, fmt.Errorf("no ECB SEK rate found for %s within 7 days of period end", period)
}

func (c *HTTPRateClient) PSMonthlyAverage(ctx context.Context, currency, period string) (domain.FXRate, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "SEK" {
		return domain.FXRate{
			Provider:      c.cfg.PSProvider,
			BaseCurrency:  "SEK",
			QuoteCurrency: "SEK",
			Period:        period,
			Rate:          1,
		}, nil
	}

	cacheKey := currency + ":" + period
	c.mu.Lock()
	if rate, ok := c.monthlyCache[cacheKey]; ok {
		c.mu.Unlock()
		return rate, nil
	}
	c.mu.Unlock()

	start, end, err := monthlyBounds(period, time.UTC)
	if err != nil {
		return domain.FXRate{}, err
	}
	var response struct {
		Value []struct {
			Date  string  `json:"date"`
			Value float64 `json:"value"`
		} `json:"value"`
	}
	url := fmt.Sprintf("%s/swea/v1/Observations/sek%spmi/%s/%s",
		strings.TrimRight(c.cfg.RiksbankBaseURL, "/"),
		strings.ToLower(currency),
		start.Format("2006-01-02"),
		end.Format("2006-01-02"),
	)
	if err := c.getJSON(ctx, url, &response); err != nil {
		return domain.FXRate{}, err
	}
	if len(response.Value) == 0 {
		return domain.FXRate{}, fmt.Errorf("no Riksbank observations found for %s %s", currency, period)
	}

	var total float64
	var lastDate time.Time
	for _, observation := range response.Value {
		total += observation.Value
		if parsed, err := time.Parse("2006-01-02", observation.Date); err == nil && parsed.After(lastDate) {
			lastDate = parsed
		}
	}
	rate := domain.FXRate{
		Provider:      c.cfg.PSProvider,
		BaseCurrency:  currency,
		QuoteCurrency: "SEK",
		Period:        period,
		Rate:          total / float64(len(response.Value)),
		ObservedAt:    lastDate,
	}

	c.mu.Lock()
	c.monthlyCache[cacheKey] = rate
	c.mu.Unlock()
	return rate, nil
}

func (c *HTTPRateClient) loadECB(ctx context.Context) error {
	c.mu.Lock()
	loaded := len(c.ecbRates) > 0
	c.mu.Unlock()
	if loaded {
		return nil
	}

	url := strings.TrimRight(c.cfg.ECBBaseURL, "/") + "/stats/eurofxref/eurofxref-hist.xml"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create ECB request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("load ECB rates: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ECB rates returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var envelope struct {
		Cube struct {
			Days []struct {
				Time  string `xml:"time,attr"`
				Rates []struct {
					Currency string `xml:"currency,attr"`
					Rate     string `xml:"rate,attr"`
				} `xml:"Cube"`
			} `xml:"Cube"`
		} `xml:"Cube"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("decode ECB XML: %w", err)
	}

	rates := map[string]float64{}
	for _, day := range envelope.Cube.Days {
		for _, rate := range day.Rates {
			if rate.Currency != "SEK" {
				continue
			}
			var parsed float64
			if _, err := fmt.Sscanf(rate.Rate, "%f", &parsed); err != nil {
				return fmt.Errorf("parse ECB SEK rate %q: %w", rate.Rate, err)
			}
			rates[day.Time] = parsed
		}
	}

	c.mu.Lock()
	c.ecbRates = rates
	c.mu.Unlock()
	return nil
}

func (c *HTTPRateClient) getJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create HTTP request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call rate provider: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read rate response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("rate provider returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode rate response: %w", err)
	}
	return nil
}
