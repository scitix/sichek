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
	"testing"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/stretchr/testify/assert"
)

func dur(d time.Duration) *common.Duration { return &common.Duration{Duration: d} }
func boolp(b bool) *bool                   { return &b }

func TestResolvedURL(t *testing.T) {
	c := &SysinfoConfig{BaseURL: "https://oss.example/base"}
	// absolute URL escape hatch wins
	assert.Equal(t, "https://abs/x.sh", c.ResolvedURL(SourceSpec{URL: "https://abs/x.sh", Path: "ignored"}))
	// base + path (leading slash on path tolerated, no double slash)
	assert.Equal(t, "https://oss.example/base/scripts/os/collect-config.sh",
		c.ResolvedURL(SourceSpec{Path: "/scripts/os/collect-config.sh"}))
}

func TestPerSourceOverrides(t *testing.T) {
	c := &SysinfoConfig{
		QueryInterval: common.Duration{Duration: 24 * time.Hour},
		Timeout:       common.Duration{Duration: 60 * time.Second},
	}
	// defaults fall through to engine values
	assert.Equal(t, 24*time.Hour, c.SourceInterval(SourceSpec{}))
	assert.Equal(t, 60*time.Second, c.SourceTimeout(SourceSpec{}))
	assert.True(t, c.SourceEnabled(SourceSpec{}))
	// overrides win
	assert.Equal(t, 12*time.Hour, c.SourceInterval(SourceSpec{Interval: dur(12 * time.Hour)}))
	assert.Equal(t, 90*time.Second, c.SourceTimeout(SourceSpec{Timeout: dur(90 * time.Second)}))
	assert.False(t, c.SourceEnabled(SourceSpec{Enable: boolp(false)}))
}

func TestApplyDefaultsFillsZeros(t *testing.T) {
	c := &SysinfoConfig{}
	c.applyDefaults()
	assert.Equal(t, 24*time.Hour, c.QueryInterval.Duration)
	assert.Equal(t, 60*time.Second, c.Timeout.Duration)
}

func TestEnabledDefaultsTrueWhenUnset(t *testing.T) {
	c := &SysinfoConfig{}
	assert.True(t, c.Enabled())
	assert.False(t, (&SysinfoConfig{Enable: boolp(false)}).Enabled())
	assert.True(t, (&SysinfoConfig{Enable: boolp(true)}).Enabled())
}

func TestBaseURLFromEnv(t *testing.T) {
	t.Setenv("SICHEK_SYSINFO_BASE_URL", "https://env.example/base/")
	c := &SysinfoConfig{}
	assert.Equal(t, "https://env.example/base/scripts/os/collect-config.sh",
		c.ResolvedURL(SourceSpec{Path: "scripts/os/collect-config.sh"}))
}
