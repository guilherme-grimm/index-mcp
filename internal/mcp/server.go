package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/guilherme-grimm/ctxindex/internal/index"
	"github.com/guilherme-grimm/ctxindex/internal/pathutil"
)

type SymbolsInArgs struct {
	Path string `json:"path" jsonschema:"file path (absolute under project root, or root-relative) to list top-level symbols for"`
}

type ExportsOfArgs struct {
	Path string `json:"path" jsonschema:"file path (absolute under project root, or root-relative) to list exported symbols for"`
}

type CallersOfArgs struct {
	Symbol string `json:"symbol" jsonschema:"symbol name; bare (Foo) or qualified (pkg.Foo, pkg.Receiver.Method)"`
}

type ReadSymbolArgs struct {
	Name string `json:"name" jsonschema:"symbol name; bare (Foo) or qualified (pkg.Foo, pkg.Receiver.Method)"`
}

// JSON output shapes — match HANDOFF "MCP tool shapes" exactly.

type symbolOut struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Signature string `json:"signature,omitempty"`
	Line      int    `json:"line"`
	Exported  bool   `json:"exported"`
}

type symbolsResult struct {
	Symbols []symbolOut `json:"symbols"`
}

type readSymbolOne struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	Source string `json:"source"`
}

type readSymbolMatches struct {
	Matches []readSymbolOne `json:"matches"`
}

type callSiteOut struct {
	CallerFunc string `json:"caller_func"`
	File       string `json:"file"`
	Line       int    `json:"line"`
}

type callersResult struct {
	Symbol string        `json:"symbol"`
	Sites  []callSiteOut `json:"sites"`
}

func NewServer(name, version, root string, idx *index.Index) *mcpsdk.Server {
	srv := mcpsdk.NewServer(&mcpsdk.Implementation{Name: name, Version: version}, nil)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "symbols_in",
		Description: "List top-level symbols (func/type/var/const/method) in a file with kind, signature, and line. No bodies.",
	}, symbolsInHandler(root, idx))

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "exports_of",
		Description: "Like symbols_in, filtered to exported (capitalized) symbols.",
	}, exportsOfHandler(root, idx))

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "callers_of",
		Description: "List call sites of a symbol across the indexed project. Returns each site's caller function, file, and line.",
	}, callersOfHandler(idx))

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "read_symbol",
		Description: "Return the source of a single top-level symbol (function body, type decl, etc.) — not the whole file.",
	}, readSymbolHandler(idx))

	return srv
}

func symbolsInHandler(root string, idx *index.Index) func(context.Context, *mcpsdk.CallToolRequest, SymbolsInArgs) (*mcpsdk.CallToolResult, any, error) {
	return func(_ context.Context, _ *mcpsdk.CallToolRequest, args SymbolsInArgs) (*mcpsdk.CallToolResult, any, error) {
		abs, err := pathutil.Resolve(root, args.Path)
		if err != nil {
			return nil, nil, err
		}
		return nil, symbolsResult{Symbols: toSymbolOut(idx.SymbolsIn(abs))}, nil
	}
}

func exportsOfHandler(root string, idx *index.Index) func(context.Context, *mcpsdk.CallToolRequest, ExportsOfArgs) (*mcpsdk.CallToolResult, any, error) {
	return func(_ context.Context, _ *mcpsdk.CallToolRequest, args ExportsOfArgs) (*mcpsdk.CallToolResult, any, error) {
		abs, err := pathutil.Resolve(root, args.Path)
		if err != nil {
			return nil, nil, err
		}
		return nil, symbolsResult{Symbols: toSymbolOut(idx.ExportsOf(abs))}, nil
	}
}

func readSymbolHandler(idx *index.Index) func(context.Context, *mcpsdk.CallToolRequest, ReadSymbolArgs) (*mcpsdk.CallToolResult, any, error) {
	return func(_ context.Context, _ *mcpsdk.CallToolRequest, args ReadSymbolArgs) (*mcpsdk.CallToolResult, any, error) {
		matches, err := idx.ReadSymbol(args.Name)
		if err != nil {
			return nil, nil, err
		}
		if len(matches) == 0 {
			return nil, nil, fmt.Errorf("no symbol named %q", args.Name)
		}
		outs := make([]readSymbolOne, len(matches))
		for i, m := range matches {
			outs[i] = readSymbolOne{
				Name:   m.Name,
				Kind:   m.Kind,
				File:   m.File,
				Line:   m.Line,
				Source: m.Source,
			}
		}
		if len(outs) == 1 {
			return nil, outs[0], nil
		}
		return nil, readSymbolMatches{Matches: outs}, nil
	}
}

func callersOfHandler(idx *index.Index) func(context.Context, *mcpsdk.CallToolRequest, CallersOfArgs) (*mcpsdk.CallToolResult, any, error) {
	return func(_ context.Context, _ *mcpsdk.CallToolRequest, args CallersOfArgs) (*mcpsdk.CallToolResult, any, error) {
		sym, sites, err := idx.CallersOf(args.Symbol)
		if err != nil {
			return nil, nil, err
		}
		outs := make([]callSiteOut, len(sites))
		for i, s := range sites {
			outs[i] = callSiteOut{CallerFunc: s.CallerFunc, File: s.File, Line: s.Line}
		}
		return nil, callersResult{Symbol: sym, Sites: outs}, nil
	}
}

func toSymbolOut(in []index.Symbol) []symbolOut {
	out := make([]symbolOut, len(in))
	for i, s := range in {
		out[i] = symbolOut{
			Name:      s.Name,
			Kind:      s.Kind,
			Signature: s.Signature,
			Line:      s.Line,
			Exported:  s.Exported,
		}
	}
	return out
}