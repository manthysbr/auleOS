package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/manthysbr/auleOS/pkg/watchdog"
)

func main() {
	port := flag.Int("port", 8080, "HTTP port to listen on")
	socketPath := flag.String("socket", "", "Unix socket path (if set, overrides TCP)")
	flag.Parse()

	cfg := watchdog.Config{
		Port:       *port,
		SocketPath: *socketPath,
	}

	server := watchdog.NewServer(cfg)

	// Start server in goroutine
	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Shutdown failed: %v", err)
	}
	log.Println("Watchdog stopped")
}
