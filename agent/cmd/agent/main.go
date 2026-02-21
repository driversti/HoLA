package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/driversti/hola/internal/api"
	"github.com/driversti/hola/internal/auth"
	"github.com/driversti/hola/internal/docker"
	"github.com/driversti/hola/internal/registry"
	"github.com/driversti/hola/internal/update"
	"github.com/driversti/hola/internal/ws"
)

const (
	version = "0.2.0"
	repo    = "driversti/HoLA"
)

func main() {
	token := flag.String("token", "", "Bearer token for API authentication")
	flag.Parse()

	if *token == "" {
		*token = os.Getenv("HOLA_TOKEN")
	}
	if *token == "" {
		slog.Error("no auth token provided: set HOLA_TOKEN env var or use --token flag")
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	dockerClient, err := docker.NewClient()
	if err != nil {
		slog.Error("failed to connect to Docker", "error", err)
		os.Exit(1)
	}
	defer dockerClient.Close()

	registryStore, err := registry.NewStore("")
	if err != nil {
		slog.Error("failed to init registry store", "error", err)
		os.Exit(1)
	}

	// WebSocket event hub — listens for Docker container events.
	eventHub := ws.NewEventHub(dockerClient)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go eventHub.Run(ctx)

	wsHandler := ws.NewHandler(eventHub)
	authMiddleware := auth.NewMiddleware(*token)
	updater := update.New(version, repo)
	router := api.NewRouter(version, authMiddleware, dockerClient, wsHandler, registryStore, updater)

	srv := &http.Server{
		Addr:    ":8420",
		Handler: router,
		// ReadHeaderTimeout (not ReadTimeout) protects HTTP header parsing
		// without killing long-lived WebSocket connections.
		ReadHeaderTimeout: 10 * time.Second,
		// WriteTimeout must be 0 for WebSocket connections to stay alive.
		WriteTimeout: 0,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("starting HoLA agent", "port", 8420, "version", version)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("shutting down", "signal", sig.String())

	cancel() // Stop event hub.

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("forced shutdown", "error", err)
		os.Exit(1)
	}

	fmt.Println("HoLA agent stopped")
}
