# Progress Log

Retrospective journal for v0. Phases 1–4 built and verified; Phase 5 landed docs (`README.md`, `FUTURE.md`) but deferred the `graph-go` integration test and coverage audit — the v0 experiment will exercise the server against a real project anyway. Source of truth for *what to build* is `HANDOFF.md`; forward-looking items live in `FUTURE.md`.

## What was built

**Phase 1** — Skeleton + MCP server with all four tools stubbed. `go.mod` at `github.com/guilherme-grimm/ctxindex` (Go 1.26.2); binary builds to `./bin/ctxindex`. SDK: `github.com/modelcontextprotocol/go-sdk` v1.5.0.

**Phase 2** — Index build via `golang.org/x/tools/go/packages` (modes: `NeedName|NeedFiles|NeedSyntax|NeedTypes|NeedTypesInfo|NeedCompiledGoFiles|NeedImports|NeedDeps`). Top-level decl walk produces `Symbol{Name, Kind, Signature, Package, File, Line, Exported, Receiver}`. Call resolution via `pkg.TypesInfo.Uses[ident]` → `*types.Func`, scoped to first-party packages (`firstParty` keyed by `pkg.PkgPath`). Signatures formatted via `go/printer` on `*ast.FuncType` with `"func "` stitched on the front. Fixture at `testdata/fixture/` — two packages exercising exported/unexported, methods, cross-package, and a generic.

**Phase 3** — `SymbolsIn` / `ExportsOf` / `ReadSymbol` live; `pathutil.Resolve` (uses `filepath.Rel` + `..` check for containment). `Symbol` gained unexported `startPos`/`endPos` for source slicing — JSON shape preserved. `ReadSymbol` returns flat object for 1 match, `{matches:[...]}` for >1, tool error for 0. Fixture: unexported `helper()` added to `app/` to create intentional ambiguity with `greet.helper`.

**Phase 4** — `callers_of` live. `CallSite` gained unexported `calleePkg`/`calleeRecv` (same precedent as Phase 3 on `Symbol`) so qualified input (`pkg.Foo`, `pkg.Recv.Method`) can filter without widening the JSON shape. `calleeReceiverName(*types.Func)` derives the receiver through `types.Type`, naturally dropping generic instantiations via `Named.Obj().Name()`.

**Phase 5** — `README.md` (install, `.mcp.json`, tool reference with example outputs, scope/limits, thesis status). `FUTURE.md` (HANDOFF parking-lot + items raised during phases 1–4). Skipped for now: coverage audit for generics/interface-method cases, `graph-go` stdio integration test.

Final state: 27 tests green, `indexed 2 packages, 10 symbols, 6 call sites` on the fixture, all four MCP tools returning HANDOFF-shaped JSON.

## Watch-outs worth remembering

- **Piping into the binary closes stdin → server flushes nothing.** Tests and smoke scripts need to keep stdin open until the expected responses land. The fix in every smoke script so far: `(printf ...; sleep 1) | ./bin/ctxindex ...`. Any Phase-5-style integration test driving the server programmatically will hit this.
- **`go/packages` cold load is 2–10s on a medium project.** Only 300ms on the fixture; budget accordingly when running against real repos.
- **`ast.Inspect` + FuncDecl stack doesn't cleanly pop on post-order.** The build.go workaround nests a second `ast.Inspect` into `FuncDecl.Body` and returns `false` from the outer traversal to avoid double-visiting. Consequence: closures (nested `FuncLit`) attribute to the outermost top-level `FuncDecl`, not per-closure. Fine for v0; flagged in `FUTURE.md` if it starts to matter.
- **Interface method calls attribute to the interface's `*types.Func`,** not any concrete impl. Intentional; documented in `build.go`. Expansion to impls is a v1 consideration.
- **Function-value calls (`f := pkg.Foo; f()`) are dropped** — `f` resolves to `*types.Var`. Same parking-lot.
- **`Package` on symbols is the short name (`greet`), not the import path.** `fnObj.Pkg().Name()` in `recordCallSite` mirrors this. Two first-party packages sharing a short name will collide silently. Acceptable for v0, same failure mode as `ReadSymbol`.
- **`read_symbol` on a single-spec `type X struct{...}`** slices from the type name, not the `type` keyword (we slice the `TypeSpec`, not its enclosing `GenDecl`). Tests assert `Contains("Greeter struct")` rather than `HasPrefix("type ")`.
- **`go mod tidy` is not optional** after adding `golang.org/x/tools/go/packages`. Transitive deps (`golang.org/x/sync/errgroup`, `golang.org/x/mod/semver`) don't come in via bare `go get`.

## Shaping invariants

- **HANDOFF.md is authority** for behavior and JSON shapes. When implementation tempts a shape extension, prefer unexported fields on internal types — the `Symbol.startPos`/`endPos` pattern (Phase 3) and `CallSite.calleePkg`/`calleeRecv` (Phase 4) both followed this rule.
- **Call resolution is always `types.Info.Uses`, never string matching.** Non-negotiable per HANDOFF watch-outs.
- **Unknown-input ergonomics.** Unknown file path in `SymbolsIn`, unknown symbol in `ReadSymbol`, zero-caller symbol in `CallersOf` → all return empty results, not errors. Malformed qualifier (4+ parts, empty input) is an error. Fresh implementers: don't "helpfully" turn empty results into errors.
- **Fixture fit for purpose.** `greet.helper` is called, `app.helper` is uncalled — chosen so `CallersOf("helper")` exercises the ambiguous-bare-name case while `CallersOf("app.helper")` exercises the empty-match-under-qualifier case. Don't edit the fixture without checking which tests anchor on that asymmetry.

## What's next

The experiment (run Claude Code on a real project with and without the server, compare token spend and `read_file` count, write up in `EXPERIMENT.md`) is the deliverable that decides whether v0 holds. Separate session, per HANDOFF. Everything else — file watching, MRU, impact scoring, TS, persistence — waits on that result.
