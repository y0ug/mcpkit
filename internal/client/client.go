package client

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"

	"golang.org/x/exp/jsonrpc2"
)

// Client defines the interface for MCP client operations
type Client interface {
	// Initialize sends the initialize request to the server and stores the capabilities
	Initialize(ctx context.Context) (*ServerInfo, error)

	// Ping sends a ping request to check if the server is alive
	Ping(ctx context.Context) error

	// ListTools requests the list of available tools from the server
	ListTools(ctx context.Context, cursor *string) ([]Tool, *string, error)

	// ListResources requests the list of available resources from the server
	ListResources(ctx context.Context, cursor *string) ([]Resource, *string, error)

	// ReadResource reads a specific resource from the server
	ReadResource(ctx context.Context, uri string) (*[]interface{}, error)

	// CallTool executes a specific tool with given parameters
	CallTool(ctx context.Context, name string, args map[string]interface{}) (*CallToolResult, error)

	// Close shuts down the MCP client and server
	Close() error
}

type client struct {
	conn     *jsonrpc2.Connection
	cancelFn context.CancelFunc
	ctx      context.Context
	logger   *slog.Logger
	doneChan chan error

	// Track initialization state
	initialized bool

	// Server capabilities received during initialization
	ServerInfo *ServerInfo

	cmd    *exec.Cmd
	Stream *Stream
}

type Stream struct {
	Stdin  io.WriteCloser
	Stdout io.ReadCloser
	Stderr io.ReadCloser
}

func FetchAll[T any](
	ctx context.Context,
	fetch func(ctx context.Context, cursor *string) ([]T, *string, error),
) ([]T, error) {
	var allItems []T
	var cursor *string

	for {
		items, nextCursor, err := fetch(ctx, cursor)
		if err != nil {
			return nil, fmt.Errorf("fetch failed: %w", err)
		}

		allItems = append(allItems, items...)

		if nextCursor == nil {
			break
		}

		cursor = nextCursor
	}

	return allItems, nil
}

func logHandler(logger *slog.Logger) jsonrpc2.HandlerFunc {
	return func(ctx context.Context, req *jsonrpc2.Request) (interface{}, error) {
		logger.Info("Request received",
			"method", req.Method,
			"id", req.ID.Raw(),
			"params", string(req.Params))
		return nil, jsonrpc2.ErrNotHandled
	}
}

type FatalServerError struct {
	Msg string
}

func (e *FatalServerError) Error() string {
	return e.Msg
}

// New creates a new MCP client and starts the language server
func New(
	ctxParent context.Context,
	logger *slog.Logger,
	serverCmd string,
	args ...string,
) (Client, error) {
	cmd := exec.Command(serverCmd, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start MCP server: %w", err)
	}

	// Channel to check if the process is running
	doneChan := make(chan error, 1)
	go func() {
		doneChan <- cmd.Wait()
	}()

	ctx, cancel := context.WithCancel(ctxParent)

	client := &client{
		cmd:      cmd,
		logger:   logger,
		ctx:      ctx,
		cancelFn: cancel,
		doneChan: doneChan,
	}
	// Start error monitoring in a goroutine
	go client.monitorErrors(stderr)

	dialer := &StdioStream{
		reader: stdout,
		writer: stdin,
	}

	// HeaderFramer is the jsonrpc2.Framer options
	// That's what MCP servers are expecting
	debug := false
	framer := NewLineRawFramer()
	if debug {
		framer = &LoggingFramer{
			Base: framer,
		}
	}

	conn, err := jsonrpc2.Dial(
		ctx,
		dialer,
		jsonrpc2.ConnectionOptions{
			Handler: logHandler(logger),
			Framer:  framer,
		},
	)
	if err != nil {
		cancel()
		cmd.Process.Kill()
		return nil, fmt.Errorf("dial error: %w", err)
	}
	client.conn = conn
	return client, nil
}

func (c *client) monitorErrors(stderr io.ReadCloser) {
	// Process and print stderr errors
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			errText := scanner.Text()
			if errText == "" {
				continue
			}

			c.logger.Debug("reading", "stderr", errText)

			// // Check for fatal errors
			if strings.Contains(strings.ToLower(errText), "error:") ||
				strings.Contains(strings.ToLower(errText), "fatal:") {
				c.logger.Error("error", "error", errText)
				// return
			}
		}

		// Check for scanner errors
		if err := scanner.Err(); err != nil {
			c.logger.Error("error reading stderr", "error", err)
		}
	}()

	// Monitor process exit
	for {
		select {
		case <-c.ctx.Done():
			return
		case err := <-c.doneChan:
			// if c.cmd.ProcessState != nil {
			c.logger.Error("process exited", "error", err)
			// }
			c.Close()
		}
	}
}

type ServerInfo InitializeResult

