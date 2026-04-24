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
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
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
	data, err := os.ReadFile(cfgFile)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return ReporterConfig{}, fmt.Errorf("load reporter config: %w", err)
	}
	f := reporterFile{Reporter: cfg}
	if err := yaml.Unmarshal(data, &f); err != nil {
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
	return out
}

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
