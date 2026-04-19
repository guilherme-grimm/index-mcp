package index

import "go/token"

type Symbol struct {
	Name      string
	Kind      string
	Signature string
	Package   string
	File      string
	Line      int
	Exported  bool
	Receiver  string

	// Source-slice range in the shared FileSet. Unexported so they never
	// leak into MCP JSON output; used by ReadSymbol to slice raw bytes.
	startPos token.Pos
	endPos   token.Pos
}

type CallSite struct {
	CallerFunc string
	File       string
	Line       int

	// Callee identity, used to filter by qualified input. Unexported so
	// they never leak into MCP JSON output — HANDOFF's shape is preserved.
	calleePkg  string
	calleeRecv string
}

type Index struct {
	symbols   map[string][]Symbol
	byFile    map[string][]Symbol
	callSites map[string][]CallSite
	fset      *token.FileSet
	pkgCount  int
}

func (idx *Index) Stats() (pkgs, syms, sites int) {
	for _, v := range idx.symbols {
		syms += len(v)
	}
	for _, v := range idx.callSites {
		sites += len(v)
	}
	return idx.pkgCount, syms, sites
}
