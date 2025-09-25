package router

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"

	"go-mcp/mcp/tools"
	"go-mcp/mcp/types"
)

// MethodHandler processes an incoming JSON-RPC request and returns a result or an error.
type MethodHandler func(ctx context.Context, req types.Request) (any, *types.Error)

// Server implements a stdio-based MCP server.
type Server struct {
	decoder     *json.Decoder
	encoder     *json.Encoder
	writer      *bufio.Writer
	writerMutex sync.Mutex

	handlers  map[string]MethodHandler
	tools     map[string]tools.Registration
	toolOrder []string

	logger *log.Logger
	info   types.ServerInfo

	initialized bool
}

// Option customises server behavior during construction.
type Option func(*Server)

// WithLogger configures the logger used by the server.
func WithLogger(logger *log.Logger) Option {
	return func(s *Server) {
		if logger != nil {
			s.logger = logger
		}
	}
}

// WithServerInfo overrides the default server info advertised during initialization.
func WithServerInfo(name, version string) Option {
	return func(s *Server) {
		if name != "" {
			s.info.Name = name
		}
		if version != "" {
			s.info.Version = version
		}
	}
}

// NewServer 构建一个基于 stdio 的 MCP 服务器，并在初始化阶段绑定所有已注册工具。
func NewServer(r io.Reader, w io.Writer, opts ...Option) (*Server, error) {
	bufWriter := bufio.NewWriter(w)
	srv := &Server{
		decoder:   json.NewDecoder(r),
		encoder:   json.NewEncoder(bufWriter),
		writer:    bufWriter,
		handlers:  map[string]MethodHandler{},
		tools:     map[string]tools.Registration{},
		toolOrder: make([]string, 0),
		logger:    log.New(io.Discard, "", log.LstdFlags),
		info: types.ServerInfo{
			Name:    "go-mcp-server",
			Version: "dev",
		},
	}

	srv.decoder.UseNumber()
	srv.encoder.SetEscapeHTML(false)

	for _, opt := range opts {
		opt(srv)
	}

	srv.registerBuiltins()

	if err := srv.registerDefaultTools(); err != nil {
		return nil, err
	}

	return srv, nil
}

// Run processes JSON-RPC messages until the input stream is closed or the context is cancelled.
func (s *Server) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var req types.Request
		if err := s.decoder.Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("decode request: %w", err)
		}

		// Skip invalid protocol versions but respond with an error when possible.
		if req.JSONRPC != "" && req.JSONRPC != types.JSONRPCVersion {
			s.respondWithError(req, types.NewErrorf(types.CodeInvalidRequest, "expected jsonrpc version %s", types.JSONRPCVersion))
			continue
		}

		s.dispatch(ctx, req)
	}
}

// RegisterTool adds a new tool definition and handler.
func (s *Server) RegisterTool(def tools.ToolDefinition, handler tools.Handler) error {
	registration, err := tools.NewRegistration(def, handler)
	if err != nil {
		return err
	}

	name := registration.Definition.Name
	if _, exists := s.tools[name]; exists {
		return fmt.Errorf("tool %s already registered", name)
	}

	s.tools[name] = registration
	s.toolOrder = append(s.toolOrder, name)
	return nil
}

func (s *Server) dispatch(ctx context.Context, req types.Request) {
	handler, ok := s.handlers[req.Method]
	if !ok {
		s.logger.Printf("unknown method: %s", req.Method)
		s.respondWithError(req, types.NewErrorf(types.CodeMethodNotFound, "method %q not found", req.Method))
		return
	}

	result, errObj := handler(ctx, req)

	// Notifications do not carry an id and therefore do not expect a response.
	if req.ID == nil {
		return
	}

	resp := types.Response{
		JSONRPC: types.JSONRPCVersion,
		ID:      types.CloneID(req.ID),
	}

	if errObj != nil {
		resp.Error = errObj
	} else {
		if result == nil {
			resp.Result = struct{}{}
		} else {
			resp.Result = result
		}
	}

	if err := s.writeResponse(resp); err != nil {
		s.logger.Printf("failed to send response: %v", err)
	}
}

