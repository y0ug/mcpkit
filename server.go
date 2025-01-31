package mcpkit

import "sync"

type Server struct {
	protocol   *protocol
	tools      sync.Map
	serverInfo ServerInfo
}

func (s *Server) RegisterTool(name string, description string, handler any) {
	tool := &Tool{
		Name:        name,
		Description: &description,
		InputSchema: ToolInputSchema{},
	}
	s.tools.Store(name, tool)
}
