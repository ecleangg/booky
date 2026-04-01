package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/ecleangg/booky/internal/bokio"
)

func bokioCheckHandler(logger *slog.Logger, client *bokio.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}

		result, err := client.Check(r.Context())
		if err != nil {
			logger.Error("bokio check failed", "error", err)
			writeError(w, http.StatusBadGateway, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"bokio":  result,
		})
	}
}
