# sichek-collector Application Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a standalone in-cluster HTTP service (`sichek-collector`) that receives the latest `snapshot.json` from each GPU node via POST and persists one file per node. Optional bulk GET returns NDJSON of all nodes' snapshots.

**Architecture:** Single-binary Go application in its own repo. `net/http` server with two handlers (POST/GET) backed by a filesystem `Store`. One file per node, atomic tmp+rename writes, latest-only (no archival). Collector is agnostic to snapshot body contents — stores raw bytes; node identity comes from `X-Sichek-Node` header. Deployed as 1-replica Deployment + ClusterIP Service + 2GB PVC.

**Tech Stack:** Go 1.22+, `net/http` stdlib, stdlib `compress/gzip`, stdlib `encoding/json`, `testing` stdlib, Docker multi-stage build, plain K8s YAML.

**Spec reference:** `docs/superpowers/specs/2026-04-23-sichek-collector-design.md`

---

## File Structure (new repo `sichek-collector/`)

```
sichek-collector/
├── cmd/
│   └── sichek-collector/
│       └── main.go                       # entry: load env, start server
├── internal/
│   ├── config/
│   │   ├── config.go                     # Config struct + LoadFromEnv
│   │   └── config_test.go
│   ├── store/
│   │   ├── store.go                      # Store interface
│   │   ├── fs.go                         # FSStore implementation
│   │   └── fs_test.go
│   └── server/
│       ├── server.go                     # HTTP server bootstrap + router
│       ├── snapshot.go                   # POST/GET /api/v1/snapshots handlers
│       ├── snapshot_test.go
│       ├── health.go                     # GET /healthz
│       └── middleware.go                 # gzip decode, body size limit, auth
├── deploy/
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── pvc.yaml
│   └── ingress.yaml                      # optional, for external GET
├── Dockerfile
├── Makefile
├── .gitignore
├── .dockerignore
├── go.mod
├── go.sum
└── README.md
```

**Responsibilities:**
- `config` — Single source of all runtime config; parsed once at startup.
- `store` — Abstracts "write latest snapshot for node X" and "list all snapshots"; filesystem is the only implementation now.
- `server` — HTTP routing, header validation, delegates to `store`.
- `cmd/sichek-collector/main.go` — Wire everything, handle signals.

---

## Prerequisites

Assumed available on developer machine:
- Go 1.22+
- Docker
- `git`
- Network access for `go get` (stdlib-only, so no third-party deps needed)

**Assumed decisions already made (see spec):**
- Repo is **new**, independent from the sichek repo. No shared Go module.
- Port **38080**.
- No Prometheus `/metrics` endpoint.

---

## Task 1: Initialize repository scaffold

**Files:**
- Create: `sichek-collector/go.mod`
- Create: `sichek-collector/.gitignore`
- Create: `sichek-collector/README.md`
- Create: `sichek-collector/Makefile`

- [ ] **Step 1: Create repo directory and init git**

Pick a parent directory (e.g., `~/devnet/`). Run:
```bash
mkdir -p ~/devnet/sichek-collector && cd ~/devnet/sichek-collector
git init -b main
```

- [ ] **Step 2: Create `go.mod`**

```bash
go mod init github.com/scitix/sichek-collector
```

This creates a `go.mod` with `module github.com/scitix/sichek-collector` and `go 1.22`. No third-party deps yet.

- [ ] **Step 3: Create `.gitignore`**

Write `.gitignore`:
```
# Binaries
/bin/
*.exe
sichek-collector

# Go
vendor/
*.test
*.out
coverage.txt

# IDE
.idea/
.vscode/
*.swp

# Local
.env
/tmp/
```

- [ ] **Step 4: Create `Makefile`**

Write `Makefile`:
```makefile
.PHONY: build test lint fmt vet docker clean

BINARY := sichek-collector
PKG := ./...
IMAGE ?= sichek-collector
TAG ?= dev

build:
	go build -o bin/$(BINARY) ./cmd/sichek-collector

test:
	go test -race -count=1 -coverprofile=coverage.txt $(PKG)

vet:
	go vet $(PKG)

fmt:
	gofmt -s -w .

lint: fmt vet

docker:
	docker build -t $(IMAGE):$(TAG) .

clean:
	rm -rf bin/ coverage.txt
```

- [ ] **Step 5: Create minimal `README.md`**

Write `README.md`:
```markdown
# sichek-collector

In-cluster HTTP service that receives the latest `snapshot.json` from each
sichek-instrumented GPU node and stores one file per node.

See design: https://github.com/scitix/sichek/blob/main/docs/superpowers/specs/2026-04-23-sichek-collector-design.md

## Build
    make build

## Test
    make test

## Run locally
    DATA_DIR=./tmp LISTEN_ADDR=:38080 ./bin/sichek-collector
```

- [ ] **Step 6: Commit**

```bash
git add .
git commit -m "chore: init repo scaffold"
```

---

## Task 2: Config package — Load env vars into a typed struct

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test `config_test.go`**

