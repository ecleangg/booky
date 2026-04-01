package httpapi

import (
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/ecleangg/booky/internal/stripe"
)

func stripeWebhookHandler(logger *slog.Logger, service *stripe.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}

		payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4<<20))
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}

		signature := r.Header.Get("Stripe-Signature")
		if err := service.HandleWebhook(r.Context(), payload, signature); err != nil {
			logger.Error("stripe webhook failed", "error", err)
			status := http.StatusInternalServerError
			if errors.Is(err, io.ErrUnexpectedEOF) || isClientError(err) {
				status = http.StatusBadRequest
			}
			if status == http.StatusInternalServerError && (signature == "" || err.Error() == "stripe webhook signature mismatch") {
				status = http.StatusUnauthorized
			}
			writeError(w, status, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"status": "accepted"})
	}
}
