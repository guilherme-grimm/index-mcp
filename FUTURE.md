# Future — v1+ parking lot

Items deliberately deferred from v0. Do not build any of these until the v0 experiment result tells us which ones are actually worth it. The point of v0 is to learn whether the thesis holds; unprompted expansion defeats that.

## From `HANDOFF.md` (out-of-scope list)

- **File watching / `fsnotify`.** Rebuild incrementally on edits instead of at startup.
- **Change tracking / MRU.** Track which files the agent (or the user) has touched recently; weight or surface those in queries.
- **Impact scoring.** Given a change, report which first-party symbols depend on it — transitively.
- **LSP integration.** Speak LSP to a long-running gopls / other server instead of running `go/packages` ourselves. Trades startup cost for protocol complexity.
- **TypeScript / non-Go languages.** Each language needs its own indexer; the MCP surface can stay shared.
- **Multi-project support.** Today one server = one root. Could support multiple roots, or a workspace model.
- **Persistence.** SQLite or similar so restart doesn't re-pay the `go/packages` cold load.

## Raised during implementation

- **Rename `ctxindex`.** Module path is `github.com/guilherme-grimm/ctxindex`; the binary and tool name are both placeholders the user flagged as bad. Nothing external depends on it yet.
- **`read_symbol` on single-spec `type X struct {...}`** slices from the type name, not the `type` keyword. If agent usage finds this awkward, switch to slicing the enclosing `GenDecl` when it holds a single spec.
- **Function-value calls.** `f := pkg.Foo; f()` resolves to a `*types.Var`, not a `*types.Func`; currently skipped. Would need data-flow-lite to attribute.
- **Interface method calls expansion.** Currently attribute to the interface method. A future tool could return concrete implementations via `types.Implements` / `golang.org/x/tools/go/callgraph`.
- **Per-closure caller attribution.** Today, calls inside a nested `FuncLit` attribute to the outermost top-level `FuncDecl`. Fine for v0; could track closure identities if it matters.
- **Type aliases, embedded types, methods via embedding.** Deliberately not chased in v0. `go/types` can answer these; deferred to keep the v0 surface small.
- **Cross-package callee disambiguation in the wire format.** `CallersOf` filters qualified input internally, but the output JSON doesn't expose callee-package per site — so for a bare name spanning two packages, the agent can only disambiguate by `caller_func`. If that turns out to be a pain point, widen the JSON shape.
- **Shared qualifier parser.** `ReadSymbol` and `CallersOf` both split on `.` to parse `pkg.Name` / `pkg.Recv.Method`. Two call sites is not a pattern; if a third appears, factor out.
- **Package-name collisions.** Two first-party packages sharing a short name (`foo/v1/thing` + `foo/v2/thing` both `package thing`) will collide under `Symbol.Package`. Acceptable for v0; a path-qualified form (`foo/v2/thing.Foo`) would resolve it.

## Speculative

- **Doc comment tool.** `doc_of(symbol)` returning the godoc — cheap extension.
- **Type-of-expression tool.** Given a file/line, return the inferred type. LSP-like.
- **Graph export.** Dump the call graph as a format consumable by visualization tools.
- **Session metrics.** Log which tools the agent actually used per session — direct signal for the v0/v1 decision.
