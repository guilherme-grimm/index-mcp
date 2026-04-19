# Context Index MCP Server — v0 Handoff

## What this is

A Go MCP server that exposes a codebase's **semantic structure** (symbols, call graphs, exports) as queryable tools, so LLM agents can ask structured questions *instead of reading full files*. The goal is to reduce token usage and context rot in long coding sessions by replacing `read_file` calls with targeted queries like `callers_of("useAuth")` or `exports_of("handler.go")`.

This is v0 — a falsifiable prototype to test whether the core thesis holds before investing further. Build it lean. No file watching, no LSP, no change tracking yet. Just symbol queries.

## Thesis to validate

**Hypothesis:** An agent given structural query tools over a Go codebase can implement a non-trivial feature while reading significantly fewer full files (and therefore using significantly fewer tokens) than an agent using only `read_file` / `grep` / `ls`.

**Success criteria for v0:**
- Running a representative task on `graph-go` with the tool vs. without shows ≥30% reduction in tokens spent on code reading.
- The agent naturally reaches for the structural tools when they're available (doesn't fall back to `read_file` constantly).
- Qualitatively: the final code quality is at least as good as the control run.

**If v0 fails:** the thesis is weaker than expected and the full vision (file watching, MRU, impact scoring, TS support) should be reconsidered, not just built on top.

## Scope — what to build

### In scope for v0
- Go-only analysis, using `golang.org/x/tools/go/packages` and `go/ast`
- MCP server (stdio transport is fine) with exactly four tools:
  - `symbols_in(path)` → list of top-level symbols in the file, with kind (func/type/var/const) and signature. No bodies.
  - `exports_of(path)` → subset of `symbols_in` filtered to exported (capitalized) symbols.
  - `callers_of(symbol)` → list of `{path, line, calling_function}` for every call site of the symbol in the project.
  - `read_symbol(name)` → returns the full source of just that symbol (function body, type decl, etc.), not the whole file.
- Single project scope: server is started with a project root path and only analyzes packages under it.
- In-memory index built at startup. Acceptable to rebuild on every server start — no incremental updates yet.
- Basic caching: parse the package set once, reuse across tool calls.

### Explicitly out of scope for v0
- File watching / fsnotify
- Change tracking / MRU / impact scoring
- LSP integration
- TypeScript / any non-Go language
- Multi-project support
- Incremental re-indexing on file edits
- Persistence (SQLite, etc.) — everything in memory
- Any UI

Do not add these. If they seem tempting, note them in a `FUTURE.md` and move on.

## Architecture — suggested shape

```
context-index/
├── cmd/
│   └── ctxindex/
│       └── main.go          # MCP server entry, stdio transport
├── internal/
│   ├── index/
│   │   ├── index.go         # Builds and holds the symbol index
│   │   ├── loader.go        # go/packages loading
│   │   └── query.go         # Query methods backing the MCP tools
│   └── mcp/
│       └── server.go        # MCP tool registration + handlers
├── go.mod
├── README.md
└── FUTURE.md                # Parking lot for v1+ ideas
```

### Data model

```go
type Symbol struct {
    Name       string
    Kind       string   // "func", "type", "var", "const", "method"
    Signature  string   // for funcs/methods; empty otherwise
    Package    string
    File       string   // absolute path
    Line       int
    Exported   bool
    Receiver   string   // for methods, empty for funcs
}

type CallSite struct {
    CallerFunc string   // fully qualified: pkg.Func or pkg.Type.Method
    File       string
    Line       int
}

type Index struct {
    symbols    map[string][]Symbol       // key: unqualified name (many symbols may share a name across packages)
    byFile     map[string][]Symbol       // key: absolute file path
    callSites  map[string][]CallSite     // key: symbol name
    fset       *token.FileSet
}
```

Key implementation note: `go/packages` with `packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo` gives you enough to walk ASTs and resolve call targets. Use `types.Info.Uses` to map identifier references to their definitions — that's how you build `callSites` correctly (not by string matching).

