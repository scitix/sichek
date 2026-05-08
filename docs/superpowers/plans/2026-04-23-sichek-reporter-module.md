# sichek Reporter Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `Reporter` module to the sichek daemon that periodically reads `/var/sichek/data/snapshot.json` and POSTs it to the in-cluster `sichek-collector` service. Configurable via `default_user_config.yaml` with a master on/off toggle (defaults to off).

**Architecture:** New file `service/reporter.go` defining `ReporterConfig`, `Reporter`, and `Reporter.Run(ctx)`. `Run` is a ticker loop that reads the snapshot file, optionally gzip-encodes it, and POSTs with header `X-Sichek-Node`, with retry/backoff. The daemon creates the reporter at startup and calls `go reporter.Run(ctx)` when enabled. daemon `Stop()` already cancels the context, which exits the loop.

**Tech Stack:** Go 1.21+, existing sichek dependencies (`sirupsen/logrus`, `gopkg.in/yaml.v3` via `pkg/utils`); `net/http`, `compress/gzip` from stdlib. No new third-party deps.

**Spec reference:** `docs/superpowers/specs/2026-04-23-sichek-collector-design.md` §3

**Branch:** `feat/sichek-collector` (already created; worktree `/root/devnet/sichek2`)

---

## File Structure

**Created:**
- `service/reporter.go` — `ReporterConfig`, `Reporter`, `Reporter.Run(ctx)`
- `service/reporter_test.go` — unit tests using `httptest.Server`

**Modified:**
- `service/daemon.go` — add `reporter *Reporter` field; instantiate in `NewService`; start goroutine in `Run`
- `config/default_user_config.yaml` — add `reporter:` section (disabled by default)

**Untouched** (intentionally):
- `service/snapshot.go` — Reporter reads the file on disk, not the in-memory SnapshotManager state, to keep coupling minimal. The file is already persisted atomically.

---

## Prerequisites

- Branch `feat/sichek-collector` checked out (already done).
- Existing daemon builds green: `cd /root/devnet/sichek2 && go build ./...`
- Worktree is this repo at `/root/devnet/sichek2`.

---

## Task 1: Baseline verification

- [ ] **Step 1: Confirm branch and clean tree**

```bash
cd /root/devnet/sichek2
git status --short
git branch --show-current
```
Expected branch: `feat/sichek-collector`. Ignore untracked `k8s/*.yaml` and `scripts/sichek_*.sh` — they're not our concern and must not land in Reporter commits.

- [ ] **Step 2: Confirm tests pass before changes**

```bash
go test ./service/...
```
Expected: PASS (or matches pre-existing baseline). If anything fails pre-existing, note it but do not fix in this plan.

- [ ] **Step 3: Confirm module builds**

```bash
go build ./...
```
Expected: no output, exit 0.

---

## Task 2: Add `ReporterConfig` type + loader

**Files:**
- Create: `service/reporter.go` (skeleton only in this task — the struct and loader)
- Create: `service/reporter_test.go` (loader tests only in this task)

- [ ] **Step 1: Write failing config loader tests**

Create `service/reporter_test.go`:
```go
/*
Copyright 2024 The Scitix Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0
*/
package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeCfg(t *testing.T, yaml string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "cfg.yaml")
	if err := os.WriteFile(p, []byte(yaml), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return p
}

func TestLoadReporterConfig_Defaults(t *testing.T) {
	// Empty config → all defaults.
	p := writeCfg(t, "")
	cfg, err := LoadReporterConfig(p)
	if err != nil {
		t.Fatalf("LoadReporterConfig: %v", err)
	}
	if cfg.Enable {
		t.Errorf("Enable=true, want false (default off)")
	}
	if cfg.Endpoint != "http://sichek-collector.monitoring.svc:38080/api/v1/snapshots" {
		t.Errorf("Endpoint=%q", cfg.Endpoint)
	}
	if cfg.Interval != 60*time.Second {
		t.Errorf("Interval=%v", cfg.Interval)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout=%v", cfg.Timeout)
	}
	if cfg.RetryMax != 3 {
		t.Errorf("RetryMax=%d", cfg.RetryMax)
	}
	if !cfg.Gzip {
		t.Errorf("Gzip=false, want true")
	}
}

func TestLoadReporterConfig_Overrides(t *testing.T) {
	p := writeCfg(t, `
