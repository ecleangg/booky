package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/ecleangg/booky/internal/accounting"
	"github.com/ecleangg/booky/internal/config"
)

func dailyCloseHandler(cfg config.Config, logger *slog.Logger, service *accounting.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}

		loc, err := cfg.Location()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}

		dateParam := r.URL.Query().Get("date")
		postingDate := time.Now().In(loc)
		if dateParam != "" {
			parsed, err := time.ParseInLocation("2006-01-02", dateParam, loc)
			if err != nil {
				writeError(w, http.StatusBadRequest, fmt.Errorf("invalid date query param: %w", err))
				return
			}
			postingDate = parsed
		}

		if err := service.RunDailyClose(r.Context(), postingDate); err != nil {
			logger.Error("daily close failed", "posting_date", postingDate.Format("2006-01-02"), "error", err)
			writeError(w, http.StatusInternalServerError, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"status":       "completed",
			"posting_date": postingDate.Format("2006-01-02"),
		})
	}
}
