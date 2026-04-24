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
	"os"
	"time"

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
