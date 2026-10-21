package main

import (
	"InariBlueNotifier/config"
	"InariBlueNotifier/contacts"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	logger := config.NewLoggerWithoutFile()

	databasePath, err := contacts.DefaultDatabasePath()
	if err != nil {
		logger.Error("failed to determine contacts database path", slog.String("error", err.Error()))
		os.Exit(1)
	}
	flag.StringVar(&databasePath, "database", databasePath, "read-only contacts database path")
	flag.Parse()

	store, err := contacts.NewStore(databasePath)
	if err != nil {
		logger.Error("failed to open contacts database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer store.Close()

	server, err := contacts.NewServer(store, logger)
	if err != nil {
		logger.Error("failed to start contacts helper", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer server.Close()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case <-signals:
		return
	}
}