```go
package config

import (
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	// All env vars unset → defaults apply
	t.Setenv("LISTEN_ADDR", "")
	t.Setenv("DATA_DIR", "")
	t.Setenv("ENABLE_READ_API", "")
	t.Setenv("MAX_BODY_SIZE", "")
	t.Setenv("READ_TOKEN", "")
	t.Setenv("LOG_LEVEL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() err=%v", err)
	}
	if cfg.ListenAddr != ":38080" {
		t.Errorf("ListenAddr=%q want :38080", cfg.ListenAddr)
	}
	if cfg.DataDir != "/data" {
		t.Errorf("DataDir=%q want /data", cfg.DataDir)
	}
	if cfg.EnableReadAPI != false {
		t.Errorf("EnableReadAPI=%v want false", cfg.EnableReadAPI)
	}
	if cfg.MaxBodySize != 10*1024*1024 {
		t.Errorf("MaxBodySize=%d want 10MB", cfg.MaxBodySize)
	}
	if cfg.ReadToken != "" {
		t.Errorf("ReadToken=%q want empty", cfg.ReadToken)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel=%q want info", cfg.LogLevel)
	}
	if cfg.ShutdownGrace != 10*time.Second {
		t.Errorf("ShutdownGrace=%v want 10s", cfg.ShutdownGrace)
	}
}

func TestLoad_Overrides(t *testing.T) {
	t.Setenv("LISTEN_ADDR", ":9999")
	t.Setenv("DATA_DIR", "/tmp/foo")
	t.Setenv("ENABLE_READ_API", "true")
	t.Setenv("MAX_BODY_SIZE", "1048576")
	t.Setenv("READ_TOKEN", "secret")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() err=%v", err)
	}
	if cfg.ListenAddr != ":9999" {
		t.Errorf("ListenAddr=%q", cfg.ListenAddr)
	}
	if cfg.DataDir != "/tmp/foo" {
		t.Errorf("DataDir=%q", cfg.DataDir)
	}
	if !cfg.EnableReadAPI {
		t.Errorf("EnableReadAPI want true")
	}
	if cfg.MaxBodySize != 1048576 {
		t.Errorf("MaxBodySize=%d", cfg.MaxBodySize)
	}
	if cfg.ReadToken != "secret" {
		t.Errorf("ReadToken=%q", cfg.ReadToken)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel=%q", cfg.LogLevel)
	}
}

func TestLoad_InvalidMaxBodySize(t *testing.T) {
	t.Setenv("MAX_BODY_SIZE", "not-a-number")
	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error, got nil")
	}
}
```

- [ ] **Step 2: Run and verify failure**

```bash
cd ~/devnet/sichek-collector
go test ./internal/config/...
```
Expected: FAIL — `config.Load` undefined.

- [ ] **Step 3: Implement `config.go`**

Write `internal/config/config.go`:
```go
// Package config loads runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	ListenAddr    string
	DataDir       string
	EnableReadAPI bool
	MaxBodySize   int64
	ReadToken     string
	LogLevel      string
	ShutdownGrace time.Duration
}

func Load() (*Config, error) {
	c := &Config{
		ListenAddr:    envStr("LISTEN_ADDR", ":38080"),
		DataDir:       envStr("DATA_DIR", "/data"),
		EnableReadAPI: envBool("ENABLE_READ_API", false),
		ReadToken:     envStr("READ_TOKEN", ""),
		LogLevel:      envStr("LOG_LEVEL", "info"),
		ShutdownGrace: 10 * time.Second,
	}

	mbs, err := envInt64("MAX_BODY_SIZE", 10*1024*1024)
	if err != nil {
		return nil, fmt.Errorf("MAX_BODY_SIZE: %w", err)
	}
	c.MaxBodySize = mbs
	return c, nil
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func envInt64(key string, def int64) (int64, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, err
	}
	return n, nil
}
```

- [ ] **Step 4: Run and verify pass**

```bash
go test ./internal/config/... -v
```
Expected: PASS — all three tests.

- [ ] **Step 5: Commit**

```bash
git add internal/config
git commit -m "feat(config): load runtime config from env"
```

---

## Task 3: Store package — `Store` interface

**Files:**
- Create: `internal/store/store.go`

- [ ] **Step 1: Write `store.go`**

```go
// Package store defines how collector persists per-node snapshots.
package store

import (
	"context"
	"errors"
	"io"
)

// ErrInvalidNode is returned when the node name contains unsafe characters.
var ErrInvalidNode = errors.New("invalid node name")

// Entry represents one persisted snapshot.
type Entry struct {
	Node string        // node identity (filename stem)
	Body io.ReadCloser // raw bytes as stored; caller must Close
}

// Store persists one latest snapshot per node.
//
// Semantics:
//   - Put(node, body) writes atomically (tmp + rename). Successive Puts overwrite.
//   - List returns iterator over all nodes currently stored, in undefined order.
//
// Implementations must be safe for concurrent calls with different nodes.
// Concurrent Puts to the same node resolve to "last writer wins" without
// corrupting the file.
type Store interface {
	Put(ctx context.Context, node string, body io.Reader) error
	List(ctx context.Context) (Iterator, error)
}

// Iterator yields entries one at a time. Close releases underlying resources.
type Iterator interface {
	Next(ctx context.Context) (*Entry, error) // returns (nil, io.EOF) when done
	Close() error
}
```

- [ ] **Step 2: Commit**

No code to test yet (interface only); proceed to implementation.

```bash
git add internal/store/store.go
git commit -m "feat(store): define Store interface"
```

---

## Task 4: Store package — filesystem implementation (Put)

**Files:**
- Create: `internal/store/fs.go`
- Create: `internal/store/fs_test.go`

- [ ] **Step 1: Write failing tests for `Put`**

Write `internal/store/fs_test.go`:
```go
package store

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func newFS(t *testing.T) *FSStore {
	t.Helper()
	dir := t.TempDir()
	s, err := NewFSStore(dir)
	if err != nil {
		t.Fatalf("NewFSStore: %v", err)
	}
	return s
}

func TestFS_Put_WritesFile(t *testing.T) {
	s := newFS(t)
	err := s.Put(context.Background(), "node-a", strings.NewReader(`{"x":1}`))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(s.dir, "node-a.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != `{"x":1}` {
		t.Errorf("content=%q", string(got))
	}
}

func TestFS_Put_Overwrites(t *testing.T) {
	s := newFS(t)
	ctx := context.Background()
	if err := s.Put(ctx, "node-a", strings.NewReader("v1")); err != nil {
		t.Fatalf("first Put: %v", err)
	}
	if err := s.Put(ctx, "node-a", strings.NewReader("v2")); err != nil {
		t.Fatalf("second Put: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(s.dir, "node-a.json"))
	if string(got) != "v2" {
		t.Errorf("content=%q want v2", string(got))
	}
}

func TestFS_Put_InvalidNode(t *testing.T) {
	s := newFS(t)
	cases := []string{"", "../evil", "a/b", "a\x00b", "."}
	for _, name := range cases {
		err := s.Put(context.Background(), name, strings.NewReader("x"))
		if err != ErrInvalidNode {
			t.Errorf("Put(%q) err=%v want ErrInvalidNode", name, err)
		}
	}
}

func TestFS_Put_NoTmpLeftBehind(t *testing.T) {
	s := newFS(t)
	_ = s.Put(context.Background(), "node-a", strings.NewReader("x"))
	entries, _ := os.ReadDir(s.dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("found leftover tmp: %s", e.Name())
		}
	}
}

func TestFS_Put_ConcurrentSameNode(t *testing.T) {
	s := newFS(t)
	var wg sync.WaitGroup
	ctx := context.Background()
	for i := 0; i < 20; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			_ = s.Put(ctx, "node-a", bytes.NewReader([]byte{byte(i)}))
		}()
	}
	wg.Wait()
	// File exists and is exactly 1 byte; no crash.
	fi, err := os.Stat(filepath.Join(s.dir, "node-a.json"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if fi.Size() != 1 {
		t.Errorf("size=%d want 1", fi.Size())
	}
}

func TestFS_Put_ContextCanceled(t *testing.T) {
	s := newFS(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Source that would block; canceled context should short-circuit.
	err := s.Put(ctx, "node-a", strings.NewReader("x"))
	if err == nil {
		t.Errorf("Put expected error on canceled ctx, got nil")
	}
}

// Helper: suppress unused import when building under some toolchains.
var _ = io.Discard
```

