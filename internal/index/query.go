package index

import (
	"fmt"
	"os"
	"strings"
)

// SymbolWithSource extends Symbol with the raw source bytes of the decl.
type SymbolWithSource struct {
	Symbol
	Source string
}

// SymbolsIn returns top-level symbols declared in absPath. Unknown paths
// return an empty slice (not an error) — per HANDOFF, unknown files simply
// have no indexed symbols.
func (idx *Index) SymbolsIn(absPath string) []Symbol {
	return idx.byFile[absPath]
}

// ExportsOf is SymbolsIn filtered to exported (capitalized) symbols.
func (idx *Index) ExportsOf(absPath string) []Symbol {
	all := idx.SymbolsIn(absPath)
	out := make([]Symbol, 0, len(all))
	for _, s := range all {
		if s.Exported {
			out = append(out, s)
		}
	}
	return out
}

// ReadSymbol resolves name to matching symbols and returns their source.
// name forms: "Foo", "pkg.Foo", "pkg.Recv.Method". Empty slice when no
// match; error on malformed qualifier or file read failure.
func (idx *Index) ReadSymbol(name string) ([]SymbolWithSource, error) {
	if name == "" {
		return nil, fmt.Errorf("symbol name is empty")
	}
	parts := strings.Split(name, ".")

	var candidates []Symbol
	switch len(parts) {
	case 1:
		candidates = idx.symbols[parts[0]]
	case 2:
		pkg, nm := parts[0], parts[1]
		for _, s := range idx.symbols[nm] {
			if s.Package == pkg {
				candidates = append(candidates, s)
			}
		}
	case 3:
		pkg, recv, nm := parts[0], parts[1], parts[2]
		for _, s := range idx.symbols[nm] {
			if s.Package == pkg && s.Receiver == recv {
				candidates = append(candidates, s)
			}
		}
	default:
		return nil, fmt.Errorf("invalid qualified name %q: want Name, pkg.Name, or pkg.Recv.Method", name)
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	// Cache per-file reads across candidates in the same call.
	files := map[string][]byte{}
	out := make([]SymbolWithSource, 0, len(candidates))
	for _, s := range candidates {
		src, ok := files[s.File]
		if !ok {
			b, err := os.ReadFile(s.File)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", s.File, err)
			}
			src = b
			files[s.File] = b
		}
		startOff := idx.fset.Position(s.startPos).Offset
		endOff := idx.fset.Position(s.endPos).Offset
		if startOff < 0 || endOff > len(src) || startOff > endOff {
			return nil, fmt.Errorf("symbol %s: bad offsets [%d:%d] in %s", s.Name, startOff, endOff, s.File)
		}
		out = append(out, SymbolWithSource{Symbol: s, Source: string(src[startOff:endOff])})
	}
	return out, nil
}

// CallersOf returns call sites of the symbol. Input forms: "Foo",
// "pkg.Foo", "pkg.Recv.Method". The first return is the original input
// echoed back. Empty slice when no match; error on malformed qualifier.
func (idx *Index) CallersOf(input string) (string, []CallSite, error) {
	if input == "" {
		return "", nil, fmt.Errorf("symbol name is empty")
	}
	parts := strings.Split(input, ".")

	var name, wantPkg, wantRecv string
	switch len(parts) {
	case 1:
		name = parts[0]
	case 2:
		wantPkg, name = parts[0], parts[1]
	case 3:
		wantPkg, wantRecv, name = parts[0], parts[1], parts[2]
	default:
		return "", nil, fmt.Errorf("invalid qualified name %q: want Name, pkg.Name, or pkg.Recv.Method", input)
	}

	all := idx.callSites[name]
	out := make([]CallSite, 0, len(all))
	for _, s := range all {
		if wantPkg != "" && s.calleePkg != wantPkg {
			continue
		}
		if wantRecv != "" && s.calleeRecv != wantRecv {
			continue
		}
		out = append(out, s)
	}
	return input, out, nil
}
