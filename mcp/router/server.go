package router

import (
	"bufio"
	"encoding/json"
	"fmt"
	"go-mcp/mcp/tools"
	"go-mcp/mcp/types"
	"io"
	"os"
)

// Server implements a stdio-based MCP server.
type Server struct {
	input  io.Reader
	output io.Writer

	tools map[string]types.MonitorTool

	info types.ServerInfo

	initialized bool
}

// Option customises server behavior during construction.
type Option func(*Server)

// NewServer 构建一个基于 stdio 的 MCP 服务器，并在初始化阶段绑定所有已注册工具。
func NewServer() *Server {
	return &Server{
		input:  os.Stdin,
		output: os.Stdout,
		tools:  make(map[string]types.MonitorTool),
		info: types.ServerInfo{
			Name:    "go-mcp-server",
			Version: "dev",
		},
	}
}

// Run processes JSON-RPC messages until the input stream is closed or the context is cancelled.
func (s *Server) Run() error {
	if s.initialized {
		return fmt.Errorf("路由器已经在运行")
	}

	// 启动 MCP 路由器，但不输出日志避免干扰 JSON-RPC
	s.initialized = true

	// 初始化工具
	if err := s.InitializeTools(); err != nil {
		return fmt.Errorf("初始化工具失败: %v", err)
	}

	// 启动消息处理循环
	return s.dispatch()
}

// InitializeTools 初始化所有监控工具
func (s *Server) InitializeTools() error {
	// 初始化监控工具，但不输出日志避免干扰 JSON-RPC

	// 创建工具实例
	cpuTool := tools.NewCPUTool()
	diskTool := tools.NewDiskTool()
	memoryTool := tools.NewMemoryTool()
	networkTool := tools.NewNetworkTool()
	processTool := tools.NewProcessTool()
	systemTool := tools.NewSystemTool()

	// 注册工具
	s.tools[cpuTool.GetName()] = cpuTool
	s.tools[diskTool.GetName()] = diskTool
	s.tools[memoryTool.GetName()] = memoryTool
	s.tools[networkTool.GetName()] = networkTool
	s.tools[processTool.GetName()] = processTool
	s.tools[systemTool.GetName()] = systemTool

	// 工具初始化完成，但不输出日志避免干扰 JSON-RPC

	return nil
}

func (s *Server) dispatch() error {
	scanner := bufio.NewScanner(s.input)
	for s.initialized && scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// 解析 JSON-RPC 请求
		var req types.Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			// 解析失败，但不输出日志到避免干扰 JSON-RPC
			// 发送解析错误响应（只有在有ID的情况下）
			var rawMessage map[string]interface{}
			json.Unmarshal([]byte(line), &rawMessage)
			if id, hasID := rawMessage["id"]; hasID {
				errorResp := &types.Response{
					JSONRPC: "2.0",
					ID:      id,
					Error: &types.Error{
						Code:    -32700,
						Message: "Parse error: " + err.Error(),
					},
				}
				s.writeResponse(errorResp)
			}
			continue
		}

		// 检查是否是通知（没有 ID 字段）
		isNotification := req.ID == nil

		// 处理请求
		response := s.handleRequest(&req)

		// 发送响应（只有非通知的请求才发送响应）
		if response != nil && !isNotification {
			s.writeResponse(response)
		}
	}

	if err := scanner.Err(); err != nil {
		// 扫描错误，但不输出到 stdout
		return fmt.Errorf("扫描输入时出错: %v", err)
	}

	return nil
}

func (s *Server) handleRequest(req *types.Request) *types.Response {
	switch req.Method {
	case types.MethodInitialize:
		return s.handleInitialize(req)
	//case types.MethodInitialized, types.MethodNotificationInitialized:
	//	return s.handleInitialized(req)
	case types.MethodListTools:
		return s.handleListTools(req)
	case types.MethodCallTool:
		return s.handleCallTool(req)
	//case types.MethodListPrompts:
	//	return s.handleListPrompts(req)
	//case types.MethodListResources:
	//	return s.handleListResources(req)
	//case types.MethodReadResource:
	//	return s.handleReadResource(req)
	default:
		return s.errorResponse(req, -32601, "Method not found: "+req.Method)
	}
}

// sendResponse 发送响应
func (s *Server) writeResponse(response *types.Response) {
	respBytes, err := json.Marshal(response)
	if err != nil {
		// 序列化失败，但不输出日志避免干扰 JSON-RPC
		return
	}

	if _, err := fmt.Fprintln(s.output, string(respBytes)); err != nil {
		// 发送失败，但不输出日志避免干扰 JSON-RPC
	}
}

// handleInitialize 处理初始化请求
func (s *Server) handleInitialize(req *types.Request) *types.Response {
	// 初始化服务器，但不输出日志避免干扰 JSON-RPC

	result := types.InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: types.ServerCapabilities{
			Tools: &types.ToolsCapability{
				ListChanged: true,
			},
			Resources: &types.ResourcesCapability{
				Subscribe:   false,
				ListChanged: false,
			},
			Prompts: &types.PromptsCapability{
				ListChanged: false,
			},
		},
		ServerInfo: types.ServerInfo{
			Name:    s.info.Name,
			Version: s.info.Version,
		},
	}

	return &types.Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// handleListTools 处理工具列表请求
func (s *Server) handleListTools(req *types.Request) *types.Response {
	// 列出工具，但不输出日志避免干扰 JSON-RPC

	var toolDefinitions []types.ToolDefinition
	for _, tool := range s.tools {
		mcpTool := types.ToolDefinition{
			Name:        tool.GetName(),
			Description: tool.GetDescription(),
			InputSchema: tool.GetInputSchema(),
		}
		toolDefinitions = append(toolDefinitions, mcpTool)
	}

	result := map[string]interface{}{
		"tools": toolDefinitions,
	}

	return &types.Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func (s *Server) handleCallTool(req *types.Request) *types.Response {
	var params types.CallToolParams
	if req.Params != nil {
		paramBytes, err := json.Marshal(req.Params)
		if err != nil {
			return s.errorResponse(req, -32602, "Invalid params: "+err.Error())
		}
		if err := json.Unmarshal(paramBytes, &params); err != nil {
			return s.errorResponse(req, -32602, "Invalid params: "+err.Error())
		}
	}

	// 调用工具，但不输出日志避免干扰 JSON-RPC

	// 查找工具
	tool, exists := s.tools[params.Name]
	if !exists {
		return s.errorResponse(req, -32602, "Unknown tool: "+params.Name)
	}

	// 执行工具
	result, err := tool.Execute(params.Arguments)
	if err != nil {
		// 工具执行失败，但不输出日志避免干扰 JSON-RPC
		return &types.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: types.CallToolResult{
				Content: []types.ContentItem{
					{Type: "text", Text: "❌ " + err.Error()},
				},
				IsError: true,
			},
		}
	}

	// 工具执行成功，但不输出日志避免干扰 JSON-RPC

	return &types.Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: types.CallToolResult{
			Content: []types.ContentItem{
				{Type: "text", Text: result},
			},
		},
	}
}

// errorResponse 创建错误响应
func (s *Server) errorResponse(req *types.Request, code int, message string) *types.Response {
	// 创建错误响应，但不输出日志避免干扰 JSON-RPC

	return &types.Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Error: &types.Error{
			Code:    code,
			Message: message,
		},
	}
}
