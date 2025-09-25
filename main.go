package main

import (
    "context"
    "encoding/json"
    "errors"
    "log"
    "os"

    "go-mcp/mcp"
)

func main() {
    logger := log.New(os.Stderr, "[mcp] ", log.LstdFlags)

    server := mcp.NewServer(os.Stdin, os.Stdout,
        mcp.WithLogger(logger),
        mcp.WithServerInfo("go-mcp-example", "0.1.0"),
    )

    if err := registerEchoTool(server); err != nil {
        logger.Fatalf("unable to register echo tool: %v", err)
    }

    if err := server.Run(context.Background()); err != nil && !errors.Is(err, context.Canceled) {
        logger.Fatalf("server exited with error: %v", err)
    }
}

func registerEchoTool(server *mcp.Server) error {
    schema := mcp.JSONSchema{
        "type": "object",
        "properties": map[string]any{
            "message": map[string]any{
                "type":        "string",
                "description": "Text to echo back to the caller.",
            },
        },
        "required": []string{"message"},
    }

    return server.RegisterTool(mcp.ToolDefinition{
        Name:        "echo",
        Description: "Echo a message back to the caller.",
        InputSchema: schema,
    }, func(ctx context.Context, arguments json.RawMessage) (*mcp.CallToolResult, error) {
        type payload struct {
            Message string `json:"message"`
        }

        var p payload
        if err := json.Unmarshal(arguments, &p); err != nil {
            return nil, err
        }
        if p.Message == "" {
            return nil, errors.New("message cannot be empty")
        }

        return &mcp.CallToolResult{
            Content: []mcp.ContentItem{mcp.TextContent(p.Message)},
        }, nil
    })
}
