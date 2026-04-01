package app

import (
	"context"
	"errors"
	"net/http"
	"time"
)

func (a *App) Run(ctx context.Context) error {
	if a.Filings != nil && a.Filings.Enabled() {
		now := time.Now()
		if loc, err := a.Config.Location(); err == nil {
			now = now.In(loc)
		}
		if err := a.Filings.BackfillUpcoming(ctx, now); err != nil {
			a.Logger.Error("filings startup backfill failed", "error", err)
		}
	}

	errCh := make(chan error, 2)
	go func() {
		a.Logger.Info("http server listening", "addr", a.HTTPServer.Addr)
		if err := a.HTTPServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	go func() {
		if err := a.Scheduler.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		return a.shutdown(ctx.Err())
	case err := <-errCh:
		return a.shutdown(err)
	}
}

func (a *App) shutdown(result error) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = a.HTTPServer.Shutdown(shutdownCtx)
	a.Repo.Close()
	return result
}