reporter:
  enable: true
  endpoint: "http://collector.example:1234/api/v1/snapshots"
  interval: 30s
  timeout: 5s
  retry_max: 7
  gzip: false
`)
	cfg, err := LoadReporterConfig(p)
	if err != nil {
		t.Fatalf("LoadReporterConfig: %v", err)
	}
	if !cfg.Enable {
		t.Errorf("Enable=false")
	}
	if cfg.Endpoint != "http://collector.example:1234/api/v1/snapshots" {
		t.Errorf("Endpoint=%q", cfg.Endpoint)
	}
	if cfg.Interval != 30*time.Second {
		t.Errorf("Interval=%v", cfg.Interval)
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("Timeout=%v", cfg.Timeout)
	}
	if cfg.RetryMax != 7 {
		t.Errorf("RetryMax=%d", cfg.RetryMax)
	}
	if cfg.Gzip {
		t.Errorf("Gzip=true, want false")
	}
}

func TestLoadReporterConfig_MissingFile(t *testing.T) {
	// Empty path → defaults, no error.
	cfg, err := LoadReporterConfig("")
	if err != nil {
		t.Fatalf("empty path err=%v", err)
	}
	if cfg.Enable {
		t.Errorf("Enable=true")
	}
}

func TestLoadReporterConfig_InvalidYAML(t *testing.T) {
	p := writeCfg(t, "reporter: not a map\n")
	_, err := LoadReporterConfig(p)
	if err == nil {
		t.Errorf("expected error on invalid yaml")
	}
}
```

- [ ] **Step 2: Run and verify failure**

```bash
go test ./service/... -run ReporterConfig -v
```
Expected: FAIL — `LoadReporterConfig` undefined.

- [ ] **Step 3: Create `service/reporter.go` with config types**

```go
/*
Copyright 2024 The Scitix Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0
*/
package service

import (
	"fmt"
	"time"

	"github.com/scitix/sichek/pkg/utils"
)

// ReporterConfig controls the per-daemon snapshot POSTer.
type ReporterConfig struct {
	Enable   bool          `json:"enable"   yaml:"enable"`
	Endpoint string        `json:"endpoint" yaml:"endpoint"`
	Interval time.Duration `json:"interval" yaml:"interval"`
	Timeout  time.Duration `json:"timeout"  yaml:"timeout"`
	RetryMax int           `json:"retry_max" yaml:"retry_max"`
	Gzip     bool          `json:"gzip"     yaml:"gzip"`
}

type reporterFile struct {
	Reporter ReporterConfig `json:"reporter" yaml:"reporter"`
}

// defaultReporterConfig returns the spec-defined defaults.
func defaultReporterConfig() ReporterConfig {
	return ReporterConfig{
		Enable:   false,
		Endpoint: "http://sichek-collector.monitoring.svc:38080/api/v1/snapshots",
		Interval: 60 * time.Second,
		Timeout:  30 * time.Second,
		RetryMax: 3,
		Gzip:     true,
	}
}

// LoadReporterConfig parses the reporter block from cfgFile.
// If cfgFile is "" or missing, returns defaults.
// Explicit zero values in the file override defaults (standard yaml semantics).
func LoadReporterConfig(cfgFile string) (ReporterConfig, error) {
	cfg := defaultReporterConfig()
	if cfgFile == "" {
		return cfg, nil
	}
	f := reporterFile{Reporter: cfg}
	if err := utils.LoadFromYaml(cfgFile, &f); err != nil {
		return ReporterConfig{}, fmt.Errorf("load reporter config: %w", err)
	}
	// Re-apply defaults for zero-valued fields (yaml leaves them at Go zero values).
	merged := mergeReporterDefaults(f.Reporter, cfg)
	return merged, nil
}

// mergeReporterDefaults keeps user-provided values and fills in defaults for
// fields explicitly left at Go zero (empty string, 0 duration, 0 int).
func mergeReporterDefaults(user, def ReporterConfig) ReporterConfig {
	out := user
	if out.Endpoint == "" {
		out.Endpoint = def.Endpoint
	}
	if out.Interval == 0 {
		out.Interval = def.Interval
	}
	if out.Timeout == 0 {
		out.Timeout = def.Timeout
	}
	if out.RetryMax == 0 {
		out.RetryMax = def.RetryMax
	}
	// Gzip bool: if user omitted the key, yaml leaves false — but our default is true.
	// We can't distinguish "explicit false" from "omitted" without a *bool. Keep it
	// simple: users who want gzip off explicitly write `gzip: false`. To avoid
	// turning off gzip silently when the key is missing, we detect missing block
	// via Enable + Endpoint being at defaults. For MVP we accept the limitation:
	// if the whole reporter block is absent, defaults apply (unmarshal leaves
	// the pre-filled Gzip=true alone). If the user provides partial reporter:{...}
	// without `gzip:`, their Gzip becomes false (Go zero). Document this in spec
	// as "always set gzip explicitly when overriding any other reporter field".
	return out
}
```