func (s *Server) respondWithError(req types.Request, errObj *types.Error) {
	if req.ID == nil {
		s.logger.Printf("dropping error for notification %s: %s", req.Method, errObj.Message)
		return
	}

	resp := types.Response{
		JSONRPC: types.JSONRPCVersion,
		ID:      types.CloneID(req.ID),
		Error:   errObj,
	}
	if err := s.writeResponse(resp); err != nil {
		s.logger.Printf("failed to send error response: %v", err)
	}
}

func (s *Server) writeResponse(resp types.Response) error {
	s.writerMutex.Lock()
	defer s.writerMutex.Unlock()

	if err := s.encoder.Encode(resp); err != nil {
		return err
	}
	return s.writer.Flush()
}

func (s *Server) registerBuiltins() {
	s.handlers["initialize"] = s.handleInitialize
	s.handlers["ping"] = s.handlePing
	s.handlers["tools/list"] = s.handleListTools
	s.handlers["tools/call"] = s.handleCallTool
}

func (s *Server) registerDefaultTools() error {
	for _, registration := range tools.Registered() {
		if err := s.RegisterTool(registration.Definition, registration.Handler); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) handleInitialize(_ context.Context, req types.Request) (any, *types.Error) {
	var params types.InitializeParams
	if len(req.Params) > 0 {
		if err := types.DecodeParams(req.Params, &params); err != nil {
			return nil, types.NewInvalidParamsError(err)
		}
	}

	s.initialized = true

	result := types.InitializeResult{
		ProtocolVersion: types.ProtocolVersion,
		ServerInfo:      s.info,
		Capabilities: types.ServerCapabilities{
			Tools: &types.ToolCapability{
				List: true,
				Call: true,
			},
		},
	}

	if params.ClientInfo != nil {
		s.logger.Printf("client connected: %s %s", params.ClientInfo.Name, params.ClientInfo.Version)
	}

	return result, nil
}

func (s *Server) handlePing(ctx context.Context, req types.Request) (any, *types.Error) {
	_ = ctx

	var params types.PingParams
	if len(req.Params) > 0 {
		if err := types.DecodeParams(req.Params, &params); err != nil {
			return nil, types.NewInvalidParamsError(err)
		}
	}

	return types.PingResult{Status: "ok", Message: params.Message}, nil
}

func (s *Server) handleListTools(ctx context.Context, req types.Request) (any, *types.Error) {
	_ = ctx
	_ = req

	toolDefinitions := make([]tools.ToolDefinition, 0, len(s.toolOrder))
	for _, name := range s.toolOrder {
		registration := s.tools[name]
		toolDefinitions = append(toolDefinitions, registration.Definition)
	}

	return tools.ListToolsResult{Tools: toolDefinitions}, nil
}

func (s *Server) handleCallTool(ctx context.Context, req types.Request) (any, *types.Error) {
	var params types.CallToolParams
	if len(req.Params) == 0 {
		return nil, types.NewInvalidParamsError(errors.New("missing params"))
	}
	if err := types.DecodeParams(req.Params, &params); err != nil {
		return nil, types.NewInvalidParamsError(err)
	}
	if params.Name == "" {
		return nil, types.NewInvalidParamsError(errors.New("missing tool name"))
	}

	registration, ok := s.tools[params.Name]
	if !ok {
		return nil, types.NewErrorf(types.CodeApplicationError, "tool %q not registered", params.Name)
	}

	result, err := registration.Handler(ctx, params.Arguments)
	if err != nil {
		return nil, types.NewToolError(err)
	}

	if result == nil {
		return types.CallToolResult{Content: []types.ContentItem{}}, nil
	}

	return result, nil
}
