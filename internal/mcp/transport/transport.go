// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package transport serves the tools of internal/mcp/tools over MCP on stdio.
// It is the only package in the repository that imports the protocol library
// (the official Go SDK, pinned in go.mod): the tools stay plain functions, and
// the architecture test keeps the library out of every other package under
// internal/. One process answers both protocol eras, the initialize handshake of
// 2025-11-25 and earlier and the per-request metadata of 2026-07-28, because the
// library does; see docs/internal/plans/mcp-delegation-spec.md, R1.
//
// Downshift by Tiago Vilas Boas
// https://github.com/tiagovilasboas/downshift
package transport

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tiagovilasboas/downshift/internal/mcp/tools"
)

// MaxMessageBytes caps one incoming message, like the 1 MiB cap of the hooks.
const MaxMessageBytes = 1 << 20

// Info names the server to its clients.
type Info struct {
	Name         string
	Version      string
	Instructions string
}

// Server is an MCP server for a fixed set of tools.
type Server struct{ srv *mcp.Server }

// New builds a server for ts. log receives protocol and panic notes and must
// not write to standard output, which carries only protocol messages; nil
// discards them.
func New(info Info, ts []tools.Tool, log *slog.Logger) (*Server, error) {
	if err := tools.Validate(ts); err != nil {
		return nil, err
	}
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	s := mcp.NewServer(&mcp.Implementation{Name: info.Name, Version: info.Version},
		&mcp.ServerOptions{Instructions: info.Instructions, Logger: log})
	for _, t := range ts {
		s.AddTool(&mcp.Tool{Name: t.Name, Description: t.Description, InputSchema: t.Schema()}, adapt(t, log))
	}
	return &Server{srv: s}, nil
}

// Serve answers one client over in and out until the client closes the input
// or ctx ends. Messages are newline-delimited JSON.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	return s.srv.Run(ctx, &mcp.IOTransport{
		Reader:        io.NopCloser(in),
		Writer:        nopWriteCloser{out},
		MaxLineLength: MaxMessageBytes,
	})
}

// ServeStdio is Serve on the process's standard streams.
func (s *Server) ServeStdio(ctx context.Context) error {
	return s.Serve(ctx, os.Stdin, os.Stdout)
}

// adapt turns a plain tool into a library handler. A handler error becomes a
// failed result, so the model reads it; a panic becomes a failed result and the
// server keeps serving.
func adapt(t tools.Tool, log *slog.Logger) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (res *mcp.CallToolResult, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("tool panicked", "tool", t.Name)
				res, err = failed("internal error"), nil
			}
		}()
		var args json.RawMessage
		if req != nil && req.Params != nil {
			args = req.Params.Arguments
		}
		out, herr := t.Handler(ctx, args)
		if herr != nil {
			return failed(herr.Error()), nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: out.Text}},
			IsError: out.IsError,
		}, nil
	}
}

func failed(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: msg}}, IsError: true}
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }
