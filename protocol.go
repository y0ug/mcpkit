package mcpkit

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"golang.org/x/exp/jsonrpc2"
)

type Protocol interface {
	Start(ctx context.Context) error

	Request(ctx context.Context, message *jsonrpc2.Request) error

	Close() error

	// AddHandler sets the callback for when a message (request, notification or response) is received
	AddHandler(handler func(ctx context.Context, handler jsonrpc2.HandlerFunc))
}

// protocol is our MCP implementation.
type protocol struct {
	logger     *slog.Logger
	handlers   map[string]jsonrpc2.HandlerFunc
	mu         sync.Mutex
	cancelFunc context.CancelFunc
}

// NewServer creates a new Server instance with the given logger.
func NewProcol(logger *slog.Logger) *protocol {
	s := &protocol{
		logger:   logger,
		handlers: make(map[string]jsonrpc2.HandlerFunc),
	}
	s.AddHandler("initialize", s.handleInitialize)
	s.AddHandler("ping", s.handlePing)
	s.AddHandler("tools/list", s.handleToolsList)
	return s
}

// Serve starts the MCP server on stdio, handling requests until EOF or signal.
func (s *protocol) Serve(ctx context.Context) error {
	ctx, s.cancelFunc = signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer s.cancelFunc()
	// // Make sure we handle Ctrl+C / SIGINT so we can exit gracefully
	// ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	// defer cancel()

	// We'll read from stdin and write to stdout
	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout

	// Set up the framer to read/write each JSON message delimited by a newline
	framer := NewLineRawFramer()

	// Set up our JSON-RPC handler that dispatches to MCP methods
	handler := jsonrpc2.HandlerFunc(s.handle)

	dialer := &StdioStream{
		reader: reader,
		writer: writer,
	}
	// Build the server connection.
	// conn := jsonrpc2.NetDialer(reader, writer, handler, framer)
	conn, err := jsonrpc2.Dial(
		ctx,
		dialer,
		jsonrpc2.ConnectionOptions{Handler: handler, Framer: framer},
	)
	if err != nil {
		s.cancelFunc()
		return fmt.Errorf("failed to create the MCP server: %w", err)
	}
	defer conn.Close()
	return conn.Wait()
}

func (p *protocol) AddHandler(
	method string,
	handler jsonrpc2.HandlerFunc,
) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers[method] = handler
}

// handle processes each incoming JSON-RPC 2.0 request by method name.
func (s *protocol) handle(ctx context.Context, r *jsonrpc2.Request) (resp interface{}, err error) {
	s.logger.Debug("Server received request",
		"method", r.Method,
		"id", r.ID.Raw(),
		"params", string(r.Params))

	if !r.ID.IsValid() {
		// notification we process them like
		// a classic handler the method name should not overlap
	}
	if handler, ok := s.handlers[r.Method]; ok {
		resp, err = handler(ctx, r)
	} else if r.Method == "exit" {
	} else {
		err = jsonrpc2.ErrNotHandled
	}
	return
}

// handleInitialize implements the MCP "initialize" request.
func (s *protocol) handleInitialize(
	ctx context.Context,
	r *jsonrpc2.Request,
) (result interface{}, err error) {
	var params InitializeRequestParams
	if err = json.Unmarshal(r.Params, &params); err != nil {
		return nil, fmt.Errorf("failed to unmarshal initialize params: %w", err)
	}

	// Prepare our InitializeResult
	result = InitializeResult{
		ProtocolVersion: "2024-11-05",
		ServerInfo: Implementation{
			Name:    "time",
			Version: "0.0.1",
		},
		Capabilities: ServerCapabilities{
			Tools: &ServerCapabilitiesTools{
				// We do have tools, so we can set "listChanged" if we ever notify
				ListChanged: new(bool),
			},
		},
	}

	// Return the result. The jsonrpc2 library will encode it as JSON for us.
	return result, nil
}

func (s *protocol) handlePing(
	ctx context.Context,
	r *jsonrpc2.Request,
) (result interface{}, err error) {
	return
}

func (s *protocol) handleToolsList(
	ctx context.Context,
	r *jsonrpc2.Request,
) (result interface{}, err error) {
	// var params client.ListToolsRequestParams
	// _ = json.Unmarshal(r.Params, &params) // ignoring errors for brevity

	// Return a single "time" tool with an empty input schema (just for demo).
	result = ListToolsResult{
		Tools: []Tool{
			{
				Name:        "time",
				Description: strPtr("Return the current server time."),
				InputSchema: ToolInputSchema{
					Type: "object",
					// No required properties in this example
				},
			},
		},
	}

	return result, nil
}

// strPtr is a small helper to take a string literal and return *string
func strPtr(s string) *string {
	return &s
}
