package main

import (
	"InariBlueNotifier/contacts"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	databasePath, err := contacts.DefaultDatabasePath()
	if err != nil {
		log.Fatalf("failed to determine contacts database path: %v", err)
	}
	flag.StringVar(&databasePath, "database", databasePath, "read-only contacts database path")
	flag.Parse()

	store, err := contacts.NewStore(databasePath)
	if err != nil {
		log.Fatalf("failed to open contacts database: %v", err)
	}
	defer store.Close()

	server, err := contacts.NewServer(store)
	if err != nil {
		log.Fatalf("failed to start contacts helper: %v", err)
	}
	defer server.Close()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case <-signals:
		return
	}
}