- [ ] **Step 2: Run and verify failure**

```bash
go test ./internal/store/... -v
```
Expected: FAIL — `NewFSStore` undefined.

- [ ] **Step 3: Implement `fs.go`**

Write `internal/store/fs.go`:
```go
package store

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const fileExt = ".json"

// FSStore stores each node's snapshot at {dir}/{node}.json.
type FSStore struct {
	dir string
}

func NewFSStore(dir string) (*FSStore, error) {
	if dir == "" {
		return nil, fmt.Errorf("empty data dir")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dir, err)
	}
	// Sanity: write + remove a probe file to surface permission errors at startup.
	probe := filepath.Join(dir, ".writecheck")
	if err := os.WriteFile(probe, []byte{}, 0o600); err != nil {
		return nil, fmt.Errorf("data dir not writable: %w", err)
	}
	_ = os.Remove(probe)
	return &FSStore{dir: dir}, nil
}

// validNode accepts only [A-Za-z0-9._-]+ up to 253 chars (K8s node name limit).
func validNode(name string) bool {
	if name == "" || len(name) > 253 || name == "." || name == ".." {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '.' || c == '_' || c == '-':
		default:
			return false
		}
	}
	return !strings.Contains(name, "..")
}

func (s *FSStore) Put(ctx context.Context, node string, body io.Reader) error {
	if !validNode(node) {
		return ErrInvalidNode
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	final := filepath.Join(s.dir, node+fileExt)
	tmp, err := os.CreateTemp(s.dir, node+".*.tmp")
	if err != nil {
		return fmt.Errorf("create tmp: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	// Copy honoring context cancellation.
	if _, err := copyWithCtx(ctx, tmp, body); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("sync tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close tmp: %w", err)
	}
	if err := os.Rename(tmpName, final); err != nil {
		cleanup()
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// copyWithCtx copies src → dst in chunks, returning ctx.Err() if canceled.
func copyWithCtx(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	const chunk = 32 * 1024
	buf := make([]byte, chunk)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		n, rerr := src.Read(buf)
		if n > 0 {
			w, werr := dst.Write(buf[:n])
			total += int64(w)
			if werr != nil {
				return total, werr
			}
		}
		if rerr == io.EOF {
			return total, nil
		}
		if rerr != nil {
			return total, rerr
		}
	}
}
```

- [ ] **Step 4: Run and verify pass**

```bash
go test ./internal/store/... -v -race
```
Expected: all 6 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store/
git commit -m "feat(store): filesystem Put with atomic write"
```

---

## Task 5: Store package — filesystem List iterator

**Files:**
- Modify: `internal/store/fs.go`
- Modify: `internal/store/fs_test.go`

- [ ] **Step 1: Append failing tests for `List`**

Append to `internal/store/fs_test.go`:
```go
func TestFS_List_Empty(t *testing.T) {
	s := newFS(t)
	it, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	defer it.Close()
	e, err := it.Next(context.Background())
	if err != io.EOF {
		t.Errorf("Next err=%v want io.EOF", err)
	}
	if e != nil {
		t.Errorf("Next entry=%v want nil", e)
	}
}

func TestFS_List_MultipleNodes(t *testing.T) {
	s := newFS(t)
	ctx := context.Background()
	nodes := map[string]string{
		"node-a": "aa",
		"node-b": "bb",
		"node-c": "cc",
	}
	for n, v := range nodes {
		if err := s.Put(ctx, n, strings.NewReader(v)); err != nil {
			t.Fatalf("Put %s: %v", n, err)
		}
	}
	it, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	defer it.Close()

	seen := map[string]string{}
	for {
		e, err := it.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		b, err := io.ReadAll(e.Body)
		_ = e.Body.Close()
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		seen[e.Node] = string(b)
	}
	for n, v := range nodes {
		if seen[n] != v {
			t.Errorf("node %s: got %q want %q", n, seen[n], v)
		}
	}
}

func TestFS_List_IgnoresNonJSON(t *testing.T) {
	s := newFS(t)
	ctx := context.Background()
	_ = s.Put(ctx, "node-a", strings.NewReader("x"))
	// Sprinkle unrelated files.
	_ = os.WriteFile(filepath.Join(s.dir, "README.md"), []byte("ignore"), 0o600)
	_ = os.WriteFile(filepath.Join(s.dir, "foo.tmp"), []byte("ignore"), 0o600)

	it, _ := s.List(ctx)
	defer it.Close()
	var count int
	for {
		e, err := it.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		_ = e.Body.Close()
		count++
	}
	if count != 1 {
		t.Errorf("count=%d want 1", count)
	}
}
```

- [ ] **Step 2: Run and verify failure**

```bash
go test ./internal/store/... -v -race
```
Expected: FAIL — `s.List` undefined.

- [ ] **Step 3: Implement `List` in `fs.go`**

Append to `internal/store/fs.go`:
```go
func (s *FSStore) List(ctx context.Context) (Iterator, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("readdir: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if !strings.HasSuffix(n, fileExt) {
			continue
		}
		names = append(names, n)
	}
	return &fsIterator{dir: s.dir, names: names}, nil
}

type fsIterator struct {
	dir   string
	names []string
	idx   int
}

func (it *fsIterator) Next(ctx context.Context) (*Entry, error) {
	for it.idx < len(it.names) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := it.names[it.idx]
		it.idx++
		node := strings.TrimSuffix(name, fileExt)
		f, err := os.Open(filepath.Join(it.dir, name))
		if err != nil {
			// Node file may have been deleted between readdir and open — skip.
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		return &Entry{Node: node, Body: f}, nil
	}
	return nil, io.EOF
}

func (it *fsIterator) Close() error { return nil }
```

- [ ] **Step 4: Run and verify pass**

```bash
go test ./internal/store/... -v -race
```
Expected: all 9 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store/
git commit -m "feat(store): filesystem List iterator"
```

---

## Task 6: Server middleware — body size limit + gzip decoding + auth

**Files:**
- Create: `internal/server/middleware.go`
- Create: `internal/server/middleware_test.go`

- [ ] **Step 1: Write failing tests for middleware**

Write `internal/server/middleware_test.go`:
```go
package server

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func echoHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		w.Write(b)
	})
}

func TestWithBodyLimit_UnderLimit(t *testing.T) {
	h := WithBodyLimit(10, echoHandler())
	req := httptest.NewRequest("POST", "/", strings.NewReader("hello"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("code=%d", rec.Code)
	}
	if rec.Body.String() != "hello" {
		t.Errorf("body=%q", rec.Body.String())
	}
}

func TestWithBodyLimit_OverLimit(t *testing.T) {
	h := WithBodyLimit(4, echoHandler())
	req := httptest.NewRequest("POST", "/", strings.NewReader("hello"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("code=%d want 413", rec.Code)
	}
}

func TestWithGzipDecode_Passthrough(t *testing.T) {
	h := WithGzipDecode(echoHandler())
	req := httptest.NewRequest("POST", "/", strings.NewReader("plain"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Body.String() != "plain" {
		t.Errorf("body=%q", rec.Body.String())
	}
}

func TestWithGzipDecode_Decodes(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write([]byte("payload"))
	gz.Close()

	h := WithGzipDecode(echoHandler())
	req := httptest.NewRequest("POST", "/", &buf)
	req.Header.Set("Content-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("code=%d body=%q", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "payload" {
		t.Errorf("body=%q", rec.Body.String())
	}
}

func TestWithGzipDecode_MalformedGzip(t *testing.T) {
	h := WithGzipDecode(echoHandler())
	req := httptest.NewRequest("POST", "/", strings.NewReader("\x1f\x8bNOPE"))
	req.Header.Set("Content-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("code=%d want 400", rec.Code)
	}
}

func TestWithBearerAuth_NoTokenConfigured(t *testing.T) {
	h := WithBearerAuth("", echoHandler())
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("code=%d want 200", rec.Code)
	}
}

func TestWithBearerAuth_MissingHeader(t *testing.T) {
	h := WithBearerAuth("secret", echoHandler())
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Errorf("code=%d want 401", rec.Code)
	}
}

func TestWithBearerAuth_WrongToken(t *testing.T) {
	h := WithBearerAuth("secret", echoHandler())
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Errorf("code=%d want 401", rec.Code)
	}
}

func TestWithBearerAuth_CorrectToken(t *testing.T) {
	h := WithBearerAuth("secret", echoHandler())
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("code=%d want 200", rec.Code)
	}
}
```

- [ ] **Step 2: Run and verify failure**

```bash
go test ./internal/server/... -v
```
Expected: FAIL — `WithBodyLimit`, `WithGzipDecode`, `WithBearerAuth` undefined.

- [ ] **Step 3: Implement `middleware.go`**

Write `internal/server/middleware.go`:
```go
package server

import (
	"compress/gzip"
	"crypto/subtle"
	"net/http"
	"strings"
)

// WithBodyLimit caps r.Body to max bytes; attempts to read beyond return
// an error which handlers should surface as 413.
func WithBodyLimit(max int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, max)
		next.ServeHTTP(w, r)
	})
}

// WithGzipDecode wraps r.Body in a gzip.Reader when the request carries
// `Content-Encoding: gzip`. Other encodings pass through.
func WithGzipDecode(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.EqualFold(r.Header.Get("Content-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		gr, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, "malformed gzip body", http.StatusBadRequest)
			return
		}
		defer gr.Close()
		// Replace body so downstream handler reads decoded bytes.
		origClose := r.Body
		r.Body = gzipReadCloser{Reader: gr, orig: origClose}
		r.Header.Del("Content-Encoding")
		next.ServeHTTP(w, r)
	})
}

type gzipReadCloser struct {
	*gzip.Reader
	orig interface{ Close() error }
}

func (g gzipReadCloser) Close() error {
	_ = g.Reader.Close()
	return g.orig.Close()
}

// WithBearerAuth requires `Authorization: Bearer <token>` when token != "".
// If token == "", all requests pass through (auth disabled).
func WithBearerAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		h := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(h, prefix) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		got := h[len(prefix):]
		if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 4: Run and verify pass**

```bash
go test ./internal/server/... -v
```
Expected: all 9 middleware tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/
git commit -m "feat(server): add body-limit, gzip-decode, bearer-auth middleware"
```

---

## Task 7: Server — POST /api/v1/snapshots handler

**Files:**
- Create: `internal/server/snapshot.go`
- Create: `internal/server/snapshot_test.go`

- [ ] **Step 1: Write failing POST tests**

