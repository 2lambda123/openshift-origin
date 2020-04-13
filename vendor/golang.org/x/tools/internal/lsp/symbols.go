// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lsp

import (
	"context"

	"golang.org/x/tools/internal/lsp/protocol"
	"golang.org/x/tools/internal/lsp/source"
	"golang.org/x/tools/internal/span"
	"golang.org/x/tools/internal/telemetry/trace"
)

func (s *Server) documentSymbol(ctx context.Context, params *protocol.DocumentSymbolParams) ([]protocol.DocumentSymbol, error) {
	ctx, done := trace.StartSpan(ctx, "lsp.Server.documentSymbol")
	defer done()
	uri := span.NewURI(params.TextDocument.URI)
	view := s.session.ViewOf(uri)
	f, err := getGoFile(ctx, view, uri)
	if err != nil {
		return nil, err
	}
	m, err := getMapper(ctx, f)
	if err != nil {
		return nil, err
	}
	symbols, err := source.DocumentSymbols(ctx, f)
	if err != nil {
		return nil, err
	}
	return toProtocolDocumentSymbols(m, symbols), nil
}

func toProtocolDocumentSymbols(m *protocol.ColumnMapper, symbols []source.Symbol) []protocol.DocumentSymbol {
	result := make([]protocol.DocumentSymbol, 0, len(symbols))
	for _, s := range symbols {
		ps := protocol.DocumentSymbol{
			Name:     s.Name,
			Kind:     toProtocolSymbolKind(s.Kind),
			Detail:   s.Detail,
			Children: toProtocolDocumentSymbols(m, s.Children),
		}
		if r, err := m.Range(s.Span); err == nil {
			ps.Range = r
		}
		if r, err := m.Range(s.SelectionSpan); err == nil {
			ps.SelectionRange = r
		}
		result = append(result, ps)
	}
	return result
}

func toProtocolSymbolKind(kind source.SymbolKind) protocol.SymbolKind {
	switch kind {
	case source.StructSymbol:
		return protocol.Struct
	case source.PackageSymbol:
		return protocol.Package
	case source.VariableSymbol:
		return protocol.Variable
	case source.ConstantSymbol:
		return protocol.Constant
	case source.FunctionSymbol:
		return protocol.Function
	case source.MethodSymbol:
		return protocol.Method
	case source.InterfaceSymbol:
		return protocol.Interface
	case source.NumberSymbol:
		return protocol.Number
	case source.StringSymbol:
		return protocol.String
	case source.BooleanSymbol:
		return protocol.Boolean
	case source.FieldSymbol:
		return protocol.Field
	default:
		return 0
	}
}