Note: the merge logic above is kept deliberately simple. If users partially override `reporter:`, they must set `gzip:` explicitly. This is acceptable for MVP and documented in the README in Task 7.

- [ ] **Step 4: Run and verify pass**

```bash
go test ./service/... -run ReporterConfig -v
```
Expected: PASS.

Note: `TestLoadReporterConfig_Overrides` exercises a full override (all keys set). `TestLoadReporterConfig_Defaults` uses an empty file (whole block absent), so defaults apply.

- [ ] **Step 5: Commit**

```bash
git add service/reporter.go service/reporter_test.go
git commit -m "feat(service): reporter config loader with defaults"
```

---

## Task 3: `Reporter` struct + single push implementation

**Files:**
- Modify: `service/reporter.go`
- Modify: `service/reporter_test.go`

- [ ] **Step 1: Append failing tests for single-push**

First, update the existing `import (...)` block at the top of `service/reporter_test.go` to include the additional packages needed by the new tests. The final import block should read:
```go
import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)
```

Then append the test functions below to the end of the file:

```go

func TestReporter_pushOnce_Success(t *testing.T) {
	var gotNode, gotCE, gotCT string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotNode = r.Header.Get("X-Sichek-Node")
		gotCE = r.Header.Get("Content-Encoding")
		gotCT = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		if gotCE == "gzip" {
			gz, _ := gzip.NewReader(bytes.NewReader(b))
			b, _ = io.ReadAll(gz)
		}
		gotBody = b
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	// Write a fake snapshot file.
	snapPath := filepath.Join(t.TempDir(), "snap.json")
	os.WriteFile(snapPath, []byte(`{"ok":1}`), 0o600)

	cfg := defaultReporterConfig()
	cfg.Enable = true
	cfg.Endpoint = srv.URL
	cfg.Gzip = true
	cfg.Timeout = 2 * time.Second

	r := NewReporter(cfg, snapPath, "node-a")
	if err := r.pushOnce(context.Background()); err != nil {
		t.Fatalf("pushOnce: %v", err)
	}
	if gotNode != "node-a" {
		t.Errorf("node header=%q", gotNode)
	}
	if gotCE != "gzip" {
		t.Errorf("Content-Encoding=%q want gzip", gotCE)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type=%q want application/json", gotCT)
	}
	if string(gotBody) != `{"ok":1}` {
		t.Errorf("body=%q", string(gotBody))
	}
}

func TestReporter_pushOnce_GzipDisabled(t *testing.T) {
	var gotCE string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCE = r.Header.Get("Content-Encoding")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	snapPath := filepath.Join(t.TempDir(), "snap.json")
	os.WriteFile(snapPath, []byte("raw"), 0o600)

	cfg := defaultReporterConfig()
	cfg.Enable = true
	cfg.Endpoint = srv.URL
	cfg.Gzip = false
	cfg.Timeout = 2 * time.Second

	r := NewReporter(cfg, snapPath, "node-a")
	if err := r.pushOnce(context.Background()); err != nil {
		t.Fatalf("pushOnce: %v", err)
	}
	if gotCE != "" {
		t.Errorf("Content-Encoding=%q want empty", gotCE)
	}
	if string(gotBody) != "raw" {
		t.Errorf("body=%q", string(gotBody))
	}
}

func TestReporter_pushOnce_MissingSnapshotFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called when snapshot missing")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	cfg := defaultReporterConfig()
	cfg.Enable = true
	cfg.Endpoint = srv.URL
	r := NewReporter(cfg, "/nonexistent/snap.json", "node-a")

	err := r.pushOnce(context.Background())
	if err == nil {
		t.Errorf("expected error on missing file")
	}
}

func TestReporter_pushOnce_RetryOn5xx(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	snapPath := filepath.Join(t.TempDir(), "snap.json")
	os.WriteFile(snapPath, []byte("x"), 0o600)

	cfg := defaultReporterConfig()
	cfg.Enable = true
	cfg.Endpoint = srv.URL
	cfg.RetryMax = 3
	cfg.Timeout = 2 * time.Second

	r := NewReporter(cfg, snapPath, "node-a")
	r.backoff = func(i int) time.Duration { return 0 } // speed up test

	if err := r.pushOnce(context.Background()); err != nil {
		t.Fatalf("pushOnce: %v", err)
	}
	if got := attempts.Load(); got != 3 {
		t.Errorf("attempts=%d want 3", got)
	}
}

func TestReporter_pushOnce_NoRetryOn4xx(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	snapPath := filepath.Join(t.TempDir(), "snap.json")
	os.WriteFile(snapPath, []byte("x"), 0o600)

	cfg := defaultReporterConfig()
	cfg.Enable = true
	cfg.Endpoint = srv.URL
	cfg.RetryMax = 5
	cfg.Timeout = 2 * time.Second

	r := NewReporter(cfg, snapPath, "node-a")
	r.backoff = func(i int) time.Duration { return 0 }

	if err := r.pushOnce(context.Background()); err == nil {
		t.Errorf("expected error on 400")
	}
	if got := attempts.Load(); got != 1 {
		t.Errorf("attempts=%d want 1 (no retry on 4xx)", got)
	}
}
```

