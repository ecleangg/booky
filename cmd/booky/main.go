package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ecleangg/booky/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	configPath := os.Getenv("BOOKY_CONFIG")
	if configPath == "" {
		configPath = "config/booky.yaml"
	}

	application, err := app.New(ctx, configPath)
	if err != nil {
		log.Fatalf("initialize app: %v", err)
	}

	if err := application.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("run app: %v", err)
	}
}
