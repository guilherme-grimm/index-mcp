package index

import (
	"fmt"
	"os"

	"golang.org/x/tools/go/packages"
)

const loadMode = packages.NeedName |
	packages.NeedFiles |
	packages.NeedSyntax |
	packages.NeedTypes |
	packages.NeedTypesInfo |
	packages.NeedCompiledGoFiles |
	packages.NeedImports |
	packages.NeedDeps

func loadPackages(root string) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Mode:  loadMode,
		Dir:   root,
		Tests: false,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("packages.Load: %w", err)
	}
	for _, p := range pkgs {
		for _, e := range p.Errors {
			fmt.Fprintf(os.Stderr, "package %s: %s\n", p.PkgPath, e)
		}
	}
	return pkgs, nil
}
