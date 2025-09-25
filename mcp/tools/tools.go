package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"go-mcp/mcp/types"
)

// Handler executes a registered tool using the provided arguments.
type Handler func(ctx context.Context, arguments json.RawMessage) (*types.CallToolResult, error)

// JSONSchema represents a JSON schema document used to describe tool inputs.
type JSONSchema map[string]any

// ToolDefinition describes a tool that the server can execute.
type ToolDefinition struct {
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	InputSchema JSONSchema `json:"input_schema,omitempty"`
}

// ListToolsResult is returned by the tools/list method.
type ListToolsResult struct {
	Tools []ToolDefinition `json:"tools"`
}

// TextContent creates a text MCP content item.
func TextContent(text string) types.ContentItem {
	return types.ContentItem{
		Type: "text",
		Text: text,
	}
}

// Registration 用于封装工具定义与处理器的绑定关系。
type Registration struct {
	Definition ToolDefinition
	Handler    Handler
}

var (
	registryMu    sync.RWMutex
	registryIndex = map[string]Registration{}
	registryOrder []string
)

// NewRegistration 校验工具定义与处理器有效性，并补齐默认配置。
func NewRegistration(def ToolDefinition, handler Handler) (Registration, error) {
	if def.Name == "" {
		return Registration{}, fmt.Errorf("工具名称不能为空")
	}
	if handler == nil {
		return Registration{}, fmt.Errorf("工具 %s 的处理器为空", def.Name)
	}

	sanitized := def
	if sanitized.InputSchema == nil {
		sanitized.InputSchema = JSONSchema{"type": "object"}
	}

	return Registration{Definition: sanitized, Handler: handler}, nil
}

// Register 将工具注册到标准工具列表中，确保名称唯一且处理器有效。
func Register(def ToolDefinition, handler Handler) error {
	registration, err := NewRegistration(def, handler)
	if err != nil {
		return err
	}

	registryMu.Lock()
	defer registryMu.Unlock()

	name := registration.Definition.Name
	if _, exists := registryIndex[name]; exists {
		return fmt.Errorf("工具 %s 已存在", name)
	}

	registryIndex[name] = registration
	registryOrder = append(registryOrder, name)

	return nil
}

// MustRegister 在注册失败时直接 panic，便于在 init 阶段提前暴露问题。
func MustRegister(def ToolDefinition, handler Handler) {
	if err := Register(def, handler); err != nil {
		panic(err)
	}
}

// Registered 返回当前已注册的所有工具列表，按注册顺序排列。
func Registered() []Registration {
	registryMu.RLock()
	defer registryMu.RUnlock()

	result := make([]Registration, 0, len(registryOrder))
	for _, name := range registryOrder {
		result = append(result, registryIndex[name])
	}
	return result
}
