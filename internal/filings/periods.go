package filings

import (
	"fmt"
	"time"

	"github.com/ecleangg/booky/internal/config"
	"github.com/ecleangg/booky/internal/domain"
)

func filingDeadline(kind, period string, loc *time.Location) (time.Time, error) {
	switch kind {
	case domain.FilingKindOSSUnion:
		start, err := quarterStart(period, loc)
		if err != nil {
			return time.Time{}, err
		}
		nextQuarter := start.AddDate(0, 3, 0)
		return time.Date(nextQuarter.Year(), nextQuarter.Month(), 30, 0, 0, 0, 0, loc), nil
	case domain.FilingKindPeriodicSummary:
		start, err := monthPeriodStart(period, loc)
		if err != nil {
			return time.Time{}, err
		}
		nextMonth := start.AddDate(0, 1, 0)
		return time.Date(nextMonth.Year(), nextMonth.Month(), 25, 0, 0, 0, 0, loc), nil
	default:
		return time.Time{}, fmt.Errorf("unsupported filing kind %q", kind)
	}
}

func filingFirstSendAt(kind, period string, cfg config.Config) (time.Time, error) {
	loc := locationOrUTC(cfg)
	deadline, err := filingDeadline(kind, period, loc)
	if err != nil {
		return time.Time{}, err
	}
	hour, minute, err := cfg.FilingsSendHourMinute()
	if err != nil {
		return time.Time{}, err
	}
	sendDate := deadline.AddDate(0, 0, -cfg.Filings.LeadTimeDays)
	return time.Date(sendDate.Year(), sendDate.Month(), sendDate.Day(), hour, minute, 0, 0, loc), nil
}

func monthPeriod(t time.Time) string {
	return t.Format("2006-01")
}

func quarterPeriod(t time.Time) string {
	quarter := ((int(t.Month()) - 1) / 3) + 1
	return fmt.Sprintf("%04d-Q%d", t.Year(), quarter)
}

func quarterStart(period string, loc *time.Location) (time.Time, error) {
	var year, quarter int
	if _, err := fmt.Sscanf(period, "%4d-Q%d", &year, &quarter); err != nil || quarter < 1 || quarter > 4 {
		return time.Time{}, fmt.Errorf("invalid quarter period %q", period)
	}
	month := time.Month(((quarter - 1) * 3) + 1)
	return time.Date(year, month, 1, 0, 0, 0, 0, loc), nil
}

func quarterPeriodEnd(period string, loc *time.Location) (time.Time, error) {
	start, err := quarterStart(period, loc)
	if err != nil {
		return time.Time{}, err
	}
	return start.AddDate(0, 3, -1), nil
}

func monthPeriodStart(period string, loc *time.Location) (time.Time, error) {
	start, err := time.ParseInLocation("2006-01", period, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid month period %q: %w", period, err)
	}
	return start, nil
}

func monthlyBounds(period string, loc *time.Location) (time.Time, time.Time, error) {
	start, err := monthPeriodStart(period, loc)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return start, start.AddDate(0, 1, -1), nil
}

func periodDateRange(kind, period string, cfg config.Config) (time.Time, time.Time, error) {
	loc := locationOrUTC(cfg)
	switch kind {
	case domain.FilingKindOSSUnion:
		start, err := quarterStart(period, loc)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		return start, start.AddDate(0, 3, -1), nil
	case domain.FilingKindPeriodicSummary:
		return monthlyBounds(period, loc)
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("unsupported filing kind %q", kind)
	}
}

func completedQuarterPeriods(now time.Time, count int) []string {
	periods := make([]string, 0, count)
	currentQuarterStart := startOfQuarter(now)
	cursor := currentQuarterStart.AddDate(0, -3, 0)
	for len(periods) < count {
		periods = append(periods, quarterPeriod(cursor))
		cursor = cursor.AddDate(0, -3, 0)
	}
	return periods
}

func startOfQuarter(t time.Time) time.Time {
	month := time.Month(((int(t.Month()) - 1) / 3 * 3) + 1)
	return time.Date(t.Year(), month, 1, 0, 0, 0, 0, t.Location())
}