- [ ] **Step 2: Run and verify failure**

```bash
go test ./service/... -run Reporter -v
```
Expected: FAIL — `NewReporter`, `pushOnce`, `Reporter.backoff` undefined.

- [ ] **Step 3: Implement `Reporter` and `pushOnce` in `reporter.go`**

First, update the `import (...)` block at the top of `service/reporter.go` so it reads:
```go
import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
)
```

Then append the code below to the end of the file:

```go

// Reporter periodically POSTs snapshot bytes to a collector.
type Reporter struct {
	cfg          ReporterConfig
	snapshotPath string
	nodeName     string
	client       *http.Client

	// backoff allows tests to inject zero-sleep. Defaults to exponential.
	backoff func(attempt int) time.Duration
}

// NewReporter constructs a Reporter. Call Run(ctx) to start the loop.
func NewReporter(cfg ReporterConfig, snapshotPath, nodeName string) *Reporter {
	return &Reporter{
		cfg:          cfg,
		snapshotPath: snapshotPath,
		nodeName:     nodeName,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		backoff: defaultBackoff,
	}
}

func defaultBackoff(attempt int) time.Duration {
	// 1s, 2s, 4s, 8s capped at 30s.
	d := time.Second << attempt
	if d > 30*time.Second {
		d = 30 * time.Second
	}
	return d
}

// pushOnce reads the snapshot file and POSTs it, with retries on 5xx or
// transport errors up to cfg.RetryMax. 4xx responses abort immediately.
func (r *Reporter) pushOnce(ctx context.Context) error {
	raw, err := os.ReadFile(r.snapshotPath)
	if err != nil {
		return fmt.Errorf("read snapshot: %w", err)
	}

	body := raw
	if r.cfg.Gzip {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		if _, err := gz.Write(raw); err != nil {
			return fmt.Errorf("gzip write: %w", err)
		}
		if err := gz.Close(); err != nil {
			return fmt.Errorf("gzip close: %w", err)
		}
		body = buf.Bytes()
	}

	var lastErr error
	attempts := r.cfg.RetryMax
	if attempts < 1 {
		attempts = 1
	}
	for i := 0; i < attempts; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(r.backoff(i - 1)):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.cfg.Endpoint, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("new request: %w", err)
		}
		req.Header.Set("X-Sichek-Node", r.nodeName)
		req.Header.Set("Content-Type", "application/json")
		if r.cfg.Gzip {
			req.Header.Set("Content-Encoding", "gzip")
		}

		resp, err := r.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		// Drain + close to allow conn reuse.
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		// 4xx: don't retry — configuration/auth errors won't fix themselves.
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return fmt.Errorf("collector returned %d", resp.StatusCode)
		}
		lastErr = fmt.Errorf("collector returned %d", resp.StatusCode)
	}
	return lastErr
}

// logEntry returns a structured logger with the standard reporter fields.
func (r *Reporter) logEntry() *logrus.Entry {
	return logrus.WithFields(logrus.Fields{
		"service": "reporter",
		"node":    r.nodeName,
	})
}
```

