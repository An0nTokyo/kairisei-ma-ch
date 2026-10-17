package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kairisei.local/mserver/internal/server"
)

func main() {
	portText := flag.String("port", "26020", "game port")
	dataDir := flag.String("data", "", "data directory")
	flag.Parse()
	if *dataDir == "" {
		fmt.Fprintln(os.Stderr, "data directory is required")
		os.Exit(1)
	}
	running, err := server.Start(*dataDir, *portText)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	<-signals
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := running.Shutdown(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
