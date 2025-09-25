package mcp

import (
    "bufio"
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "log"
    "sync"
)

// MethodHandler processes an incoming JSON-RPC request and returns a result or an error.
type MethodHandler func(ctx context.Context, req request) (any, *Error)

// ToolHandler executes a registered tool using the provided arguments.
type ToolHandler func(ctx context.Context, arguments json.RawMessage) (*CallToolResult, error)

// Server implements a stdio-based MCP server.
type Server struct {
    decoder     *json.Decoder
    encoder     *json.Encoder
    writer      *bufio.Writer
    writerMutex sync.Mutex

    handlers map[string]MethodHandler
    tools    map[string]ToolRegistration

    logger *log.Logger
    info   ServerInfo

    initialized bool
}

// ToolRegistration binds a tool definition to its handler function.
type ToolRegistration struct {
    Definition ToolDefinition
    Handler    ToolHandler
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

// NewServer constructs a stdio MCP server.
func NewServer(r io.Reader, w io.Writer, opts ...Option) *Server {
    bufWriter := bufio.NewWriter(w)
    srv := &Server{
        decoder: json.NewDecoder(r),
        encoder: json.NewEncoder(bufWriter),
        writer:  bufWriter,
        handlers: map[string]MethodHandler{},
        tools:    map[string]ToolRegistration{},
        logger:   log.New(io.Discard, "", log.LstdFlags),
        info: ServerInfo{
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

    return srv
}

// Run processes JSON-RPC messages until the input stream is closed or the context is cancelled.
func (s *Server) Run(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        var req request
        if err := s.decoder.Decode(&req); err != nil {
            if errors.Is(err, io.EOF) {
                return nil
            }
            return fmt.Errorf("decode request: %w", err)
        }

        // Skip invalid protocol versions but respond with an error when possible.
        if req.JSONRPC != "" && req.JSONRPC != JSONRPCVersion {
            s.respondWithError(req, &Error{
                Code:    CodeInvalidRequest,
                Message: fmt.Sprintf("expected jsonrpc version %s", JSONRPCVersion),
            })
            continue
        }

        s.dispatch(ctx, req)
    }
}

// RegisterTool adds a new tool definition and handler.
func (s *Server) RegisterTool(def ToolDefinition, handler ToolHandler) error {
    if def.Name == "" {
        return fmt.Errorf("tool name is required")
    }
    if handler == nil {
        return fmt.Errorf("tool handler for %s is nil", def.Name)
    }
    if _, exists := s.tools[def.Name]; exists {
        return fmt.Errorf("tool %s already registered", def.Name)
    }
    if def.InputSchema == nil {
        def.InputSchema = JSONSchema{
            "type": "object",
        }
    }

    s.tools[def.Name] = ToolRegistration{Definition: def, Handler: handler}
    return nil
}

func (s *Server) dispatch(ctx context.Context, req request) {
    handler, ok := s.handlers[req.Method]
    if !ok {
        s.logger.Printf("unknown method: %s", req.Method)
        s.respondWithError(req, &Error{
            Code:    CodeMethodNotFound,
            Message: fmt.Sprintf("method %q not found", req.Method),
        })
        return
    }

    result, errObj := handler(ctx, req)

    // Notifications do not carry an id and therefore do not expect a response.
    if req.ID == nil {
        return
    }

    resp := response{
        JSONRPC: JSONRPCVersion,
        ID:      cloneID(req.ID),
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

func (s *Server) respondWithError(req request, errObj *Error) {
    if req.ID == nil {
        s.logger.Printf("dropping error for notification %s: %s", req.Method, errObj.Message)
        return
    }

    resp := response{
        JSONRPC: JSONRPCVersion,
        ID:      cloneID(req.ID),
        Error:   errObj,
    }
    if err := s.writeResponse(resp); err != nil {
        s.logger.Printf("failed to send error response: %v", err)
    }
}

func (s *Server) writeResponse(resp response) error {
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

func (s *Server) handleInitialize(ctx context.Context, req request) (any, *Error) {
    var params InitializeParams
    if len(req.Params) > 0 {
        if err := decodeParams(req.Params, &params); err != nil {
            return nil, newInvalidParamsError(err)
        }
    }

    s.initialized = true

    result := InitializeResult{
        ProtocolVersion: ProtocolVersion,
        ServerInfo:      s.info,
        Capabilities: ServerCapabilities{
            Tools: &ToolCapability{
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

func (s *Server) handlePing(ctx context.Context, req request) (any, *Error) {
    _ = ctx

    var params PingParams
    if len(req.Params) > 0 {
        if err := decodeParams(req.Params, &params); err != nil {
            return nil, newInvalidParamsError(err)
        }
    }

    return PingResult{Status: "ok", Message: params.Message}, nil
}

func (s *Server) handleListTools(ctx context.Context, req request) (any, *Error) {
    _ = ctx
    _ = req

    tools := make([]ToolDefinition, 0, len(s.tools))
    for _, registration := range s.tools {
        tools = append(tools, registration.Definition)
    }

    return ListToolsResult{Tools: tools}, nil
}

func (s *Server) handleCallTool(ctx context.Context, req request) (any, *Error) {
    var params CallToolParams
    if len(req.Params) == 0 {
        return nil, newInvalidParamsError(errors.New("missing params"))
    }
    if err := decodeParams(req.Params, &params); err != nil {
        return nil, newInvalidParamsError(err)
    }
    if params.Name == "" {
        return nil, newInvalidParamsError(errors.New("missing tool name"))
    }

    registration, ok := s.tools[params.Name]
    if !ok {
        return nil, &Error{
            Code:    CodeApplicationError,
            Message: fmt.Sprintf("tool %q not registered", params.Name),
        }
    }

    result, err := registration.Handler(ctx, params.Arguments)
    if err != nil {
        return nil, &Error{
            Code:    CodeToolError,
            Message: err.Error(),
        }
    }

    if result == nil {
        return CallToolResult{Content: []ContentItem{}}, nil
    }

    return result, nil
}

func cloneID(id *json.RawMessage) *json.RawMessage {
    if id == nil {
        return nil
    }
    clone := make(json.RawMessage, len(*id))
    copy(clone, *id)
    return &clone
}

func newInvalidParamsError(err error) *Error {
    return &Error{
        Code:    CodeInvalidParams,
        Message: err.Error(),
    }
}

func decodeParams(raw json.RawMessage, target any) error {
    decoder := json.NewDecoder(bytes.NewReader(raw))
    decoder.DisallowUnknownFields()
    return decoder.Decode(target)
}