(Imports already updated at the start of this step.)

- [ ] **Step 4: Run and verify pass**

```bash
go test ./service/... -run Reporter -v -race
```
Expected: all Reporter tests PASS.

- [ ] **Step 5: Commit**

```bash
git add service/reporter.go service/reporter_test.go
git commit -m "feat(service): Reporter.pushOnce with gzip + retry"
```

---

## Task 4: Reporter ticker loop + disabled no-op

**Files:**
- Modify: `service/reporter.go`
- Modify: `service/reporter_test.go`

- [ ] **Step 1: Append failing tests for `Run`**

Append to `service/reporter_test.go`:
```go
func TestReporter_Run_DisabledNoOp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called when disabled")
	}))
	defer srv.Close()

	cfg := defaultReporterConfig()
	cfg.Enable = false
	cfg.Endpoint = srv.URL
	cfg.Interval = 10 * time.Millisecond

	r := NewReporter(cfg, "/tmp/unused", "node-a")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	r.Run(ctx) // should return immediately without blocking the full timeout
}

func TestReporter_Run_PushesPeriodically(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	snapPath := filepath.Join(t.TempDir(), "snap.json")
	os.WriteFile(snapPath, []byte("x"), 0o600)

	cfg := defaultReporterConfig()
	cfg.Enable = true
	cfg.Endpoint = srv.URL
	cfg.Interval = 20 * time.Millisecond
	cfg.Timeout = 1 * time.Second

	r := NewReporter(cfg, snapPath, "node-a")
	r.backoff = func(i int) time.Duration { return 0 }

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()
	r.Run(ctx)

	// With 20ms interval over ~120ms we expect 3-6 calls (boundary tolerance).
	if n := calls.Load(); n < 2 {
		t.Errorf("calls=%d, expected >=2", n)
	}
}

func TestReporter_Run_ContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	snapPath := filepath.Join(t.TempDir(), "snap.json")
	os.WriteFile(snapPath, []byte("x"), 0o600)

	cfg := defaultReporterConfig()
	cfg.Enable = true
	cfg.Endpoint = srv.URL
	cfg.Interval = 1 * time.Second

	r := NewReporter(cfg, snapPath, "node-a")
	r.backoff = func(i int) time.Duration { return 0 }

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		r.Run(ctx)
		close(done)
	}()
	// Let first push happen, then cancel.
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Error("Run did not return within 500ms after ctx cancel")
	}
}

func TestReporter_Run_PushFailureDoesNotKillLoop(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	snapPath := filepath.Join(t.TempDir(), "snap.json")
	os.WriteFile(snapPath, []byte("x"), 0o600)

	cfg := defaultReporterConfig()
	cfg.Enable = true
	cfg.Endpoint = srv.URL
	cfg.Interval = 20 * time.Millisecond
	cfg.Timeout = 200 * time.Millisecond
	cfg.RetryMax = 1

	r := NewReporter(cfg, snapPath, "node-a")
	r.backoff = func(i int) time.Duration { return 0 }

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	r.Run(ctx)

	if n := calls.Load(); n < 2 {
		t.Errorf("calls=%d, expected >=2 despite server 5xx", n)
	}
}
```

