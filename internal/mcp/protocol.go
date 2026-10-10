// Package mcp implements a Model Context Protocol server over stdio that
// exposes acurl checks as tools for AI assistants.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

// ProtocolVersion is the newest MCP revision this server implements.
const ProtocolVersion = "2025-06-18"

// supported lists revisions whose tool semantics this server handles.
var supported = map[string]bool{"2025-06-18": true, "2025-03-26": true, "2024-11-05": true}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// JSON-RPC error codes.
const (
	codeParse          = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternal       = -32603
)

// Tool is an MCP tool definition plus its handler.
type Tool struct {
	Name        string         `json:"name"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	Annotations map[string]any `json:"annotations,omitempty"`

	handle func(ctx context.Context, args json.RawMessage) (*ToolResult, error)
}

// ToolResult is the result of tools/call.
type ToolResult struct {
	Content           []Content `json:"content"`
	StructuredContent any       `json:"structuredContent,omitempty"`
	IsError           bool      `json:"isError,omitempty"`
}

// Content is one content block of a tool result.
type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func textResult(text string, structured any) *ToolResult {
	return &ToolResult{Content: []Content{{Type: "text", Text: text}}, StructuredContent: structured}
}

func toolError(msg string) *ToolResult {
	return &ToolResult{Content: []Content{{Type: "text", Text: msg}}, IsError: true}
}

// Server is an MCP server.
type Server struct {
	Name, Version string
	Instructions  string
	tools         []Tool

	out sync.Mutex
	wg  sync.WaitGroup
}

// Serve reads newline-delimited JSON-RPC messages from r and writes
// responses to w until r is closed or ctx is done. Tool calls run
// concurrently; responses may arrive out of order, as JSON-RPC allows.
func (s *Server) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), 16<<20)
	enc := json.NewEncoder(w)
	write := func(resp response) {
		s.out.Lock()
		defer s.out.Unlock()
		_ = enc.Encode(resp)
	}
	defer s.wg.Wait()

	for sc.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			write(response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{codeParse, "parse error"}})
			continue
		}
		if req.JSONRPC != "2.0" || req.Method == "" {
			if req.ID != nil {
				write(response{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{codeInvalidRequest, "invalid request"}})
			}
			continue
		}
		if req.ID == nil {
			continue // notifications (initialized, cancelled, ...) need no reply
		}
		if req.Method == "tools/call" {
			s.wg.Add(1)
			go func(req request) {
				defer s.wg.Done()
				write(s.dispatch(ctx, req))
			}(req)
			continue
		}
		write(s.dispatch(ctx, req))
	}
	return sc.Err()
}

func (s *Server) dispatch(ctx context.Context, req request) response {
	resp := response{JSONRPC: "2.0", ID: req.ID}
	result, err := s.handle(ctx, req)
	if err != nil {
		var re *rpcError
		if !errors.As(err, &re) {
			re = &rpcError{codeInternal, err.Error()}
		}
		resp.Error = re
		return resp
	}
	resp.Result = result
	return resp
}

func (e *rpcError) Error() string { return e.Message }

func (s *Server) handle(ctx context.Context, req request) (any, error) {
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &p)
		version := ProtocolVersion
		if supported[p.ProtocolVersion] {
			version = p.ProtocolVersion
		}
		return map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": s.Name, "title": "acurl", "version": s.Version},
			"instructions":    s.Instructions,
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": s.tools}, nil
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, &rpcError{codeInvalidParams, "invalid tools/call params"}
		}
		for _, t := range s.tools {
			if t.Name == p.Name {
				res, err := t.handle(ctx, p.Arguments)
				if err != nil {
					// Tool failures are reported in the result so the model can see them.
					return toolError(err.Error()), nil
				}
				return res, nil
			}
		}
		return nil, &rpcError{codeInvalidParams, fmt.Sprintf("unknown tool %q", p.Name)}
	}
	return nil, &rpcError{codeMethodNotFound, "method not found: " + req.Method}
}
