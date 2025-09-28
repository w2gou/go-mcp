package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"go-mcp/mcp/router"
)

func main() {
	server := router.NewServer()

	if err := server.Run(); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "服务器启动失败: %v\n", err)
		os.Exit(1)
	}
}