- [ ] **Step 2: Run and verify failure**

```bash
go test ./service/... -run Reporter_Run -v
```
Expected: FAIL — `Reporter.Run` undefined.

- [ ] **Step 3: Implement `Run`**

Append to `service/reporter.go`:
```go
// Run starts the reporter loop. When ctx is canceled, Run returns.
// If the reporter is disabled via config, Run returns immediately.
// Panics inside pushOnce are recovered so that the daemon's health-check
// pipeline is never affected.
func (r *Reporter) Run(ctx context.Context) {
	if !r.cfg.Enable {
		r.logEntry().Info("reporter disabled; exiting")
		return
	}
	r.logEntry().Infof("reporter started; endpoint=%s interval=%v gzip=%v",
		r.cfg.Endpoint, r.cfg.Interval, r.cfg.Gzip)

	ticker := time.NewTicker(r.cfg.Interval)
	defer ticker.Stop()

	// Fire an initial push immediately so the collector sees the node
	// as soon as the daemon comes up.
	r.pushWithRecover(ctx)

	for {
		select {
		case <-ctx.Done():
			r.logEntry().Info("reporter stopped (context canceled)")
			return
		case <-ticker.C:
			r.pushWithRecover(ctx)
		}
	}
}

func (r *Reporter) pushWithRecover(ctx context.Context) {
	defer func() {
		if p := recover(); p != nil {
			r.logEntry().Errorf("reporter panic: %v", p)
		}
	}()
	if err := r.pushOnce(ctx); err != nil {
		r.logEntry().Warnf("push failed: %v", err)
	}
}
```

- [ ] **Step 4: Run and verify pass**

```bash
go test ./service/... -run Reporter -v -race
```
Expected: all Reporter tests PASS.

- [ ] **Step 5: Commit**

```bash
git add service/reporter.go service/reporter_test.go
git commit -m "feat(service): Reporter.Run periodic loop with panic recover"
```

---

## Task 5: Node name resolution (NODE_NAME env → hostname)

**Files:**
- Modify: `service/reporter.go`
- Modify: `service/reporter_test.go`

- [ ] **Step 1: Append failing test for node name resolution**

Append to `service/reporter_test.go`:
```go
func TestResolveNodeName_EnvOverride(t *testing.T) {
	t.Setenv("NODE_NAME", "from-env")
	got := ResolveNodeName()
	if got != "from-env" {
		t.Errorf("ResolveNodeName=%q", got)
	}
}

func TestResolveNodeName_FallbackToHostname(t *testing.T) {
	t.Setenv("NODE_NAME", "")
	got := ResolveNodeName()
	host, _ := os.Hostname()
	if got != host {
		t.Errorf("ResolveNodeName=%q want %q", got, host)
	}
}
```

- [ ] **Step 2: Run and verify failure**

```bash
go test ./service/... -run ResolveNodeName -v
```
Expected: FAIL — `ResolveNodeName` undefined.

- [ ] **Step 3: Implement**

Append to `service/reporter.go`:
```go
// ResolveNodeName returns the preferred node identity:
// NODE_NAME env (set by K8s DaemonSet spec.nodeName fieldRef) if present,
// else os.Hostname().
func ResolveNodeName() string {
	if v := os.Getenv("NODE_NAME"); v != "" {
		return v
	}
	h, err := os.Hostname()
	if err != nil {
		return ""
	}
	return h
}
```

- [ ] **Step 4: Run and verify pass**

```bash
go test ./service/... -run ResolveNodeName -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add service/reporter.go service/reporter_test.go
git commit -m "feat(service): ResolveNodeName prefers NODE_NAME env"
```

---

## Task 6: Wire Reporter into `DaemonService`

**Files:**
- Modify: `service/daemon.go`

- [ ] **Step 1: Read current `service/daemon.go`** to locate insertion points (lines will shift slightly after edits).

```bash
grep -n "snapshotMgr\|DaemonService\b" /root/devnet/sichek2/service/daemon.go
```
You'll need the two areas:
- struct field list near line 50 (`snapshotMgr          *SnapshotManager`)
- `NewService` body near line 71 (`snapshotMgr, err := NewSnapshotManager(cfgFile)`)
- `Run` method near line 91
- `Stop` method near line 173