### MCP tool shapes

Return structured JSON, not prose. Keep payloads tight — the whole point is token efficiency.

```
symbols_in(path: string) → {
  symbols: [
    { name, kind, signature, line, exported }
  ]
}

exports_of(path: string) → same shape, filtered

callers_of(symbol: string) → {
  symbol,
  sites: [
    { caller_func, file, line }
  ]
}

read_symbol(name: string) → {
  name, kind, file, line, source
}
```

For `callers_of` and `read_symbol`, if the name is ambiguous (multiple symbols with the same unqualified name across packages), return all matches and let the caller disambiguate. Don't guess.

## Dependencies

```
golang.org/x/tools/go/packages
go/ast, go/token, go/types (stdlib)
```

For the MCP server, use the official Go SDK if one exists and is stable; otherwise implement the stdio JSON-RPC loop directly — it's not much code. Check [modelcontextprotocol.io](https://modelcontextprotocol.io) for the current Go SDK status before picking.

## Testing strategy for v0

1. **Unit tests** on the index against a small fixture package in `testdata/`.
2. **Integration test** against `graph-go` itself: start the server pointed at `graph-go`'s root, hit each tool, assert sensible results (e.g., `callers_of("BuildGraph")` returns known call sites).
3. **The real test** — run Claude Code on `graph-go` with and without the MCP server, on a task like "add a new output format for the graph." Compare:
   - Total input tokens (via `/cost` or `ccusage`)
   - Number of `read_file` calls made
   - Subjective code quality of the result
   
   Log both sessions. Write up results in `EXPERIMENT.md`. This is the most important deliverable — without it, the thesis is unvalidated.

## Watch-outs

- **`go/packages` is slow on cold loads.** Expect 2-10s startup on a medium project. That's fine for v0 but will matter later.
- **Call resolution needs `types.Info`, not AST alone.** Don't shortcut with identifier string matching — it'll give wrong results for shadowed names, method calls on interfaces, etc.
- **Generics** — if the target project uses generics, test that symbol resolution handles instantiations. Recent `go/packages` versions handle this, but verify.
- **Don't return bodies in `symbols_in` / `exports_of`.** That defeats the purpose. Only `read_symbol` returns source.
- **Keep the MCP server stateless across tool calls** except for the index itself. No session state.

## Cost tracking — what to use

Not part of the build, but for the session where you test v0, have this set up ahead of time:

1. **`/cost` in Claude Code** — built in, use mid-session for quick checks.
2. **`ccusage`** — `npm install -g ccusage` then `ccusage` for daily breakdowns, `ccusage --since 2026-04-19` to scope to your experiment window. This reads `~/.claude/` logs and gives token + dollar totals per session. Use this for the experiment comparison.
3. Log the session IDs of the two test runs (control vs. tool-enabled) so you can isolate them in `ccusage` output.

## Definition of done for v0

- [ ] Four MCP tools working against `graph-go`
- [ ] Unit tests + integration test green
- [ ] Server registerable in Claude Code's MCP config
- [ ] `EXPERIMENT.md` written with control vs. tool-enabled comparison on one real task
- [ ] `FUTURE.md` with the parking-lot items (file watching, MRU, impact, TS, etc.)
- [ ] Honest conclusion in the README: does the thesis hold, partially hold, or fail? What's the next bet?

If it holds: v1 scopes file watching + MRU + impact scoring. If it partially holds: figure out what the agent actually reached for vs. ignored, and build the next version around that. If it fails: write up why, commit to the repo, move on. All three outcomes are wins — you'll know more than you do now.

## Session context for the implementing agent

This project came out of a design conversation on context efficiency for LLM coding agents. The core insight was that long agentic sessions burn tokens on re-reading files the agent already has a mental model of, and that exposing structural queries (via LSP-style semantic analysis) could let agents replace reads with targeted lookups. v0 is a focused test of that insight on a single language (Go) with a minimal tool surface. Resist scope creep — the point is to learn whether the core idea is worth building out, not to ship a finished product.
