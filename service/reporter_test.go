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