- [ ] **Step 2: Add `reporter` field to `DaemonService`**

Edit the struct (around line 50):
```go
type DaemonService struct {
	ctx                  context.Context
	cancel               context.CancelFunc
	components           map[string]common.Component
	componentsLock       sync.RWMutex
	componentsStatus     map[string]bool
	componentsStatusLock sync.RWMutex
	componentResults     map[string]<-chan *common.Result
	node                 string
	metrics              *metrics.HealthCheckResMetrics
	notifier             Notifier
	snapshotMgr          *SnapshotManager
	reporter             *Reporter // NEW
}
```

- [ ] **Step 3: Instantiate Reporter in `NewService`**

After the `snapshotMgr` is created (around line 73), add:
```go
	// Reporter: periodically POST snapshot.json to sichek-collector.
	reporterCfg, err := LoadReporterConfig(cfgFile)
	if err != nil {
		logrus.WithField("daemon", "new").Warnf("load reporter config failed: %v", err)
		reporterCfg = defaultReporterConfig() // disabled
	}
	var reporter *Reporter
	if reporterCfg.Enable {
		snapPath := consts.DefaultSnapshotPath
		if snapshotMgr != nil && snapshotMgr.path != "" {
			snapPath = snapshotMgr.path
		}
		reporter = NewReporter(reporterCfg, snapPath, ResolveNodeName())
	}
```

Then in the `daemonService := &DaemonService{...}` literal, add:
```go
		reporter:         reporter,
```

If `snapshotMgr.path` is currently unexported, we need to expose it. Check:
```bash
grep -n "path " /root/devnet/sichek2/service/snapshot.go
```
`path` is a lower-case field on `SnapshotManager`. Since we're in the same package (`service`), direct access works — no export needed.

- [ ] **Step 4: Start Reporter in `Run`**

Find `func (d *DaemonService) Run()` and add before the `for componentName, resultChan := range d.componentResults {` loop:
```go
	if d.reporter != nil {
		go d.reporter.Run(d.ctx)
	}
```

- [ ] **Step 5: Verify Stop still cancels context**

`Stop()` already calls `d.cancel()`, which propagates to the reporter's ctx. No change required.

- [ ] **Step 6: Build + test**

```bash
go build ./...
go test ./service/... -v -race
```
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add service/daemon.go
git commit -m "feat(service): wire Reporter into DaemonService lifecycle"
```

---

## Task 7: Update `default_user_config.yaml`

**Files:**
- Modify: `config/default_user_config.yaml`

- [ ] **Step 1: Append the `reporter:` block**

Open `/root/devnet/sichek2/config/default_user_config.yaml` and add, after the existing `snapshot:` block (around line 7):
```yaml

reporter:
  enable: false  # master switch; flip to true after deploying sichek-collector
  endpoint: "http://sichek-collector.monitoring.svc:38080/api/v1/snapshots"
  interval: 60s
  timeout: 30s
  retry_max: 3
  gzip: true     # keep true unless gzip cannot be decoded upstream
```

Important: if you edit this file inline from the reporter block, keep `gzip: true` explicit so partial overrides (see Task 2 merge note) don't silently turn it off.

- [ ] **Step 2: Validate YAML parses**

```bash
python3 -c "import yaml; yaml.safe_load(open('/root/devnet/sichek2/config/default_user_config.yaml'))" && echo OK
```
Expected: `OK`.

- [ ] **Step 3: End-to-end go test**

```bash
cd /root/devnet/sichek2
go test ./...
```
Expected: PASS across the board.

- [ ] **Step 4: Commit**

```bash
git add config/default_user_config.yaml
git commit -m "feat(config): add reporter block (disabled by default)"
```

---

## Task 8: Manual end-to-end verification against real collector

This task is a manual smoke test and is **not required for code review to pass**, but it is required before enabling in any real cluster.

Prerequisites:
- `sichek-collector` image built from the sister plan and deployed, or run locally via `docker run`.

- [ ] **Step 1: Launch collector locally**

```bash
# In the sichek-collector repo:
make build
mkdir -p /tmp/coll-data
DATA_DIR=/tmp/coll-data LISTEN_ADDR=:38080 ENABLE_READ_API=true \
  ./bin/sichek-collector &
