/*
Copyright 2024 The Scitix Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0
*/
package service

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
	r.backoff = func(i int) time.Duration { return 0 }

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
	r.Run(ctx)
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
