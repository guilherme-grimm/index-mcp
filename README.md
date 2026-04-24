# ctxindex

**A structural index for Go, exposed to LLM agents over MCP.** Four tools let an agent ask *"what symbols are in this file?"*, *"who calls this function?"*, *"show me just this one symbol"* — instead of reading whole files it already has a mental model of.

The thesis: long coding sessions burn tokens re-reading files. Structural queries can replace many of those reads.

## Status: v0.1 — thesis validated

First measured task (small implementation on a real Go project):

| Metric         | With ctxindex | Without   | Delta   |
| -------------- | ------------- | --------- | ------- |
| Claude API cost | **$4.65**    | $5.50     | **−15%** |
| Wall time      | **~15 min**   | >20 min   | **−25%** |

Qualitative: time spent on file-reading dropped noticeably; the agent reached for `read_symbol` and `callers_of` in place of whole-file reads. Single data point — not a benchmark, but enough to keep going.

v0.1 locks in the four-tool surface. v1 scopes file watching, MRU, and impact scoring.

## Tools

All tools return compact structured JSON (no file bodies in `symbols_in` / `exports_of`).

### `symbols_in(path)`

Top-level symbols in a file with kind and signature. No bodies.

```json
{
  "symbols": [
    { "name": "main", "kind": "func", "signature": "func main()", "line": 9, "exported": false },
    { "name": "run",  "kind": "func", "signature": "func run()",  "line": 15, "exported": false }
  ]
}
```

`kind` is one of `func`, `method`, `type`, `var`, `const`. `path` can be absolute (must be under the server's `--root`) or root-relative.

### `exports_of(path)`

Same shape as `symbols_in`, filtered to exported (capitalized) symbols.

### `callers_of(symbol)`

Call sites of a symbol across the indexed project.

```json
{
  "symbol": "Hello",
  "sites": [
    { "caller_func": "main.main", "file": "/abs/path/app/main.go", "line": 11 }
  ]
}
```

`symbol` forms: `Foo` (bare), `pkg.Foo`, or `pkg.Receiver.Method`. A bare name that's ambiguous across packages returns all sites merged — the agent disambiguates via `caller_func` + `file`. Call resolution uses `go/types` (not string matching), so interface method calls attribute to the interface method, and shadowed names resolve correctly.

### `read_symbol(name)`

The full source of one top-level symbol — function body, type decl, etc. Not the whole file.

```json
{
  "name": "NewGreeter",
  "kind": "func",
  "file": "/abs/path/greet/greet.go",
  "line": 9,
  "source": "func NewGreeter(name string) *Greeter {\n\treturn &Greeter{Name: name}\n}"
}
```

`name` forms match `callers_of`. When ambiguous, returns `{ "matches": [ ... ] }` with one entry per match. Unknown names return a tool error.

## Install

```
go build -o ./bin/ctxindex ./cmd/ctxindex
```

Binary writes logs to stderr, speaks MCP over stdio on stdout.

## Configure in Claude Code

Add to your `.mcp.json` (project-local) or `~/.claude.json` (global):

```json
{
  "mcpServers": {
    "ctxindex": {
      "command": "/absolute/path/to/bin/ctxindex",
      "args": ["--root", "/absolute/path/to/your/go/project"]
    }
  }
}
```

`--root` defaults to the server's working directory if omitted. Only Go packages under that root are indexed; stdlib and module dependencies are not.

## Startup

On start, `ctxindex` loads every package under `--root` via `go/packages` and builds the index in memory. Expect 2–10s cold-load on a medium project. Stderr log:

```
ctxindex 0.1.0 starting (root=/abs/path)
indexed 42 packages, 1873 symbols, 4921 call sites in 3.2s
```

No incremental updates — restart the server to pick up edits. (v1 territory; see `FUTURE.md`.)

## Scope

Go only. In-memory. Rebuilt on every start. Deliberately out of scope for v0.1:

- File watching / incremental re-indexing
- Change tracking, MRU, impact scoring
- LSP integration
- Non-Go languages
- Multi-project support
- Persistence

Known partial cases:

- Function-value calls (`f := pkg.Foo; f()`) aren't attributed — the callee resolves to a `*types.Var`, not a `*types.Func`.
- Interface method calls attribute to the interface method, not concrete implementations. Intentional; a future version could optionally expand to impls.
- Top-level calls outside any function (e.g. `var x = f()`) attribute to `pkg.<init>`.
- Closures attribute to the enclosing top-level `FuncDecl`, not per-closure.

## Testing

```
go test ./...
```

Unit tests run against `testdata/fixture/` — a two-package Go module exercising exported/unexported funcs, a method on a type, a cross-package call, and a generic function.

## Repo layout

```
cmd/ctxindex/main.go     # flag parsing, index build, MCP server start
internal/
  index/                 # packages load, AST walk, symbol + call-site index, queries
  mcp/                   # MCP tool registration, JSON output shapes
  pathutil/              # absolute/relative path resolution with root containment
testdata/fixture/        # small Go module for unit tests
HANDOFF.md               # design rationale, scope boundaries, success criteria
PROGRESS.md              # build journal
FUTURE.md                # v1+ parking lot
```
