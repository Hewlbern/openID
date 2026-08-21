package openidmcp

import (
	"bufio"
	"encoding/json"
	"io"
)

// ServeStdio speaks newline-delimited JSON-RPC on stdin/stdout for grokbot.
func (s *Server) ServeStdio(in io.Reader, out io.Writer) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	enc := json.NewEncoder(out)
	for sc.Scan() {
		line := sc.Bytes()
		if len(bytesTrimSpace(line)) == 0 {
			continue
		}
		resp := s.Handle(line)
		if resp == nil {
			continue
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	return sc.Err()
}
