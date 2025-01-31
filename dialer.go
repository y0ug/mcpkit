package mcpkit

import (
	"context"
	"io"
)

// Implement the ReadWriteCloser interface of jsonrpc2.Dialer
type StdioStream struct {
	reader io.Reader
	writer io.WriteCloser
	closer io.Closer
}

func (s *StdioStream) Read(p []byte) (int, error) {
	return s.reader.Read(p)
}

func (s *StdioStream) Write(p []byte) (int, error) {
	return s.writer.Write(p)
}

func (s *StdioStream) Close() error {
	if err := s.writer.Close(); err != nil {
		return err
	}
	if closer, ok := s.reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (s *StdioStream) Dial(ctx context.Context) (io.ReadWriteCloser, error) {
	// TODO: Check if already closed
	return s, nil
}