Write `internal/server/snapshot_test.go`:
```go
package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scitix/sichek-collector/internal/store"
)

func newTestStore(t *testing.T) *store.FSStore {
	t.Helper()
	dir := t.TempDir()
	s, err := store.NewFSStore(dir)
	if err != nil {
		t.Fatalf("NewFSStore: %v", err)
	}
	return s
}

func readFile(t *testing.T, s *store.FSStore, node string) string {
	t.Helper()
	ctx := context.Background()
	it, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	defer it.Close()
	for {
		e, err := it.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if e.Node == node {
			b, _ := io.ReadAll(e.Body)
			e.Body.Close()
			return string(b)
		}
		e.Body.Close()
	}
	t.Fatalf("node %s not found", node)
	return ""
}

func TestSnapshotPOST_OK(t *testing.T) {
	s := newTestStore(t)
	h := NewSnapshotHandler(s, true, 1024*1024)

	req := httptest.NewRequest("POST", "/api/v1/snapshots", strings.NewReader(`{"ok":1}`))
	req.Header.Set("X-Sichek-Node", "node-a")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 204 {
		t.Fatalf("code=%d body=%q", rec.Code, rec.Body.String())
	}
	if got := readFile(t, s, "node-a"); got != `{"ok":1}` {
		t.Errorf("stored=%q", got)
	}
}

func TestSnapshotPOST_MissingNodeHeader(t *testing.T) {
	s := newTestStore(t)
	h := NewSnapshotHandler(s, true, 1024*1024)
	req := httptest.NewRequest("POST", "/api/v1/snapshots", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Errorf("code=%d want 400", rec.Code)
	}
}

func TestSnapshotPOST_InvalidNode(t *testing.T) {
	s := newTestStore(t)
	h := NewSnapshotHandler(s, true, 1024*1024)
	req := httptest.NewRequest("POST", "/api/v1/snapshots", strings.NewReader(`{}`))
	req.Header.Set("X-Sichek-Node", "../evil")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Errorf("code=%d want 400", rec.Code)
	}
}

func TestSnapshotPOST_OversizedBody(t *testing.T) {
	s := newTestStore(t)
	h := WithBodyLimit(3, NewSnapshotHandler(s, true, 3))
	req := httptest.NewRequest("POST", "/api/v1/snapshots", strings.NewReader("12345"))
	req.Header.Set("X-Sichek-Node", "node-a")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("code=%d want 413", rec.Code)
	}
}

func TestSnapshotPOST_Gzip(t *testing.T) {
	s := newTestStore(t)
	h := WithGzipDecode(NewSnapshotHandler(s, true, 1024*1024))

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write([]byte(`{"v":2}`))
	gz.Close()

	req := httptest.NewRequest("POST", "/api/v1/snapshots", &buf)
	req.Header.Set("X-Sichek-Node", "node-a")
	req.Header.Set("Content-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 204 {
		t.Fatalf("code=%d body=%q", rec.Code, rec.Body.String())
	}
	if got := readFile(t, s, "node-a"); got != `{"v":2}` {
		t.Errorf("stored=%q want {\"v\":2}", got)
	}
}

func TestSnapshotPOST_WrongMethod(t *testing.T) {
	s := newTestStore(t)
	h := NewSnapshotHandler(s, true, 1024*1024)
	req := httptest.NewRequest("PUT", "/api/v1/snapshots", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("code=%d want 405", rec.Code)
	}
}

// Quiet unused imports in early dev.
var _ = os.Stat
var _ = filepath.Join
```

- [ ] **Step 2: Run and verify failure**

```bash
go test ./internal/server/... -v
```
Expected: FAIL — `NewSnapshotHandler` undefined.

- [ ] **Step 3: Implement POST path in `snapshot.go`**

Write `internal/server/snapshot.go`:
```go
package server

import (
	"errors"
	"net/http"

	"github.com/scitix/sichek-collector/internal/store"
)

const HeaderNode = "X-Sichek-Node"

// SnapshotHandler serves POST /api/v1/snapshots and (when readEnabled) GET.
type SnapshotHandler struct {
	store       store.Store
	readEnabled bool
	maxBody     int64
}

func NewSnapshotHandler(s store.Store, readEnabled bool, maxBody int64) *SnapshotHandler {
	return &SnapshotHandler{store: s, readEnabled: readEnabled, maxBody: maxBody}
}

func (h *SnapshotHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.handlePost(w, r)
	case http.MethodGet:
		if !h.readEnabled {
			http.Error(w, "read api disabled", http.StatusNotFound)
			return
		}
		h.handleGet(w, r)
	default:
		w.Header().Set("Allow", "POST, GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *SnapshotHandler) handlePost(w http.ResponseWriter, r *http.Request) {
	node := r.Header.Get(HeaderNode)
	if node == "" {
		http.Error(w, "missing "+HeaderNode, http.StatusBadRequest)
		return
	}
	if err := h.store.Put(r.Context(), node, r.Body); err != nil {
		if errors.Is(err, store.ErrInvalidNode) {
			http.Error(w, "invalid node", http.StatusBadRequest)
			return
		}
		// MaxBytesReader signals over-limit via an error on Read.
		var mbsErr *http.MaxBytesError
		if errors.As(err, &mbsErr) {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SnapshotHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	// Implemented in Task 8.
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
```

- [ ] **Step 4: Add go.sum / module import**

Since `snapshot_test.go` imports `github.com/scitix/sichek-collector/internal/store`, confirm module path matches. Run:
```bash
go mod tidy
```

- [ ] **Step 5: Run and verify pass**

```bash
go test ./internal/server/... -v
```
Expected: POST tests PASS. `TestHandler_Get_*` (added later) currently absent.

- [ ] **Step 6: Commit**

```bash
git add internal/server/ go.mod go.sum
git commit -m "feat(server): POST /api/v1/snapshots handler"
```

---

## Task 8: Server — GET /api/v1/snapshots NDJSON streaming

**Files:**
- Modify: `internal/server/snapshot.go`
- Modify: `internal/server/snapshot_test.go`

- [ ] **Step 1: Append failing GET tests**

