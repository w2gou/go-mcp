package mcp

import "encoding/json"

const (
    // JSONRPCVersion is the version of the JSON-RPC protocol we speak.
    JSONRPCVersion = "2.0"
    // ProtocolVersion identifies the MCP protocol version supported by this server implementation.
    ProtocolVersion = "2024-06-24"
)

// request models a JSON-RPC request or notification envelope.
type request struct {
    JSONRPC string           `json:"jsonrpc"`
    ID      *json.RawMessage `json:"id,omitempty"`
    Method  string           `json:"method"`
    Params  json.RawMessage  `json:"params,omitempty"`
}

// response models a JSON-RPC response envelope.
type response struct {
    JSONRPC string           `json:"jsonrpc"`
    ID      *json.RawMessage `json:"id,omitempty"`
    Result  any              `json:"result,omitempty"`
    Error   *Error           `json:"error,omitempty"`
}

// Error represents a JSON-RPC error object.
type Error struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Data    any    `json:"data,omitempty"`
}

// JSON-RPC error codes used by this implementation.
const (
    CodeParseError     = -32700
    CodeInvalidRequest = -32600
    CodeMethodNotFound = -32601
    CodeInvalidParams  = -32602
    CodeInternalError  = -32603

    CodeApplicationError = -32001
    CodeToolError        = -32002
)

// InitializeParams represents the payload for the initialize request defined by MCP.
type InitializeParams struct {
    ClientInfo      *ClientInfo       `json:"clientInfo,omitempty"`
    ProtocolVersion string            `json:"protocolVersion"`
    Capabilities    json.RawMessage   `json:"capabilities,omitempty"`
    Metadata        map[string]string `json:"metadata,omitempty"`
}

// ClientInfo describes the connecting client.
type ClientInfo struct {
    Name    string `json:"name"`
    Version string `json:"version,omitempty"`
}

// InitializeResult is returned when the server handles an initialize request.
type InitializeResult struct {
    ProtocolVersion string            `json:"protocolVersion"`
    Capabilities    ServerCapabilities `json:"capabilities"`
    ServerInfo      ServerInfo        `json:"serverInfo"`
    Metadata        map[string]string `json:"metadata,omitempty"`
}

// ServerInfo announces information about this MCP server.
type ServerInfo struct {
    Name    string `json:"name"`
    Version string `json:"version,omitempty"`
}

// ServerCapabilities advertises the MCP capabilities provided by this server.
type ServerCapabilities struct {
    Tools *ToolCapability `json:"tools,omitempty"`
}

// ToolCapability signals which tool related methods are supported.
type ToolCapability struct {
    List bool `json:"list"`
    Call bool `json:"call"`
}

// PingParams represents the ping request payload.
type PingParams struct {
    Message string `json:"message,omitempty"`
}

// PingResult represents the ping response payload.
type PingResult struct {
    Status  string `json:"status"`
    Message string `json:"message,omitempty"`
}

// ListToolsResult is returned by the tools/list method.
type ListToolsResult struct {
    Tools []ToolDefinition `json:"tools"`
}

// CallToolParams is the payload for the tools/call method.
type CallToolParams struct {
    Name      string          `json:"name"`
    Arguments json.RawMessage `json:"arguments,omitempty"`
}

// CallToolResult wraps the response returned by a tool invocation.
type CallToolResult struct {
    Content []ContentItem `json:"content"`
}

// ContentItem is a minimal text-based MCP content payload.
type ContentItem struct {
    Type string `json:"type"`
    Text string `json:"text,omitempty"`
}
