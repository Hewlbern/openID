//go:build cssstub

package stream

import (
	"bytes"
	"context"
	"io"
	"sync"
)

// StreamUtil provides utility functions for stream operations
type StreamUtil struct{}

// NewStreamUtil creates a new StreamUtil
func NewStreamUtil() *StreamUtil {
	return &StreamUtil{}
}

// ReadAll reads all data from a reader
func (s *StreamUtil) ReadAll(reader io.Reader) ([]byte, error) {
	return io.ReadAll(reader)
}

// Copy copies data from a reader to a writer
func (s *StreamUtil) Copy(writer io.Writer, reader io.Reader) (int64, error) {
	return io.Copy(writer, reader)
}

// CopyBuffer copies data from a reader to a writer using a buffer
func (s *StreamUtil) CopyBuffer(writer io.Writer, reader io.Reader, buf []byte) (int64, error) {
	return io.CopyBuffer(writer, reader, buf)
}

// ReadAtLeast reads at least min bytes from a reader
func (s *StreamUtil) ReadAtLeast(reader io.Reader, buf []byte, min int) (int, error) {
	return io.ReadAtLeast(reader, buf, min)
}

// ReadFull reads exactly len(buf) bytes from a reader
func (s *StreamUtil) ReadFull(reader io.Reader, buf []byte) (int, error) {
	return io.ReadFull(reader, buf)
}

// WriteString writes a string to a writer
func (s *StreamUtil) WriteString(writer io.Writer, str string) (int, error) {
	return io.WriteString(writer, str)
}

// MultiReader creates a reader that reads from multiple readers
func (s *StreamUtil) MultiReader(readers ...io.Reader) io.Reader {
	return io.MultiReader(readers...)
}

// MultiWriter creates a writer that writes to multiple writers
func (s *StreamUtil) MultiWriter(writers ...io.Writer) io.Writer {
	return io.MultiWriter(writers...)
}

// TeeReader creates a reader that writes to a writer while reading
func (s *StreamUtil) TeeReader(reader io.Reader, writer io.Writer) io.Reader {
	return io.TeeReader(reader, writer)
}

// LimitReader creates a reader that reads at most n bytes
func (s *StreamUtil) LimitReader(reader io.Reader, n int64) io.Reader {
	return io.LimitReader(reader, n)
}

// SectionReader creates a reader that reads from a section of a reader
func (s *StreamUtil) SectionReader(reader io.ReaderAt, off int64, n int64) *io.SectionReader {
	return io.NewSectionReader(reader, off, n)
}

// Pipe creates a synchronous in-memory pipe
func (s *StreamUtil) Pipe() (*io.PipeReader, *io.PipeWriter) {
	return io.Pipe()
}

// NopCloser creates a reader that does nothing when closed
func (s *StreamUtil) NopCloser(reader io.Reader) io.ReadCloser {
	return io.NopCloser(reader)
}

// BufferedReadWriter provides buffered reading and writing
type BufferedReadWriter struct {
	reader *bytes.Reader
	writer *bytes.Buffer
	mu     sync.RWMutex
}

// NewBufferedReadWriter creates a new BufferedReadWriter
func (s *StreamUtil) NewBufferedReadWriter() *BufferedReadWriter {
	return &BufferedReadWriter{
		writer: bytes.NewBuffer(nil),
	}
}

// Write writes data to the buffer
func (b *BufferedReadWriter) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.writer.Write(p)
}

// Read reads data from the buffer
func (b *BufferedReadWriter) Read(p []byte) (int, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.reader == nil {
		b.reader = bytes.NewReader(b.writer.Bytes())
	}
	return b.reader.Read(p)
}

// Reset resets the buffer
func (b *BufferedReadWriter) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.writer.Reset()
	b.reader = nil
}

// Bytes returns the buffer's bytes
func (b *BufferedReadWriter) Bytes() []byte {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.writer.Bytes()
}

// String returns the buffer's string
func (b *BufferedReadWriter) String() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.writer.String()
}

// ContextReader provides context-aware reading
type ContextReader struct {
	reader  io.Reader
	ctx     context.Context
	timeout int64
}

// NewContextReader creates a new ContextReader
func (s *StreamUtil) NewContextReader(ctx context.Context, reader io.Reader) *ContextReader {
	return &ContextReader{
		reader: reader,
		ctx:    ctx,
	}
}

// Read reads data from the reader with context
func (c *ContextReader) Read(p []byte) (int, error) {
	select {
	case <-c.ctx.Done():
		return 0, c.ctx.Err()
	default:
		return c.reader.Read(p)
	}
}

// SetTimeout sets the read timeout
func (c *ContextReader) SetTimeout(timeout int64) {
	c.timeout = timeout
}

// GetTimeout gets the read timeout
func (c *ContextReader) GetTimeout() int64 {
	return c.timeout
}

// SetContext sets the context
func (c *ContextReader) SetContext(ctx context.Context) {
	c.ctx = ctx
}

// GetContext gets the context
func (c *ContextReader) GetContext() context.Context {
	return c.ctx
}