Append to `internal/server/snapshot_test.go`:
```go
func TestSnapshotGET_Disabled(t *testing.T) {
	s := newTestStore(t)
	h := NewSnapshotHandler(s, false, 1024)
	req := httptest.NewRequest("GET", "/api/v1/snapshots", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Errorf("code=%d want 404", rec.Code)
	}
}

func TestSnapshotGET_Empty(t *testing.T) {
	s := newTestStore(t)
	h := NewSnapshotHandler(s, true, 1024)
	req := httptest.NewRequest("GET", "/api/v1/snapshots", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("code=%d want 200", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body=%q want empty", rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/x-ndjson" {
		t.Errorf("Content-Type=%q", got)
	}
}

func TestSnapshotGET_MultiNodeNDJSON(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for _, n := range []string{"node-a", "node-b"} {
		_ = s.Put(ctx, n, strings.NewReader(`{"n":"`+n+`"}`))
	}
	h := NewSnapshotHandler(s, true, 1024)
	req := httptest.NewRequest("GET", "/api/v1/snapshots", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d", rec.Code)
	}
	lines := strings.Split(strings.TrimRight(rec.Body.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines=%d body=%q", len(lines), rec.Body.String())
	}
	seen := map[string]bool{}
	for _, ln := range lines {
		// Validate shape: {"node":"...","snapshot":{...}}
		if !strings.Contains(ln, `"node":`) || !strings.Contains(ln, `"snapshot":`) {
			t.Errorf("malformed line: %q", ln)
		}
		for _, n := range []string{"node-a", "node-b"} {
			if strings.Contains(ln, `"node":"`+n+`"`) {
				seen[n] = true
			}
		}
	}
	if !seen["node-a"] || !seen["node-b"] {
		t.Errorf("seen=%v", seen)
	}
}
```

- [ ] **Step 2: Run and verify failure**

```bash
go test ./internal/server/... -v -run GET
```
Expected: FAIL — `handleGet` returns 501.

- [ ] **Step 3: Replace `handleGet` with NDJSON streaming**

First, update the import block at the top of `internal/server/snapshot.go` to include `encoding/json` and `io`:
```go
import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/scitix/sichek-collector/internal/store"
)
```

Then replace the stub `handleGet`:
```go
func (h *SnapshotHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	it, err := h.store.List(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer it.Close()

	w.Header().Set("Content-Type", "application/x-ndjson")
	// No Content-Length — we stream.
	for {
		e, err := it.Next(r.Context())
		if err != nil {
			// io.EOF (normal) or ctx cancellation. Partial response is OK for NDJSON.
			return
		}
		if werr := writeNDJSONLine(w, e); werr != nil {
			// Client disconnected; stop.
			return
		}
	}
}

// writeNDJSONLine emits one `{"node":"<n>","snapshot":<raw>}\n` line.
// The body is written as raw bytes (trusted to be valid JSON from upload).
// Empty bodies become `"snapshot":null` so the line is still valid JSON.
func writeNDJSONLine(w http.ResponseWriter, e *store.Entry) error {
	defer e.Body.Close()

	nodeJSON, err := json.Marshal(e.Node)
	if err != nil {
		return err
	}

	if _, err := w.Write([]byte(`{"node":`)); err != nil {
		return err
	}
	if _, err := w.Write(nodeJSON); err != nil {
		return err
	}
	if _, err := w.Write([]byte(`,"snapshot":`)); err != nil {
		return err
	}

	// Buffer one chunk to detect empty-body case without buffering full payload.
	buf := make([]byte, 32*1024)
	n, rerr := e.Body.Read(buf)
	if n == 0 && rerr != nil {
		if _, err := w.Write([]byte("null")); err != nil {
			return err
		}
	} else {
		if _, err := w.Write(buf[:n]); err != nil {
			return err
		}
		// Drain remainder of body.
		if _, err := io.Copy(w, e.Body); err != nil {
			return err
		}
	}
	if _, err := w.Write([]byte("}\n")); err != nil {
		return err
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}
```

(Keep any other existing imports from Task 7.)

- [ ] **Step 4: Run and verify pass**

```bash
go test ./internal/server/... -v
```
Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/snapshot.go internal/server/snapshot_test.go
git commit -m "feat(server): GET /api/v1/snapshots streams NDJSON"
```

---

## Task 9: Health handler + router

**Files:**
- Create: `internal/server/health.go`
- Create: `internal/server/server.go`
- Create: `internal/server/server_test.go`

- [ ] **Step 1: Write `health.go`**

```go
package server

import "net/http"

// HealthHandler returns 200 OK unconditionally.
func HealthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}
```

- [ ] **Step 2: Write failing integration test for router**

Write `internal/server/server_test.go`:
```go
package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scitix/sichek-collector/internal/store"
)

func TestRouter_Routes(t *testing.T) {
	s, _ := store.NewFSStore(t.TempDir())
	h := NewRouter(Options{
		Store:         s,
		EnableReadAPI: true,
		MaxBodySize:   1024 * 1024,
		ReadToken:     "",
	})

	srv := httptest.NewServer(h)
	defer srv.Close()

	// Healthz
	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("healthz: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(b) != "ok" {
		t.Errorf("healthz code=%d body=%q", resp.StatusCode, string(b))
	}

	// POST snapshot
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/snapshots", strings.NewReader(`{"a":1}`))
	req.Header.Set("X-Sichek-Node", "node-a")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Errorf("post code=%d", resp.StatusCode)
	}

	// GET snapshot
	resp, err = http.Get(srv.URL + "/api/v1/snapshots")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	b, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("get code=%d", resp.StatusCode)
	}
	if !strings.Contains(string(b), `"node":"node-a"`) {
		t.Errorf("get body=%q", string(b))
	}

	// 404 for unknown path
	resp, _ = http.Get(srv.URL + "/nope")
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("unknown path code=%d", resp.StatusCode)
	}
}

func TestRouter_TokenProtectsGET(t *testing.T) {
	s, _ := store.NewFSStore(t.TempDir())
	h := NewRouter(Options{
		Store:         s,
		EnableReadAPI: true,
		MaxBodySize:   1024 * 1024,
		ReadToken:     "topsecret",
	})
	srv := httptest.NewServer(h)
	defer srv.Close()

	// Unauthenticated GET → 401
	resp, _ := http.Get(srv.URL + "/api/v1/snapshots")
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Errorf("no token code=%d want 401", resp.StatusCode)
	}

	// Authenticated GET → 200
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/snapshots", nil)
	req.Header.Set("Authorization", "Bearer topsecret")
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("with token code=%d want 200", resp.StatusCode)
	}

	// POST remains open (no token)
	req, _ = http.NewRequest("POST", srv.URL+"/api/v1/snapshots", strings.NewReader("{}"))
	req.Header.Set("X-Sichek-Node", "node-a")
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Errorf("post code=%d", resp.StatusCode)
	}
}
```

- [ ] **Step 3: Run and verify failure**

```bash
go test ./internal/server/... -v -run Router
```
Expected: FAIL — `NewRouter`, `Options` undefined.

- [ ] **Step 4: Implement `server.go`**

Write `internal/server/server.go`:
```go
package server

