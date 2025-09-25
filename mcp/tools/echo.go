package tools

import (
	"context"
	"encoding/json"
	"errors"

	"go-mcp/mcp/types"
)

// EchoTool returns the tool definition and handler for the echo tool.
func EchoTool() (ToolDefinition, Handler) {
	schema := JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"message": map[string]any{
				"type":        "string",
				"description": "Text to echo back to the caller.",
			},
		},
		"required": []string{"message"},
	}

	handler := func(ctx context.Context, arguments json.RawMessage) (*types.CallToolResult, error) {
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

		return &types.CallToolResult{
			Content: []types.ContentItem{TextContent(p.Message)},
		}, nil
	}

	return ToolDefinition{
		Name:        "echo",
		Description: "Echo a message back to the caller.",
		InputSchema: schema,
	}, handler
}

func init() {
	MustRegister(EchoTool())
}
