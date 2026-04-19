package index

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/packages"
)

func Build(root string) (*Index, error) {
	pkgs, err := loadPackages(root)
	if err != nil {
		return nil, err
	}

	idx := &Index{
		symbols:   map[string][]Symbol{},
		byFile:    map[string][]Symbol{},
		callSites: map[string][]CallSite{},
		fset:      token.NewFileSet(),
	}

	firstParty := map[string]bool{}
	for _, p := range pkgs {
		if p.PkgPath != "" {
			firstParty[p.PkgPath] = true
		}
	}

	// packages.Load returns a shared FileSet per package. Since all first-party
	// packages are loaded in one call they share a single *token.FileSet; grab
	// the first non-nil one.
	for _, p := range pkgs {
		if p.Fset != nil {
			idx.fset = p.Fset
			break
		}
	}

	pkgCount := 0
	for _, p := range pkgs {
		if len(p.Syntax) == 0 {
			continue
		}
		pkgCount++
		for _, file := range p.Syntax {
			indexFile(idx, p, file, firstParty)
		}
	}

	idx.pkgCount = pkgCount
	return idx, nil
}

func indexFile(idx *Index, p *packages.Package, file *ast.File, firstParty map[string]bool) {
	tokFile := idx.fset.File(file.Pos())
	if tokFile == nil {
		return
	}
	absPath := tokFile.Name()

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			sym := funcSymbol(idx.fset, p.Name, absPath, d)
			idx.symbols[sym.Name] = append(idx.symbols[sym.Name], sym)
			idx.byFile[absPath] = append(idx.byFile[absPath], sym)
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					// Slice the TypeSpec itself — inside a `type (...)` block,
					// each symbol returns just its spec, not the whole block.
					sym := Symbol{
						Name:     s.Name.Name,
						Kind:     "type",
						Package:  p.Name,
						File:     absPath,
						Line:     idx.fset.Position(s.Pos()).Line,
						Exported: s.Name.IsExported(),
						startPos: s.Pos(),
						endPos:   s.End(),
					}
					idx.symbols[sym.Name] = append(idx.symbols[sym.Name], sym)
					idx.byFile[absPath] = append(idx.byFile[absPath], sym)
				case *ast.ValueSpec:
					kind := "var"
					if d.Tok == token.CONST {
						kind = "const"
					}
					// Multi-name specs (var a, b = ...) all return the full
					// spec range; acceptable for v0.
					for _, name := range s.Names {
						sym := Symbol{
							Name:     name.Name,
							Kind:     kind,
							Package:  p.Name,
							File:     absPath,
							Line:     idx.fset.Position(name.Pos()).Line,
							Exported: name.IsExported(),
							startPos: s.Pos(),
							endPos:   s.End(),
						}
						idx.symbols[sym.Name] = append(idx.symbols[sym.Name], sym)
						idx.byFile[absPath] = append(idx.byFile[absPath], sym)
					}
				}
			}
		}
	}

	// Call sites: walk every CallExpr while tracking the enclosing FuncDecl.
	//
	// Interface method calls resolve via TypesInfo.Uses to the *types.Func on
	// the interface itself (not concrete implementations); the call is
	// attributed to the interface method name. Function-value calls
	// (f := pkg.Foo; f()) resolve to a *types.Var — skipped in v0.
	var stack []*ast.FuncDecl
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			// Pop on post-order visit.
			if len(stack) > 0 {
				// The inspector calls with nil after leaving a node; we only
				// want to pop when leaving the top-of-stack FuncDecl. Handled
				// below by matching on FuncDecl re-entry instead.
			}
			return false
		}
		if fd, ok := n.(*ast.FuncDecl); ok {
			stack = append(stack, fd)
			// Recurse children manually so we can pop afterward.
			if fd.Body != nil {
				ast.Inspect(fd.Body, func(m ast.Node) bool {
					call, ok := m.(*ast.CallExpr)
					if !ok {
						return true
					}
					recordCallSite(idx, p, fd, call, firstParty)
					return true
				})
			}
			stack = stack[:len(stack)-1]
			return false
		}
		// Top-level call expressions outside any FuncDecl (e.g. var = f()).
		if call, ok := n.(*ast.CallExpr); ok {
			recordCallSite(idx, p, nil, call, firstParty)
		}
		return true
	})
}