// Initialize sends the initialize request to the server and stores the capabilities
func (c *client) Initialize(ctx context.Context) (*ServerInfo, error) {
	method := "initialize"
	params := InitializeRequestParams{
		ClientInfo: Implementation{
			Name:    "mcptest",
			Version: "0.1.0",
		},
		ProtocolVersion: "2024-11-05",
		Capabilities:    ClientCapabilities{
			// Add capabilities as needed
		},
	}

	var result InitializeResult
	c.logger.Debug("Sending initialize request")
	if err := c.conn.Call(ctx, method, params).Await(c.ctx, &result); err != nil {
		return nil, fmt.Errorf("initialize failed: %w", err)
	}

	c.ServerInfo = (*ServerInfo)(&result)
	c.initialized = true

	c.logger.Debug("Server initialized",
		"name", c.ServerInfo.ServerInfo.Name,
		"version", c.ServerInfo.ServerInfo.Version)
	if c.ServerInfo.Instructions != nil {
		c.logger.Debug("Server instructions", "instructions", *c.ServerInfo.Instructions)
	}

	for k, v := range c.ServerInfo.Capabilities.Logging {
		c.logger.Debug("Capabilities Logging", "key", k, "value", v)
	}

	// Send initialized notification
	if err := c.conn.Notify(ctx, "notifications/initialized", nil); err != nil {
		return nil, fmt.Errorf("failed to send initialized notification: %w", err)
	}
	return c.ServerInfo, nil
}

// Ping sends a ping request to check if the server is alive
func (c *client) Ping(ctx context.Context) error {
	if !c.initialized {
		return fmt.Errorf("client not initialized")
	}
	if err := c.conn.Call(ctx, "ping", nil).Await(ctx, nil); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	return nil
}

// ListTools requests the list of available tools from the server
func (c *client) ListTools(ctx context.Context, cursor *string) ([]Tool, *string, error) {
	if !c.initialized {
		return nil, nil, fmt.Errorf("client not initialized")
	}
	params := &ListToolsRequestParams{Cursor: cursor}

	var result ListToolsResult
	if err := c.conn.Call(ctx, "tools/list", params).Await(ctx, &result); err != nil {
		return nil, nil, fmt.Errorf("list tools failed: %w", err)
	}

	return result.Tools, nil, nil
}

// ListResources requests the list of available resources from the server
func (c *client) ListResources(
	ctx context.Context,
	cursor *string,
) ([]Resource, *string, error) {
	if !c.initialized {
		return nil, nil, fmt.Errorf("client not initialized")
	}
	params := &ListResourcesRequestParams{Cursor: cursor}

	var result ListResourcesResult
	if err := c.conn.Call(ctx, "resources/list", params).Await(ctx, &result); err != nil {
		return nil, nil, fmt.Errorf("list resources failed: %w", err)
	}

	return result.Resources, result.NextCursor, nil
}

// ReadResource reads a specific resource from the server
func (c *client) ReadResource(
	ctx context.Context,
	uri string,
) (*[]interface{}, error) {
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}
	var result ReadResourceResult
	params := ReadResourceRequestParams{Uri: uri}
	if err := c.conn.Call(ctx, "resources/read", params).Await(ctx, &result); err != nil {
		return nil, fmt.Errorf("read resource failed: %w", err)
	}

	return &result.Contents, nil
}

// CallTool executes a specific tool with given parameters
func (c *client) CallTool(
	ctx context.Context,
	name string,
	args map[string]interface{},
) (*CallToolResult, error) {
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}
	params := CallToolRequestParams{
		Name:      name,
		Arguments: args,
	}
	var result CallToolResult
	if err := c.conn.Call(ctx, "tools/call", params).Await(ctx, &result); err != nil {
		return nil, fmt.Errorf("tool call failed: %w", err)
	}

	return &result, nil
}

// Close shuts down the MCP client and server
func (c *client) Close() error {
	// _ := context.Background()
	if c.initialized {
		c.initialized = false
	}

	// If we have an active connection, clean it up
	if c.conn != nil {
		ctx := context.Background()
		// Try to send exit notification
		_ = c.conn.Notify(ctx, "exit", nil)
		// Close the connection
		_ = c.conn.Close()
		c.conn = nil
	}

	select {
	case <-c.ctx.Done():
	default:
		c.logger.Debug("Closing MCP client")
		c.cancelFn()
		// Kill the process
		if c.cmd != nil && c.cmd.Process != nil {
			if c.cmd.ProcessState == nil {
				if err := c.cmd.Process.Kill(); err != nil {
					c.logger.Error("failed to kill process", "error", err)
				}
				if err := c.cmd.Wait(); err != nil {
					c.logger.Debug(
						"Process exited",
						"error",
						err,
						"code",
						c.cmd.ProcessState.ExitCode(),
					)
				}
			} else {
				c.logger.Debug("Process already exited", "code", c.cmd.ProcessState.ExitCode())
			}
		}
		// Cancel the context and wait for the process to finish

		c.logger.Debug("MCP client closed")
	}
	return nil
}
