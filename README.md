# GoMCP

A Go implementation of the Model Context Protocol (MCP) client. This package provides a clean and idiomatic way to interact with MCP servers in Go applications.

## Features

- Full MCP client implementation
- Support for all MCP operations (tools, resources, prompts)
- Streaming support
- Easy integration with chat completion systems

## Installation

```bash
go get github.com/yourusername/gomcp
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    
    "github.com/yourusername/gomcp/client"
)

func main() {
    ctx := context.Background()
    
    // Create a new MCP client
    c, err := client.NewMCPClient(ctx, log.Default(), "path/to/mcp-server")
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()
    
    // Initialize the client
    info, err := c.Initialize(ctx)
    if err != nil {
        log.Fatal(err)
    }
    
    // Use the client...
}
```

## Documentation

Coming soon

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see LICENSE file for details.
