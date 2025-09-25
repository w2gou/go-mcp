package main

import (
	"context"
	"errors"
	"log"
	"os"

	"go-mcp/mcp/router"
)

func main() {
	logger := log.New(os.Stderr, "[mcp] ", log.LstdFlags)

	server, err := router.NewServer(os.Stdin, os.Stdout,
		router.WithLogger(logger),
		router.WithServerInfo("go-mcp-example", "0.1.0"),
	)
	if err != nil {
		logger.Fatalf("unable to create server: %v", err)
	}

	if err := server.Run(context.Background()); err != nil && !errors.Is(err, context.Canceled) {
		logger.Fatalf("server exited with error: %v", err)
	}
}