import (
	"net/http"

	"github.com/scitix/sichek-collector/internal/store"
)

type Options struct {
	Store         store.Store
	EnableReadAPI bool
	MaxBodySize   int64
	ReadToken     string // if non-empty, guards GET /api/v1/snapshots
}

// NewRouter wires handlers and middleware into an http.Handler.
func NewRouter(opts Options) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", HealthHandler())

	snap := NewSnapshotHandler(opts.Store, opts.EnableReadAPI, opts.MaxBodySize)

	// POST path: body-limit + optional gzip decode. GET path: token auth.
	// We multiplex on method inside SnapshotHandler; compose the middleware
	// chains via method-aware dispatch.
	mux.Handle("/api/v1/snapshots", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			WithBodyLimit(opts.MaxBodySize, WithGzipDecode(snap)).ServeHTTP(w, r)
		case http.MethodGet:
			WithBearerAuth(opts.ReadToken, snap).ServeHTTP(w, r)
		default:
			snap.ServeHTTP(w, r) // lets snap emit 405
		}
	}))

	return mux
}
```

- [ ] **Step 5: Run and verify pass**

```bash
go test ./internal/server/... -v
```
Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/server/health.go internal/server/server.go internal/server/server_test.go
git commit -m "feat(server): health handler and router wiring"
```

---

## Task 10: Main entry — wire config, store, server, signals

**Files:**
- Create: `cmd/sichek-collector/main.go`

- [ ] **Step 1: Write `main.go`**

```go
// sichek-collector receives the latest snapshot.json from each GPU node
// and persists one file per node.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/scitix/sichek-collector/internal/config"
	"github.com/scitix/sichek-collector/internal/server"
	"github.com/scitix/sichek-collector/internal/store"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("sichek-collector: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log.Printf("config: listen=%s data=%s read_api=%v max_body=%d",
		cfg.ListenAddr, cfg.DataDir, cfg.EnableReadAPI, cfg.MaxBodySize)

	st, err := store.NewFSStore(cfg.DataDir)
	if err != nil {
		return err
	}

	router := server.NewRouter(server.Options{
		Store:         st,
		EnableReadAPI: cfg.EnableReadAPI,
		MaxBodySize:   cfg.MaxBodySize,
		ReadToken:     cfg.ReadToken,
	})

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Printf("listening on %s", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
		close(serveErr)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serveErr:
		return err
	case sig := <-sigCh:
		log.Printf("received %v, shutting down", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownGrace)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return err
	}
	return nil
}
```

- [ ] **Step 2: Build and smoke-test**

```bash
go build -o bin/sichek-collector ./cmd/sichek-collector
mkdir -p tmp/data
DATA_DIR=$(pwd)/tmp/data LISTEN_ADDR=:38080 ./bin/sichek-collector &
COLL_PID=$!
sleep 0.5

# Healthz
curl -sS http://127.0.0.1:38080/healthz
echo

# POST (should 204)
curl -sS -o /dev/null -w "post=%{http_code}\n" \
  -X POST http://127.0.0.1:38080/api/v1/snapshots \
  -H "X-Sichek-Node: dev-node" \
  -d '{"hello":"world"}'

# Read-API default off → 404
curl -sS -o /dev/null -w "get=%{http_code}\n" http://127.0.0.1:38080/api/v1/snapshots

ls tmp/data/
cat tmp/data/dev-node.json
kill $COLL_PID
```

Expected: `ok`, `post=204`, `get=404`, `dev-node.json` containing `{"hello":"world"}`.

- [ ] **Step 3: Run tests for the whole module**

```bash
go test -race -count=1 ./...
```
Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/sichek-collector/main.go
git commit -m "feat(cmd): main entry with graceful shutdown"
```

---

## Task 11: Dockerfile

**Files:**
- Create: `Dockerfile`
- Create: `.dockerignore`

- [ ] **Step 1: Write `.dockerignore`**

```
.git
.github
bin/
tmp/
coverage.txt
*.test
```

- [ ] **Step 2: Write `Dockerfile` (multi-stage, distroless final)**

```dockerfile
# syntax=docker/dockerfile:1
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" \
    -o /out/sichek-collector ./cmd/sichek-collector

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/sichek-collector /sichek-collector
USER nonroot:nonroot
EXPOSE 38080
ENTRYPOINT ["/sichek-collector"]
```

- [ ] **Step 3: Build and smoke-test the image**

```bash
make docker TAG=dev
docker run --rm -d --name collector-test \
  -e DATA_DIR=/tmp -p 38080:38080 sichek-collector:dev
sleep 1
curl -sS http://127.0.0.1:38080/healthz
docker rm -f collector-test
```
Expected: `ok`.

- [ ] **Step 4: Commit**

```bash
git add Dockerfile .dockerignore
git commit -m "build: multi-stage Dockerfile with distroless runtime"
```

---

## Task 12: K8s manifests

**Files:**
- Create: `deploy/pvc.yaml`
- Create: `deploy/deployment.yaml`
- Create: `deploy/service.yaml`
- Create: `deploy/ingress.yaml`

- [ ] **Step 1: Write `deploy/pvc.yaml`**

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: sichek-collector-data
  namespace: monitoring
  labels:
    app: sichek-collector
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: 2Gi
```

- [ ] **Step 2: Write `deploy/deployment.yaml`**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sichek-collector
  namespace: monitoring
  labels:
    app: sichek-collector
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: sichek-collector
  template:
    metadata:
      labels:
        app: sichek-collector
    spec:
      containers:
        - name: sichek-collector
          image: sichek-collector:dev
          imagePullPolicy: IfNotPresent
          ports:
            - name: http
              containerPort: 38080
          env:
            - name: LISTEN_ADDR
              value: ":38080"
            - name: DATA_DIR
              value: "/data"
            - name: ENABLE_READ_API
              value: "false"
            - name: MAX_BODY_SIZE
              value: "10485760"
            - name: LOG_LEVEL
              value: "info"
          volumeMounts:
            - name: data
              mountPath: /data
          readinessProbe:
            httpGet:
              path: /healthz
              port: http
            initialDelaySeconds: 2
            periodSeconds: 10
          livenessProbe:
            httpGet:
              path: /healthz
              port: http
            initialDelaySeconds: 10
            periodSeconds: 30
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 200m
              memory: 256Mi
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: sichek-collector-data
```

- [ ] **Step 3: Write `deploy/service.yaml`**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: sichek-collector
  namespace: monitoring
  labels:
    app: sichek-collector
spec:
  type: ClusterIP
  selector:
    app: sichek-collector
  ports:
    - name: http
      port: 38080
      targetPort: http
```

