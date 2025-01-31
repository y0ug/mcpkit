package mcpkit

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"golang.org/x/exp/jsonrpc2"
)

// LoggingFramer is a Framer decorator that logs frames on read/write.
type LoggingFramer struct {
	Base jsonrpc2.Framer // the underlying framer (e.g., HeaderFramer, RawFramer, etc.)
}

// Reader wraps the underlying framer's Reader with logging.
func (f *LoggingFramer) Reader(r io.Reader) jsonrpc2.Reader {
	baseReader := f.Base.Reader(r)
	return &loggingReader{base: baseReader}
}

// Writer wraps the underlying framer's Writer with logging.
func (f *LoggingFramer) Writer(w io.Writer) jsonrpc2.Writer {
	baseWriter := f.Base.Writer(w)
	return &loggingWriter{base: baseWriter}
}

// loggingReader implements Reader, wrapping calls to base.Read with logging.
type loggingReader struct {
	base jsonrpc2.Reader
}

func (r *loggingReader) Read(ctx context.Context) (jsonrpc2.Message, int64, error) {
	msg, n, err := r.base.Read(ctx)
	if err != nil {
		// Log the read error if desired
		fmt.Printf("[LoggingReader] Error: %v\n", err)
		return msg, n, err
	}
	// Log the successfully read frame
	fmt.Printf("[LoggingReader] Read %d bytes: %+v\n", n, msg)
	return msg, n, err
}

// loggingWriter implements Writer, wrapping calls to base.Write with logging.
type loggingWriter struct {
	base jsonrpc2.Writer
}

func (w *loggingWriter) Write(ctx context.Context, msg jsonrpc2.Message) (int64, error) {
	n, err := w.base.Write(ctx, msg)
	if err != nil {
		// Log the write error if desired
		fmt.Printf("[LoggingWriter] Error: %v\n", err)
		return n, err
	}
	// Log the successfully written frame
	fmt.Printf("[LoggingWriter] Wrote %d bytes: %+v\n", n, msg)
	return n, err
}

// NewLineRawFramer returns a Framer that encodes/decodes raw JSON messages
// exactly like RawFramer, but appends a newline at the end of each message
// on the wire.
func NewLineRawFramer() jsonrpc2.Framer {
	return newLineRawFramer{}
}

type newLineRawFramer struct{}

type newLineRawReader struct {
	in *bufio.Reader
}

type newLineRawWriter struct {
	out io.Writer
}

func (newLineRawFramer) Reader(r io.Reader) jsonrpc2.Reader {
	return &newLineRawReader{in: bufio.NewReader(r)}
}

func (newLineRawFramer) Writer(w io.Writer) jsonrpc2.Writer {
	return &newLineRawWriter{out: w}
}

func (r *newLineRawReader) Read(ctx context.Context) (jsonrpc2.Message, int64, error) {
	select {
	case <-ctx.Done():
		return nil, 0, ctx.Err()
	default:
	}

	// Read until the newline character
	line, err := r.in.ReadString('\n')
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read line: %w", err)
	}

	// Trim the newline and any other trailing whitespace
	line = strings.TrimSpace(line)
	if len(line) == 0 {
		return nil, 0, fmt.Errorf("empty message")
	}

	// Unmarshal the JSON message
	var raw json.RawMessage
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return nil, 0, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	msg, err := jsonrpc2.DecodeMessage(raw)
	return msg, int64(len(line)), err
}

func (w *newLineRawWriter) Write(ctx context.Context, msg jsonrpc2.Message) (int64, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	data, err := jsonrpc2.EncodeMessage(msg)
	if err != nil {
		return 0, fmt.Errorf("marshaling message: %w", err)
	}

	// Append a newline
	data = append(data, '\n')

	n, err := w.out.Write(data)
	return int64(n), err
}
