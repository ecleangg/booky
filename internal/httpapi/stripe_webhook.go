package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/ecleangg/booky/internal/stripe"
)

func stripeWebhookHandler(logger *slog.Logger, service *stripe.Service) http.HandlerFunc {
	return stripeWebhookWithRunner(logger, service, func(ctx context.Context, payload []byte, signature string) error {
		return service.HandleWebhook(ctx, payload, signature)
	})
}

func stripeConnectWebhookHandler(logger *slog.Logger, service *stripe.Service) http.HandlerFunc {
	return stripeWebhookWithRunner(logger, service, func(ctx context.Context, payload []byte, signature string) error {
		return service.HandleConnectWebhook(ctx, payload, signature)
	})
}

func stripeWebhookWithRunner(logger *slog.Logger, service *stripe.Service, run func(context.Context, []byte, string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, errors.New("stripe service is not configured"))
			return
		}

		payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4<<20))
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}

		signature := r.Header.Get("Stripe-Signature")
		if err := run(r.Context(), payload, signature); err != nil {
			logger.Error("stripe webhook failed", "error", err)
			status := http.StatusInternalServerError
			if errors.Is(err, stripe.ErrWebhookSignatureInvalid) ||
				errors.Is(err, stripe.ErrWebhookSignatureMismatch) ||
				errors.Is(err, stripe.ErrWebhookSignatureExpired) ||
				errors.Is(err, stripe.ErrWebhookSignatureInFuture) {
				status = http.StatusUnauthorized
			} else if errors.Is(err, stripe.ErrInvalidEvent) || errors.Is(err, io.ErrUnexpectedEOF) || isClientError(err) {
				status = http.StatusBadRequest
			}
			writeError(w, status, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"status": "accepted"})
	}
}
