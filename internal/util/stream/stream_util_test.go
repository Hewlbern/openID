//go:build cssstub

package stream

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestStreamUtil_ReadAll(t *testing.T) {
	util := NewStreamUtil()
	reader := strings.NewReader("test data")
	data, err := util.ReadAll(reader)
	if err != nil {
		t.Errorf("ReadAll() error = %v", err)
	}
	if string(data) != "test data" {
		t.Errorf("ReadAll() = %v, want %v", string(data), "test data")
	}
}

func TestStreamUtil_Copy(t *testing.T) {
	util := NewStreamUtil()
	reader := strings.NewReader("test data")
	writer := new(bytes.Buffer)
	n, err := util.Copy(writer, reader)
	if err != nil {
		t.Errorf("Copy() error = %v", err)
	}
	if n != 9 {
		t.Errorf("Copy() = %v, want %v", n, 9)
	}
	if writer.String() != "test data" {
		t.Errorf("Copy() = %v, want %v", writer.String(), "test data")
	}
}

func TestStreamUtil_WriteString(t *testing.T) {
	util := NewStreamUtil()
	writer := new(bytes.Buffer)
	n, err := util.WriteString(writer, "test data")
	if err != nil {
		t.Errorf("WriteString() error = %v", err)
	}
	if n != 9 {
		t.Errorf("WriteString() = %v, want %v", n, 9)
	}
	if writer.String() != "test data" {
		t.Errorf("WriteString() = %v, want %v", writer.String(), "test data")
	}
}

func TestStreamUtil_MultiReader(t *testing.T) {
	util := NewStreamUtil()
	readers := []io.Reader{
		strings.NewReader("test "),
		strings.NewReader("data"),
	}
	reader := util.MultiReader(readers...)
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Errorf("MultiReader() error = %v", err)
	}
	if string(data) != "test data" {
		t.Errorf("MultiReader() = %v, want %v", string(data), "test data")
	}
}

func TestStreamUtil_MultiWriter(t *testing.T) {
	util := NewStreamUtil()
	buf1 := new(bytes.Buffer)
	buf2 := new(bytes.Buffer)
	writer := util.MultiWriter(buf1, buf2)
	_, err := writer.Write([]byte("test data"))
	if err != nil {
		t.Errorf("MultiWriter() error = %v", err)
	}
	if buf1.String() != "test data" {
		t.Errorf("MultiWriter() buf1 = %v, want %v", buf1.String(), "test data")
	}
	if buf2.String() != "test data" {
		t.Errorf("MultiWriter() buf2 = %v, want %v", buf2.String(), "test data")
	}
}

func TestStreamUtil_TeeReader(t *testing.T) {
	util := NewStreamUtil()
	reader := strings.NewReader("test data")
	writer := new(bytes.Buffer)
	teeReader := util.TeeReader(reader, writer)
	data, err := io.ReadAll(teeReader)
	if err != nil {
		t.Errorf("TeeReader() error = %v", err)
	}
	if string(data) != "test data" {
		t.Errorf("TeeReader() = %v, want %v", string(data), "test data")
	}
	if writer.String() != "test data" {
		t.Errorf("TeeReader() writer = %v, want %v", writer.String(), "test data")
	}
}

func TestStreamUtil_LimitReader(t *testing.T) {
	util := NewStreamUtil()
	reader := strings.NewReader("test data")
	limitReader := util.LimitReader(reader, 4)
	data, err := io.ReadAll(limitReader)
	if err != nil {
		t.Errorf("LimitReader() error = %v", err)
	}
	if string(data) != "test" {
		t.Errorf("LimitReader() = %v, want %v", string(data), "test")
	}
}

func TestBufferedReadWriter(t *testing.T) {
	util := NewStreamUtil()
	buf := util.NewBufferedReadWriter()

	// Test Write
	_, err := buf.Write([]byte("test data"))
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}

	// Test Bytes
	if string(buf.Bytes()) != "test data" {
		t.Errorf("Bytes() = %v, want %v", string(buf.Bytes()), "test data")
	}

	// Test String
	if buf.String() != "test data" {
		t.Errorf("String() = %v, want %v", buf.String(), "test data")
	}

	// Test Read
	data := make([]byte, 9)
	n, err := buf.Read(data)
	if err != nil {
		t.Errorf("Read() error = %v", err)
	}
	if n != 9 {
		t.Errorf("Read() = %v, want %v", n, 9)
	}
	if string(data) != "test data" {
		t.Errorf("Read() = %v, want %v", string(data), "test data")
	}

	// Test Reset
	buf.Reset()
	if buf.String() != "" {
		t.Errorf("Reset() = %v, want %v", buf.String(), "")
	}
}

func TestContextReader(t *testing.T) {
	util := NewStreamUtil()
	reader := strings.NewReader("test data")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	contextReader := util.NewContextReader(ctx, reader)
	data := make([]byte, 9)
	n, err := contextReader.Read(data)
	if err != nil {
		t.Errorf("Read() error = %v", err)
	}
	if n != 9 {
		t.Errorf("Read() = %v, want %v", n, 9)
	}
	if string(data) != "test data" {
		t.Errorf("Read() = %v, want %v", string(data), "test data")
	}

	// Test timeout
	ctx, cancel = context.WithTimeout(context.Background(), 0)
	defer cancel()
	contextReader.SetContext(ctx)
	_, err = contextReader.Read(data)
	if err != context.DeadlineExceeded {
		t.Errorf("Read() error = %v, want %v", err, context.DeadlineExceeded)
	}
}
