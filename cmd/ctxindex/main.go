package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/guilherme-grimm/ctxindex/internal/index"
	"github.com/guilherme-grimm/ctxindex/internal/mcp"
)

const version = "0.1.0"

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("getwd: %v", err)
	}
	root := flag.String("root", cwd, "absolute path to the Go project root to index")
	flag.Parse()

	abs, err := filepath.Abs(*root)
	if err != nil {
		log.Fatalf("resolve --root: %v", err)
	}
	abs = filepath.Clean(abs)
	info, err := os.Stat(abs)
	if err != nil {
		log.Fatalf("stat --root %q: %v", abs, err)
	}
	if !info.IsDir() {
		log.Fatalf("--root %q is not a directory", abs)
	}

	fmt.Fprintf(os.Stderr, "ctxindex %s starting (root=%s)\n", version, abs)

	start := time.Now()
	idx, err := index.Build(abs)
	if err != nil {
		log.Fatalf("build index: %v", err)
	}
	pkgs, syms, sites := idx.Stats()
	fmt.Fprintf(os.Stderr, "indexed %d packages, %d symbols, %d call sites in %s\n",
		pkgs, syms, sites, time.Since(start).Round(time.Millisecond))

	srv := mcp.NewServer("ctxindex", version, abs, idx)
	if err := srv.Run(context.Background(), &mcpsdk.StdioTransport{}); err != nil {
		log.Fatalf("mcp server: %v", err)
	}
}
