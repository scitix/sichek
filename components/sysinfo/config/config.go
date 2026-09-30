/*
Copyright 2024 The Scitix Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package config

import (
	"os"
	"strings"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/httpclient"
)

// SysinfoUserConfig is the user-config container for the sysinfo component.
type SysinfoUserConfig struct {
	Sysinfo *SysinfoConfig `json:"sysinfo" yaml:"sysinfo"`
}

// SysinfoConfig holds engine-level defaults plus the data-driven source list.
type SysinfoConfig struct {
	Enable        *bool           `json:"enable"         yaml:"enable,omitempty"`
	BaseURL       string          `json:"base_url"       yaml:"base_url"`
	QueryInterval common.Duration `json:"query_interval" yaml:"query_interval"`
	Timeout       common.Duration `json:"timeout"        yaml:"timeout"`
	Sources       []SourceSpec    `json:"sources"        yaml:"sources"`
}

// SourceSpec declares one KV script. Interval/Timeout/Enable are optional
// per-source overrides of the engine-level defaults.
type SourceSpec struct {
	Name     string           `json:"name"     yaml:"name"`
	Path     string           `json:"path"     yaml:"path"`
	URL      string           `json:"url"      yaml:"url,omitempty"`
	Interval *common.Duration `json:"interval" yaml:"interval,omitempty"`
	Timeout  *common.Duration `json:"timeout"  yaml:"timeout,omitempty"`
	Enable   *bool            `json:"enable"   yaml:"enable,omitempty"`
}

func (c *SysinfoUserConfig) GetQueryInterval() common.Duration  { return c.Sysinfo.QueryInterval }
func (c *SysinfoUserConfig) SetQueryInterval(d common.Duration) { c.Sysinfo.QueryInterval = d }

func boolPtr(b bool) *bool { return &b }

// Enabled reports whether the engine is enabled, defaulting to true when the
// config omits the `enable` key entirely.
func (c *SysinfoConfig) Enabled() bool {
	if c.Enable != nil {
		return *c.Enable
	}
	return true
}

// NewSysinfoUserConfig loads the sysinfo section (file → prod default → dev
// fallback) and guarantees a non-nil Sysinfo with defaults applied.
func NewSysinfoUserConfig(cfgFile string) (*SysinfoUserConfig, error) {
	cfg := &SysinfoUserConfig{}
	if err := common.LoadUserConfig(cfgFile, cfg); err != nil {
		return nil, err
	}
	if cfg.Sysinfo == nil {
		cfg.Sysinfo = defaultSysinfoConfig()
	}
	cfg.Sysinfo.applyDefaults()
	return cfg, nil
}

func defaultSysinfoConfig() *SysinfoConfig {
	return &SysinfoConfig{
		Enable:        boolPtr(true),
		QueryInterval: common.Duration{Duration: consts.DefaultSysinfoQueryInterval},
		Timeout:       common.Duration{Duration: consts.DefaultSysinfoTimeout},
		Sources:       []SourceSpec{{Name: "os_config", Path: consts.DefaultSysinfoScriptPath}},
	}
}

// applyDefaults fills zero-valued engine knobs and applies engine-level env
// overrides. It never clears an explicitly-configured value.
func (c *SysinfoConfig) applyDefaults() {
	if c.QueryInterval.Duration == 0 {
		c.QueryInterval = common.Duration{Duration: consts.DefaultSysinfoQueryInterval}
	}
	if c.Timeout.Duration == 0 {
		c.Timeout = common.Duration{Duration: consts.DefaultSysinfoTimeout}
	}
	if v := os.Getenv("SICHEK_SYSINFO_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.QueryInterval = common.Duration{Duration: d}
		}
	}
	if os.Getenv("SICHEK_SYSINFO_ENABLE") == "false" {
		c.Enable = boolPtr(false)
	}
}

// ResolvedURL returns the absolute script URL for a source.
func (c *SysinfoConfig) ResolvedURL(s SourceSpec) string {
	if s.URL != "" {
		return s.URL
	}
	return c.resolveBaseURL() + "/" + strings.TrimLeft(s.Path, "/")
}

// resolveBaseURL: config base_url → SICHEK_SYSINFO_BASE_URL env →
// region-derived (SICHEK_SPEC_URL minus /specs) → hardcoded domestic fallback.
func (c *SysinfoConfig) resolveBaseURL() string {
	if c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	if env := os.Getenv("SICHEK_SYSINFO_BASE_URL"); env != "" {
		return strings.TrimRight(env, "/")
	}
	if spec := httpclient.GetSichekSpecURL(); spec != "" {
		return strings.TrimSuffix(strings.TrimRight(spec, "/"), "/specs")
	}
	return consts.DomesticScriptBaseURL
}

func (c *SysinfoConfig) SourceInterval(s SourceSpec) time.Duration {
	if s.Interval != nil && s.Interval.Duration > 0 {
		return s.Interval.Duration
	}
	return c.QueryInterval.Duration
}

func (c *SysinfoConfig) SourceTimeout(s SourceSpec) time.Duration {
	if s.Timeout != nil && s.Timeout.Duration > 0 {
		return s.Timeout.Duration
	}
	return c.Timeout.Duration
}

func (c *SysinfoConfig) SourceEnabled(s SourceSpec) bool {
	if s.Enable != nil {
		return *s.Enable
	}
	return true
}
