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
)

const version = "0.1.0"

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

	authMiddleware := auth.NewMiddleware(*token)
	router := api.NewRouter(version, authMiddleware, dockerClient)

	srv := &http.Server{
		Addr:         ":8420",
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("forced shutdown", "error", err)
		os.Exit(1)
	}

	fmt.Println("HoLA agent stopped")
}
