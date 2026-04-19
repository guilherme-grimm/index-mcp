package index

import (
	"path/filepath"
	"strings"
	"testing"
)

func buildFixture(t *testing.T) *Index {
	t.Helper()
	root, err := filepath.Abs("../../testdata/fixture")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	idx, err := Build(root)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	return idx
}

func fixturePath(t *testing.T, rel string) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("../../testdata/fixture", rel))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	return abs
}

func TestSymbolsIn_Greet(t *testing.T) {
	idx := buildFixture(t)
	got := idx.SymbolsIn(fixturePath(t, "greet/greet.go"))
	if len(got) != 5 {
		t.Fatalf("want 5 symbols, got %d: %+v", len(got), got)
	}
	want := map[string]string{
		"Greeter":    "type",
		"NewGreeter": "func",
		"Hello":      "method",
		"helper":     "func",
		"Map":        "func",
	}
	for _, s := range got {
		k, ok := want[s.Name]
		if !ok {
			t.Errorf("unexpected symbol %q", s.Name)
			continue
		}
		if s.Kind != k {
			t.Errorf("%s kind=%s want %s", s.Name, s.Kind, k)
		}
	}
}

func TestSymbolsIn_Unknown(t *testing.T) {
	idx := buildFixture(t)
	got := idx.SymbolsIn("/nonexistent/file.go")
	if len(got) != 0 {
		t.Fatalf("want empty, got %d", len(got))
	}
}

func TestExportsOf_Greet(t *testing.T) {
	idx := buildFixture(t)
	got := idx.ExportsOf(fixturePath(t, "greet/greet.go"))
	names := map[string]bool{}
	for _, s := range got {
		names[s.Name] = true
	}
	want := []string{"Greeter", "NewGreeter", "Hello", "Map"}
	if len(got) != len(want) {
		t.Fatalf("want %d, got %d: %v", len(want), len(got), names)
	}
	for _, n := range want {
		if !names[n] {
			t.Errorf("missing %q", n)
		}
	}
}

func TestReadSymbol_Func(t *testing.T) {
	idx := buildFixture(t)
	got, err := idx.ReadSymbol("NewGreeter")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	src := got[0].Source
	if !strings.HasPrefix(src, "func NewGreeter") {
		t.Errorf("source does not start with func NewGreeter: %q", src)
	}
	if !strings.HasSuffix(strings.TrimSpace(src), "}") {
		t.Errorf("source does not end with }: %q", src)
	}
}

func TestReadSymbol_Type(t *testing.T) {
	idx := buildFixture(t)
	got, err := idx.ReadSymbol("Greeter")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	if got[0].Kind != "type" {
		t.Errorf("kind=%s", got[0].Kind)
	}
	if !strings.Contains(got[0].Source, "Greeter struct") {
		t.Errorf("source missing struct decl: %q", got[0].Source)
	}
}

func TestReadSymbol_Ambiguous(t *testing.T) {
	idx := buildFixture(t)
	got, err := idx.ReadSymbol("helper")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 matches, got %d", len(got))
	}
	pkgs := map[string]bool{}
	for _, s := range got {
		pkgs[s.Package] = true
	}
	if !pkgs["greet"] || !pkgs["main"] {
		t.Errorf("expected greet+main packages, got %v", pkgs)
	}
}

func TestReadSymbol_QualifiedPkg(t *testing.T) {
	idx := buildFixture(t)
	got, err := idx.ReadSymbol("greet.helper")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 || got[0].Package != "greet" {
		t.Fatalf("want 1 greet.helper, got %+v", got)
	}
}

func TestReadSymbol_QualifiedMethod(t *testing.T) {
	idx := buildFixture(t)
	got, err := idx.ReadSymbol("greet.Greeter.Hello")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	if got[0].Receiver != "Greeter" || got[0].Kind != "method" {
		t.Errorf("receiver=%q kind=%q", got[0].Receiver, got[0].Kind)
	}
}

func TestReadSymbol_InvalidQualifier(t *testing.T) {
	idx := buildFixture(t)
	_, err := idx.ReadSymbol("a.b.c.d")
	if err == nil {
		t.Fatal("want error for 4-part name")
	}
}

func TestReadSymbol_NotFound(t *testing.T) {
	idx := buildFixture(t)
	got, err := idx.ReadSymbol("DoesNotExist")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty, got %d", len(got))
	}
}

func oneSite(t *testing.T, idx *Index, input, wantCaller string) CallSite {
	t.Helper()
	sym, sites, err := idx.CallersOf(input)
	if err != nil {
		t.Fatalf("CallersOf(%q): %v", input, err)
	}
	if sym != input {
		t.Errorf("symbol echo: got %q want %q", sym, input)
	}
	if len(sites) != 1 {
		t.Fatalf("CallersOf(%q): want 1 site, got %d: %+v", input, len(sites), sites)
	}
	if sites[0].CallerFunc != wantCaller {
		t.Errorf("CallersOf(%q): caller=%q want %q", input, sites[0].CallerFunc, wantCaller)
	}
	return sites[0]
}

func TestCallersOf_NewGreeter(t *testing.T) {
	idx := buildFixture(t)
	s := oneSite(t, idx, "NewGreeter", "main.main")
	if !strings.HasSuffix(s.File, "app/main.go") {
		t.Errorf("unexpected file %q", s.File)
	}
}

func TestCallersOf_Hello(t *testing.T) {
	idx := buildFixture(t)
	oneSite(t, idx, "Hello", "main.main")
}

func TestCallersOf_Map(t *testing.T) {
	idx := buildFixture(t)
	oneSite(t, idx, "Map", "main.run")
}

func TestCallersOf_Util(t *testing.T) {
	idx := buildFixture(t)
	oneSite(t, idx, "util", "main.run")
}

func TestCallersOf_HelperBare(t *testing.T) {
	idx := buildFixture(t)
	// greet.helper is called once (by Greeter.Hello); app.helper is uncalled.
	oneSite(t, idx, "helper", "greet.Greeter.Hello")
}

func TestCallersOf_QualifiedPkg(t *testing.T) {
	idx := buildFixture(t)
	oneSite(t, idx, "greet.helper", "greet.Greeter.Hello")
}

func TestCallersOf_QualifiedPkgNoMatch(t *testing.T) {
	idx := buildFixture(t)
	_, sites, err := idx.CallersOf("app.helper")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(sites) != 0 {
		t.Fatalf("want 0 sites, got %d: %+v", len(sites), sites)
	}
}

func TestCallersOf_QualifiedMethod(t *testing.T) {
	idx := buildFixture(t)
	oneSite(t, idx, "greet.Greeter.Hello", "main.main")
}

func TestCallersOf_Nonexistent(t *testing.T) {
	idx := buildFixture(t)
	_, sites, err := idx.CallersOf("Nonexistent")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(sites) != 0 {
		t.Fatalf("want 0, got %d", len(sites))
	}
}

func TestCallersOf_InvalidQualifier(t *testing.T) {
	idx := buildFixture(t)
	if _, _, err := idx.CallersOf("a.b.c.d"); err == nil {
		t.Fatal("want error for 4-part name")
	}
}

func TestCallersOf_Empty(t *testing.T) {
	idx := buildFixture(t)
	if _, _, err := idx.CallersOf(""); err == nil {
		t.Fatal("want error for empty")
	}
}