COLL_PID=$!
```

- [ ] **Step 2: Create a fake snapshot and run a one-shot Reporter**

```bash
cd /root/devnet/sichek2
cat > /tmp/test_snap.json <<'EOF'
{"node":"dev-laptop","timestamp":"2026-04-23T10:00:00Z","components":{"nvidia":{"ok":true}}}
EOF

go run -tags=manual - <<'EOF'
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/scitix/sichek/service"
)

func main() {
	cfg := service.ReporterConfig{
		Enable: true, Endpoint: "http://127.0.0.1:38080/api/v1/snapshots",
		Interval: 60 * time.Second, Timeout: 5 * time.Second,
		RetryMax: 3, Gzip: true,
	}
	r := service.NewReporter(cfg, "/tmp/test_snap.json", "dev-laptop")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	r.Run(ctx) // fires one immediate push, then exits on timeout
	fmt.Println("done")
}
EOF
```

- [ ] **Step 3: Verify collector stored the file**

```bash
ls /tmp/coll-data/
cat /tmp/coll-data/dev-laptop.json
```
Expected: `dev-laptop.json` present, contents match `/tmp/test_snap.json`.

- [ ] **Step 4: Verify GET returns NDJSON line**

```bash
curl -sS http://127.0.0.1:38080/api/v1/snapshots
```
Expected: one line starting with `{"node":"dev-laptop","snapshot":{...}}`.

- [ ] **Step 5: Cleanup**

```bash
kill $COLL_PID
rm -rf /tmp/coll-data /tmp/test_snap.json
```

---

## Task 9: Push branch & open PR

- [ ] **Step 1: Sanity check diff is scoped to reporter**

```bash
cd /root/devnet/sichek2
git log --oneline main..HEAD
git diff --stat main..HEAD
```
Expected files changed: `service/reporter.go`, `service/reporter_test.go`, `service/daemon.go`, `config/default_user_config.yaml`, `docs/superpowers/specs/2026-04-23-sichek-collector-design.md`, `docs/superpowers/plans/2026-04-23-*.md`. Nothing else.

- [ ] **Step 2: Push branch**

Ask the user to confirm before pushing:
> "Ready to push branch `feat/sichek-collector` to origin and open a PR. Proceed?"

On approval:
```bash
git push -u origin feat/sichek-collector
```

- [ ] **Step 3: Open PR**

```bash
gh pr create --title "feat(service): sichek daemon snapshot reporter" --body "$(cat <<'EOF'
## Summary
- Adds `service/reporter.go` — periodic HTTP POST of snapshot.json to `sichek-collector`
- Integrates Reporter goroutine into `DaemonService` lifecycle
- Adds `reporter:` config block to `default_user_config.yaml` (disabled by default)

Companion: new `sichek-collector` app in its own repo receives POSTs and
serves the data for the external analysis service.

## Spec + Plan
- Design: `docs/superpowers/specs/2026-04-23-sichek-collector-design.md`
- Plan (reporter): `docs/superpowers/plans/2026-04-23-sichek-reporter-module.md`
- Plan (collector app): `docs/superpowers/plans/2026-04-23-sichek-collector-app.md`

## Test plan
- [ ] `go test ./service/...` passes with `-race`
- [ ] Manual E2E against locally-running sichek-collector (see plan Task 8)
- [ ] Deploy to staging cluster, flip `reporter.enable: true`, verify collector
      PVC contains one file per node within one interval (~60s)
EOF
)"
```

---

## Done Criteria

- [ ] `go test ./service/... -race -count=1` passes
- [ ] `go build ./...` succeeds
- [ ] Running daemon with `reporter.enable: false` behaves identically to before (no new goroutines, no network calls)
- [ ] Running daemon with `reporter.enable: true` pointed at a test collector lands snapshot bytes in PVC within one interval
- [ ] Config changes load correctly (defaults apply when block is absent; overrides apply when set)
- [ ] No new third-party dependencies added to `go.mod`
