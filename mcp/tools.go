package mcp

// JSONSchema represents a JSON schema document used to describe tool inputs.
type JSONSchema map[string]any

// ToolDefinition describes a tool that the server can execute.
type ToolDefinition struct {
    Name        string      `json:"name"`
    Description string      `json:"description,omitempty"`
    InputSchema JSONSchema  `json:"input_schema,omitempty"`
}

// TextContent creates a text MCP content item.
func TextContent(text string) ContentItem {
    return ContentItem{
        Type: "text",
        Text: text,
    }
}
