package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tucows/interview-exercise-ai/services/api/internal/config"
	"github.com/tucows/interview-exercise-ai/services/api/internal/documents"
	"github.com/tucows/interview-exercise-ai/services/api/internal/httpapi"
	"github.com/tucows/interview-exercise-ai/services/api/internal/logging"
	"github.com/tucows/interview-exercise-ai/services/api/internal/ragclient"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logging.New(cfg.LogLevel, cfg.Environment)

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL())
	if err != nil {
		return fmt.Errorf("postgres pool: %w", err)
	}
	defer pool.Close()

	pingCtx, cancelPing := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelPing()
	if err := pool.Ping(pingCtx); err != nil {
		log.Warn("database not reachable at startup", "err", err)
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr(),
		Handler:           httpapi.New(log, documents.NewPostgresRepository(pool), ragclient.New(cfg.RAGServiceURL, cfg.RAGClientTimeout), cfg.CORSOrigins()),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		// Query waits on the LLM inside the RAG service.
		WriteTimeout: cfg.RAGClientTimeout + 15*time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.HTTPAddr())
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