func recordCallSite(idx *Index, p *packages.Package, enclosing *ast.FuncDecl, call *ast.CallExpr, firstParty map[string]bool) {
	var ident *ast.Ident
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		ident = fn
	case *ast.SelectorExpr:
		ident = fn.Sel
	default:
		return
	}
	obj := p.TypesInfo.Uses[ident]
	if obj == nil {
		return
	}
	fnObj, ok := obj.(*types.Func)
	if !ok {
		return
	}
	if fnObj.Pkg() == nil {
		return
	}
	if !firstParty[fnObj.Pkg().Path()] {
		return
	}
	site := CallSite{
		CallerFunc: callerFuncName(p.Name, enclosing),
		File:       idx.fset.Position(call.Pos()).Filename,
		Line:       idx.fset.Position(call.Pos()).Line,
		calleePkg:  fnObj.Pkg().Name(),
		calleeRecv: calleeReceiverName(fnObj),
	}
	idx.callSites[fnObj.Name()] = append(idx.callSites[fnObj.Name()], site)
}

func funcSymbol(fset *token.FileSet, pkgName, absPath string, d *ast.FuncDecl) Symbol {
	kind := "func"
	receiver := ""
	if d.Recv != nil && len(d.Recv.List) > 0 {
		kind = "method"
		receiver = formatRecv(fset, d.Recv.List[0].Type)
	}
	return Symbol{
		Name:      d.Name.Name,
		Kind:      kind,
		Signature: formatFuncSig(fset, d),
		Package:   pkgName,
		File:      absPath,
		Line:      fset.Position(d.Pos()).Line,
		Exported:  d.Name.IsExported(),
		Receiver:  receiver,
		startPos:  d.Pos(),
		endPos:    d.End(),
	}
}

func formatFuncSig(fset *token.FileSet, d *ast.FuncDecl) string {
	var buf bytes.Buffer
	buf.WriteString("func ")
	if d.Recv != nil && len(d.Recv.List) > 0 {
		buf.WriteString("(")
		printer.Fprint(&buf, fset, d.Recv.List[0].Type)
		buf.WriteString(") ")
	}
	buf.WriteString(d.Name.Name)
	// Print the FuncType without the leading "func" keyword — we already
	// wrote it. printer on ast.FuncType emits "func(...)..." so strip it.
	var ft bytes.Buffer
	printer.Fprint(&ft, fset, d.Type)
	s := ft.String()
	s = strings.TrimPrefix(s, "func")
	buf.WriteString(s)
	return buf.String()
}

func formatRecv(fset *token.FileSet, expr ast.Expr) string {
	var buf bytes.Buffer
	printer.Fprint(&buf, fset, expr)
	return strings.TrimPrefix(buf.String(), "*")
}

// calleeReceiverName returns the receiver type name for a method (empty for
// plain funcs). Strips pointer indirection and any type-parameter list, so
// *Greeter[T] → "Greeter".
func calleeReceiverName(fn *types.Func) string {
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return ""
	}
	t := sig.Recv().Type()
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	switch tt := t.(type) {
	case *types.Named:
		return tt.Obj().Name()
	case *types.Alias:
		return tt.Obj().Name()
	}
	return ""
}

func callerFuncName(pkgName string, fd *ast.FuncDecl) string {
	if fd == nil {
		return pkgName + ".<init>"
	}
	if fd.Recv != nil && len(fd.Recv.List) > 0 {
		var buf bytes.Buffer
		printer.Fprint(&buf, token.NewFileSet(), fd.Recv.List[0].Type)
		recv := strings.TrimPrefix(buf.String(), "*")
		return fmt.Sprintf("%s.%s.%s", pkgName, recv, fd.Name.Name)
	}
	return fmt.Sprintf("%s.%s", pkgName, fd.Name.Name)
}