- [ ] **Step 4: Write `deploy/ingress.yaml` (optional GET)**

```yaml
# Apply only when ENABLE_READ_API=true in the Deployment AND you want
# external analysis service to GET /api/v1/snapshots over HTTPS.
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: sichek-collector
  namespace: monitoring
  annotations:
    # Example nginx annotations; adjust per cluster.
    nginx.ingress.kubernetes.io/backend-protocol: "HTTP"
spec:
  ingressClassName: nginx
  rules:
    - host: sichek-collector.example.com
      http:
        paths:
          - path: /api/v1/snapshots
            pathType: Prefix
            backend:
              service:
                name: sichek-collector
                port:
                  number: 38080
  tls:
    - hosts:
        - sichek-collector.example.com
      secretName: sichek-collector-tls
```

- [ ] **Step 5: Validate YAML**

```bash
for f in deploy/*.yaml; do
  kubectl apply --dry-run=client -f "$f"
done
```
Expected: each file reports `created (dry run)` or `configured (dry run)`.

- [ ] **Step 6: Commit**

```bash
git add deploy/
git commit -m "deploy: K8s manifests (Deployment, Service, PVC, Ingress)"
```

---

## Task 13: End-to-end integration test

**Files:**
- Create: `cmd/sichek-collector/integration_test.go`

- [ ] **Step 1: Write integration test**

```go
//go:build integration

package main

import (
	"io"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestCollector_E2E builds and runs the binary, then POSTs + GETs.
// Run with: go test -tags=integration ./cmd/sichek-collector -v
func TestCollector_E2E(t *testing.T) {
	bin := "./bin/sichek-collector"
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	dataDir := t.TempDir()
	cmd := exec.Command(bin)
	cmd.Env = append(cmd.Env,
		"LISTEN_ADDR=:38099",
		"DATA_DIR="+dataDir,
		"ENABLE_READ_API=true",
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() {
		cmd.Process.Signal(syscall.SIGTERM)
		cmd.Wait()
	})

	// Wait for readiness.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if resp, err := http.Get("http://127.0.0.1:38099/healthz"); err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// POST.
	req, _ := http.NewRequest("POST", "http://127.0.0.1:38099/api/v1/snapshots",
		strings.NewReader(`{"node":"x","ts":"t"}`))
	req.Header.Set("X-Sichek-Node", "e2e-node")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Fatalf("post code=%d", resp.StatusCode)
	}

	// GET.
	resp, err = http.Get("http://127.0.0.1:38099/api/v1/snapshots")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), `"node":"e2e-node"`) {
		t.Errorf("get body=%q", string(b))
	}
}
```

Add `import "syscall"` at the top (test file requires it).

- [ ] **Step 2: Run integration test**

```bash
go test -tags=integration ./cmd/sichek-collector -v
```
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add cmd/sichek-collector/integration_test.go
git commit -m "test(e2e): integration test covering POST and GET"
```

---

## Task 14: README deployment section

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Overwrite `README.md` with full guide**

```markdown
# sichek-collector

In-cluster HTTP service that receives the latest `snapshot.json` from each
sichek-instrumented GPU node and persists one file per node (latest only).

## Endpoints

| Method | Path                  | Description                                       |
| ------ | --------------------- | ------------------------------------------------- |
| POST   | /api/v1/snapshots     | Node upload. Requires header `X-Sichek-Node: <name>`. Body is raw snapshot.json (optionally `Content-Encoding: gzip`). Returns 204. |
| GET    | /api/v1/snapshots     | (Optional, `ENABLE_READ_API=true`.) Streams NDJSON of all nodes: `{"node":"<n>","snapshot":{...}}\n`. |
| GET    | /healthz              | Readiness/liveness probe.                          |

## Configuration (env)

| Var               | Default     | Notes                                       |
| ----------------- | ----------- | ------------------------------------------- |
| `LISTEN_ADDR`     | `:38080`    |                                             |
| `DATA_DIR`        | `/data`     | PVC mount.                                  |
| `ENABLE_READ_API` | `false`     | Enables the GET endpoint.                   |
| `MAX_BODY_SIZE`   | `10485760`  | Bytes; POST bodies exceeding this get 413.  |
| `READ_TOKEN`      | ``          | If non-empty, GET requires `Authorization: Bearer <token>`. |
| `LOG_LEVEL`       | `info`      |                                             |

## Build

    make build        # ./bin/sichek-collector
    make test         # unit tests with -race
    make docker       # image: sichek-collector:dev

## Run locally

    mkdir -p tmp/data
    DATA_DIR=./tmp/data ENABLE_READ_API=true ./bin/sichek-collector

## Deploy to Kubernetes

    kubectl apply -f deploy/pvc.yaml
    kubectl apply -f deploy/service.yaml
    kubectl apply -f deploy/deployment.yaml
    # Optional, only if you need external HTTP access to GET:
    # kubectl apply -f deploy/ingress.yaml

Node-side reporter (in the `sichek` repo) should be configured with:

    reporter:
      enable: true
      endpoint: "http://sichek-collector.monitoring.svc:38080/api/v1/snapshots"

## Design

See `docs/superpowers/specs/2026-04-23-sichek-collector-design.md` in the
`scitix/sichek` repo.
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: README with usage, config, deployment"
```

---

## Done Criteria

- [ ] `go test -race -count=1 ./...` passes
- [ ] `go test -tags=integration ./cmd/sichek-collector` passes
- [ ] `make docker` builds image; `curl /healthz` on running container returns `ok`
- [ ] `kubectl apply -f deploy/` succeeds (dry-run at minimum)
- [ ] Manual smoke test against a real cluster: POST from a node delivers file into PVC; optional GET returns NDJSON line for that node
