# sysinfo Collector Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a config-driven `sysinfo` component that fetches OSS-hosted KV shell scripts, runs them as root, and folds their `key=value` output into `snapshot.json` under `components.sysinfo.sources.<name>`.

**Architecture:** One generic engine component under `components/sysinfo/`. A config list (`sysinfo.sources[]`) declares N scripts; the engine downloads+executes+parses each on its own interval via one goroutine per source, merging results into a composite `common.Info`. The daemon's existing `monitorComponent` picks up `LastInfo()` and writes the snapshot — no snapshot/reporter changes. Adding a future collector is a config edit, not code.

**Tech Stack:** Go 1.23, cobra CLI, `sigs.k8s.io/yaml` (via `common.LoadUserConfig`), `testify` + `httptest`, `bash`/`exec`.

## Global Constraints

- Module path `github.com/scitix/sichek`; Go 1.23; dependencies vendored (`go mod tidy && go mod vendor` if any dep is added — none expected).
- Every new `.go` file MUST carry the Apache-2.0 Scitix copyright header (see repo `CLAUDE.md`).
- Collection-only: never write the K8s annotation, never emit Prometheus metrics, never flip a `common.Result` to abnormal. Collection failure lives only in a source's `status`/`error`.
- Read-only contract: no system mutation beyond writing/deleting the component's own temp script file.
- Component name / snapshot key / CLI verb: `sysinfo` (constant `consts.ComponentNameSysinfo`).
- First source: name `os_config`, path `scripts/os/collect-config.sh`. Script output contract: one `key=value` per line, split on the **first** `=`.
- Test conventions: table-driven, `testify/assert`+`require`, `t.TempDir()` for isolation.

---

### Task 1: Config package + consts

**Files:**
- Modify: `consts/consts.go` (add `ComponentNameSysinfo`, defaults, script-base fallback; add to `DefaultComponents`)
- Create: `components/sysinfo/config/config.go`
- Test: `components/sysinfo/config/config_test.go`

**Interfaces:**
- Consumes: `common.Duration`, `common.LoadUserConfig`, `httpclient.GetSichekSpecURL`, `consts.*`.
- Produces:
  - `config.SysinfoUserConfig` (implements `common.ComponentUserConfig`), field `Sysinfo *SysinfoConfig`.
  - `config.SysinfoConfig{Enable *bool; BaseURL string; QueryInterval, Timeout common.Duration; Sources []SourceSpec}` with `func (c *SysinfoConfig) Enabled() bool` (nil→true).
  - `config.SourceSpec{Name, Path, URL string; Interval, Timeout *common.Duration; Enable *bool}`.
  - `func NewSysinfoUserConfig(cfgFile string) (*SysinfoUserConfig, error)` — loads + applies defaults; `Sysinfo` never nil.
  - `func (c *SysinfoConfig) ResolvedURL(s SourceSpec) string`
  - `func (c *SysinfoConfig) SourceInterval(s SourceSpec) time.Duration`
  - `func (c *SysinfoConfig) SourceTimeout(s SourceSpec) time.Duration`
  - `func (c *SysinfoConfig) SourceEnabled(s SourceSpec) bool`
  - consts: `ComponentNameSysinfo`, `DefaultSysinfoQueryInterval`, `DefaultSysinfoTimeout`, `DefaultSysinfoScriptPath`, `DomesticScriptBaseURL`.

- [ ] **Step 1: Add consts**

In `consts/consts.go`, add `ComponentNameSysinfo = "sysinfo"` next to the other `ComponentName*` string consts, add `ComponentNameSysinfo` to the `DefaultComponents` slice, and add this block near the other `Default*`/OSS-URL consts:

```go
// sysinfo (OS/host KV-script collector) defaults
const (
	DefaultSysinfoQueryInterval = 24 * time.Hour
	DefaultSysinfoTimeout       = 60 * time.Second
	DefaultSysinfoScriptPath    = "scripts/os/collect-config.sh"
	// DomesticScriptBaseURL is the last-resort base when SICHEK_SPEC_URL is
	// unavailable; it is DomesticSpecURL with the trailing "/specs" stripped.
	DomesticScriptBaseURL = "https://oss-cn-shanghai-2.siflow.cn/hisys:hisys-sichek-sh"
)
```

Ensure `DefaultComponents` now reads (append `ComponentNameSysinfo` at the end):

```go
DefaultComponents = []string{
	ComponentNameCPU, ComponentNameNvidia, ComponentNameInfiniband, ComponentNameEthernet, ComponentNameGpfs, ComponentNameDmesg,
	ComponentNamePodlog, ComponentNameGpuEvents, ComponentNameSyslog, ComponentNameTransceiver, ComponentNameLLDP, ComponentNameSysinfo,
}
```

- [ ] **Step 2: Write the failing config test**

Create `components/sysinfo/config/config_test.go`:

```go
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
func boolp(b bool) *bool { return &b }

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
	// A partial sysinfo: section that omits `enable` must NOT disable the
	// component: Enable stays nil → Enabled() reports true.
	c := &SysinfoConfig{}
	assert.True(t, c.Enabled())
	assert.False(t, (&SysinfoConfig{Enable: boolp(false)}).Enabled())
	assert.True(t, (&SysinfoConfig{Enable: boolp(true)}).Enabled())
}

func TestBaseURLFromEnv(t *testing.T) {
	t.Setenv("SICHEK_SYSINFO_BASE_URL", "https://env.example/base/")
	c := &SysinfoConfig{} // no config BaseURL → env tier wins over derived/fallback
	assert.Equal(t, "https://env.example/base/scripts/os/collect-config.sh",
		c.ResolvedURL(SourceSpec{Path: "scripts/os/collect-config.sh"}))
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./components/sysinfo/config/ -v`
Expected: FAIL — package/types undefined (`SysinfoConfig`, `SourceSpec`, methods).

- [ ] **Step 4: Write the config implementation**

Create `components/sysinfo/config/config.go`:

```go
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

func (c *SysinfoUserConfig) GetQueryInterval() common.Duration    { return c.Sysinfo.QueryInterval }
func (c *SysinfoUserConfig) SetQueryInterval(d common.Duration)    { c.Sysinfo.QueryInterval = d }

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

func boolPtr(b bool) *bool { return &b }

func defaultSysinfoConfig() *SysinfoConfig {
	return &SysinfoConfig{
		Enable:        boolPtr(true),
		QueryInterval: common.Duration{Duration: consts.DefaultSysinfoQueryInterval},
		Timeout:       common.Duration{Duration: consts.DefaultSysinfoTimeout},
		Sources:       []SourceSpec{{Name: "os_config", Path: consts.DefaultSysinfoScriptPath}},
	}
}

// Enabled reports whether the engine is enabled, defaulting to true when the
// config omits the `enable` key entirely (a partial `sysinfo:` section that
// sets only base_url/sources must not silently disable the component).
func (c *SysinfoConfig) Enabled() bool {
	if c.Enable != nil {
		return *c.Enable
	}
	return true
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
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./components/sysinfo/config/ -v`
Expected: PASS (3 tests).

- [ ] **Step 6: Commit**

```bash
git add consts/consts.go components/sysinfo/config/
git commit -m "feat(sysinfo): config package + consts for KV-script collector"
```

---

### Task 2: Collector (one script → SourceResult)

**Files:**
- Create: `components/sysinfo/collector/collector.go`
- Test: `components/sysinfo/collector/collector_test.go`

**Interfaces:**
- Consumes: nothing from Task 1 (independent).
- Produces:
  - `collector.SourceResult{Raw map[string]string; Source, Status, Error string; KeyCount int; CollectedAt time.Time}` with JSON tags `raw/source/status/error,omitempty/key_count/collected_at`.
  - `const StatusOK = "ok"`, `const StatusFailed = "failed"`.
  - `func Collect(ctx context.Context, name, url string, timeout time.Duration) *SourceResult`
  - `func parseKV(out string) map[string]string` (unexported; tested white-box).

- [ ] **Step 1: Write the failing collector test**

Create `components/sysinfo/collector/collector_test.go`:

```go
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
package collector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func serveScript(t *testing.T, body string, status int) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != 0 {
			w.WriteHeader(status)
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestParseKV(t *testing.T) {
	m := parseKV("a=1\n\nb=x=y\nnoeq\nc=NA\n")
	assert.Equal(t, "1", m["a"])
	assert.Equal(t, "x=y", m["b"]) // split on first = only
	assert.Equal(t, "NA", m["c"])
	_, ok := m["noeq"]
	assert.False(t, ok) // lines without = are skipped
	assert.Len(t, m, 3)
}

func TestCollectOK(t *testing.T) {
	url := serveScript(t, "#!/usr/bin/env bash\necho 'kernel.release=5.15'\necho 'x.y=1'\n", 0)
	res := Collect(context.Background(), "os_config", url, 10*time.Second)
	assert.Equal(t, StatusOK, res.Status)
	assert.Equal(t, url, res.Source)
	assert.Equal(t, "5.15", res.Raw["kernel.release"])
	assert.Equal(t, 2, res.KeyCount)
	assert.Empty(t, res.Error)
}

func TestCollectScriptNonZeroExit(t *testing.T) {
	url := serveScript(t, "#!/usr/bin/env bash\necho 'oops' >&2\nexit 1\n", 0)
	res := Collect(context.Background(), "os_config", url, 10*time.Second)
	assert.Equal(t, StatusFailed, res.Status)
	assert.NotEmpty(t, res.Error)
	assert.Empty(t, res.Raw)
}

func TestCollectDownloadFailure(t *testing.T) {
	url := serveScript(t, "nope", http.StatusServiceUnavailable)
	res := Collect(context.Background(), "os_config", url, 10*time.Second)
	assert.Equal(t, StatusFailed, res.Status)
	assert.Contains(t, res.Error, "download")
}

func TestCollectTimeout(t *testing.T) {
	url := serveScript(t, "#!/usr/bin/env bash\nsleep 5\necho 'a=1'\n", 0)
	res := Collect(context.Background(), "os_config", url, 200*time.Millisecond)
	assert.Equal(t, StatusFailed, res.Status)
	assert.Contains(t, res.Error, "timeout")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./components/sysinfo/collector/ -v`
Expected: FAIL — undefined `parseKV`, `Collect`, `SourceResult`, `StatusOK`.

- [ ] **Step 3: Write the collector implementation**

Create `components/sysinfo/collector/collector.go`:

```go
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
package collector

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	StatusOK     = "ok"
	StatusFailed = "failed"
)

// SourceResult is one script's collected output; it is JSON-serialized into
// snapshot.json under components.sysinfo.sources.<name>.
type SourceResult struct {
	Raw         map[string]string `json:"raw"`
	Source      string            `json:"source"`
	Status      string            `json:"status"`
	Error       string            `json:"error,omitempty"`
	KeyCount    int               `json:"key_count"`
	CollectedAt time.Time         `json:"collected_at"`
}

// Collect downloads the script at url, executes it with bash under a timeout,
// and parses its key=value stdout. Every failure mode yields Status=failed with
// an Error and an empty Raw — it never returns nil and never panics.
func Collect(ctx context.Context, name, url string, timeout time.Duration) *SourceResult {
	res := &SourceResult{Raw: map[string]string{}, Source: url, Status: StatusOK, CollectedAt: time.Now()}

	body, err := download(ctx, url, timeout)
	if err != nil {
		return fail(res, fmt.Sprintf("download: %v", err))
	}

	tmp, err := os.CreateTemp("", "sichek-"+sanitize(name)+"-*.sh")
	if err != nil {
		return fail(res, fmt.Sprintf("tempfile: %v", err))
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return fail(res, fmt.Sprintf("write temp: %v", err))
	}
	tmp.Close()

	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, "bash", tmpPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if cctx.Err() == context.DeadlineExceeded {
		return fail(res, "timeout")
	}
	if err != nil {
		return fail(res, fmt.Sprintf("exec: %v: %s", err, strings.TrimSpace(stderr.String())))
	}

	res.Raw = parseKV(stdout.String())
	res.KeyCount = len(res.Raw)
	return res
}

func download(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// parseKV splits each non-blank line on the FIRST '='. Lines without '=' are
// skipped. Duplicate keys: last wins.
func parseKV(out string) map[string]string {
	m := make(map[string]string)
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		i := strings.Index(line, "=")
		if i < 0 {
			continue
		}
		m[line[:i]] = line[i+1:]
	}
	return m
}

func fail(res *SourceResult, msg string) *SourceResult {
	res.Status = StatusFailed
	res.Error = msg
	res.Raw = map[string]string{}
	res.KeyCount = 0
	return res
}

func sanitize(name string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			return r
		default:
			return '_'
		}
	}, name)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./components/sysinfo/collector/ -v`
Expected: PASS (5 tests). Requires `bash` on PATH (present on Linux CI).

- [ ] **Step 5: Commit**

```bash
git add components/sysinfo/collector/
git commit -m "feat(sysinfo): collector — download, exec, parse one KV script"
```

---

### Task 3: Component (per-source engine)

**Files:**
- Create: `components/sysinfo/sysinfo.go`
- Test: `components/sysinfo/sysinfo_test.go`

**Interfaces:**
- Consumes: `config.SysinfoUserConfig`/`SysinfoConfig`/`SourceSpec` and their methods (Task 1); `collector.SourceResult`/`Collect`/`StatusOK` (Task 2).
- Produces:
  - `sysinfo.SysinfoOutput{Sources map[string]*collector.SourceResult}` implementing `common.Info`.
  - `func NewComponent(cfgFile, specFile string) (common.Component, error)` (specFile ignored; matches factory call).
  - `func CollectOne(cfgFile, name string) (*collector.SourceResult, error)` for the CLI `--source` path.

- [ ] **Step 1: Write the failing component test**

Create `components/sysinfo/sysinfo_test.go`:

```go
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
package sysinfo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/sysinfo/collector"
	"github.com/scitix/sichek/components/sysinfo/config"
	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func serveScript(t *testing.T, body string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func bp(b bool) *bool { return &b }

func newTestComponent(cfg *config.SysinfoConfig) *component {
	ctx, cancel := context.WithCancel(context.Background())
	return &component{
		ctx:           ctx,
		cancel:        cancel,
		componentName: consts.ComponentNameSysinfo,
		cfg:           &config.SysinfoUserConfig{Sysinfo: cfg},
		outputs:       map[string]*collector.SourceResult{},
		resultCh:      make(chan *common.Result),
	}
}

func TestHealthCheckRunsAllSources(t *testing.T) {
	u1 := serveScript(t, "a=1\n")
	u2 := serveScript(t, "b=2\n")
	c := newTestComponent(&config.SysinfoConfig{
		Enable:  bp(true),
		Timeout: common.Duration{Duration: 10 * time.Second},
		Sources: []config.SourceSpec{{Name: "s1", URL: u1}, {Name: "s2", URL: u2}},
	})
	_, err := c.HealthCheck(context.Background())
	require.NoError(t, err)
	info, err := c.LastInfo()
	require.NoError(t, err)
	out := info.(*SysinfoOutput)
	assert.Equal(t, collector.StatusOK, out.Sources["s1"].Status)
	assert.Equal(t, "1", out.Sources["s1"].Raw["a"])
	assert.Equal(t, "2", out.Sources["s2"].Raw["b"])
}

func TestOneSourceFailureIsolated(t *testing.T) {
	good := serveScript(t, "a=1\n")
	c := newTestComponent(&config.SysinfoConfig{
		Enable:  bp(true),
		Timeout: common.Duration{Duration: 10 * time.Second},
		Sources: []config.SourceSpec{
			{Name: "good", URL: good},
			{Name: "bad", URL: "http://127.0.0.1:1/nope.sh"},
		},
	})
	_, err := c.HealthCheck(context.Background())
	require.NoError(t, err)
	out := mustInfo(t, c)
	assert.Equal(t, collector.StatusOK, out.Sources["good"].Status)
	assert.Equal(t, collector.StatusFailed, out.Sources["bad"].Status)
}

func TestStartEmitsAndStops(t *testing.T) {
	u1 := serveScript(t, "a=1\n")
	c := newTestComponent(&config.SysinfoConfig{
		Enable:        bp(true),
		Timeout:       common.Duration{Duration: 10 * time.Second},
		QueryInterval: common.Duration{Duration: time.Hour}, // ticker won't fire during test
		Sources:       []config.SourceSpec{{Name: "s1", URL: u1}},
	})
	ch := c.Start()
	select {
	case res := <-ch: // immediate run emits a benign result
		assert.Equal(t, consts.StatusNormal, res.Status)
	case <-time.After(3 * time.Second):
		t.Fatal("no result emitted on startup")
	}
	out := mustInfo(t, c)
	assert.Equal(t, "1", out.Sources["s1"].Raw["a"])
	require.NoError(t, c.Stop())
}

func TestDisabledSkipsCollection(t *testing.T) {
	c := newTestComponent(&config.SysinfoConfig{
		Enable:  bp(false),
		Timeout: common.Duration{Duration: time.Second},
		Sources: []config.SourceSpec{{Name: "s1", URL: serveScript(t, "a=1\n")}},
	})
	_, err := c.HealthCheck(context.Background())
	require.NoError(t, err)
	assert.Empty(t, mustInfo(t, c).Sources)
}

func mustInfo(t *testing.T, c *component) *SysinfoOutput {
	t.Helper()
	info, err := c.LastInfo()
	require.NoError(t, err)
	return info.(*SysinfoOutput)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./components/sysinfo/ -v`
Expected: FAIL — undefined `component`, `SysinfoOutput`, methods.

- [ ] **Step 3: Write the component implementation**

Create `components/sysinfo/sysinfo.go`:

```go
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
package sysinfo

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/sysinfo/collector"
	"github.com/scitix/sichek/components/sysinfo/config"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

// SysinfoOutput is the component's snapshot payload: one SourceResult per source.
type SysinfoOutput struct {
	Sources map[string]*collector.SourceResult `json:"sources"`
}

func (o *SysinfoOutput) JSON() (string, error) {
	data, err := json.Marshal(o)
	return string(data), err
}

type component struct {
	ctx           context.Context
	cancel        context.CancelFunc
	componentName string

	cfg      *config.SysinfoUserConfig
	cfgMutex sync.Mutex

	outputs    map[string]*collector.SourceResult
	outputsMtx sync.RWMutex

	resultCh chan *common.Result

	// source-goroutine lifecycle
	srcCancel context.CancelFunc
	srcWG     sync.WaitGroup

	runMtx  sync.Mutex
	running bool
}

var (
	sysinfoComponent *component
	sysinfoOnce      sync.Once
)

func NewComponent(cfgFile string, specFile string) (common.Component, error) {
	var err error
	sysinfoOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic occurred when create component sysinfo: %v", r)
			}
		}()
		sysinfoComponent, err = newComponent(cfgFile)
	})
	return sysinfoComponent, err
}

func newComponent(cfgFile string) (*component, error) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg, err := config.NewSysinfoUserConfig(cfgFile)
	if err != nil {
		cancel()
		return nil, err
	}
	return &component{
		ctx:           ctx,
		cancel:        cancel,
		componentName: consts.ComponentNameSysinfo,
		cfg:           cfg,
		outputs:       make(map[string]*collector.SourceResult),
		resultCh:      make(chan *common.Result),
	}, nil
}

func (c *component) Name() string { return c.componentName }

func (c *component) GetTimeout() time.Duration {
	c.cfgMutex.Lock()
	defer c.cfgMutex.Unlock()
	return c.cfg.Sysinfo.QueryInterval.Duration
}

// HealthCheck runs every enabled source synchronously and returns a benign
// result. Used by the one-shot CLI path (RunComponentCheck).
func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	c.cfgMutex.Lock()
	sc := c.cfg.Sysinfo
	c.cfgMutex.Unlock()
	if sc.Enabled() {
		for _, src := range sc.Sources {
			if !sc.SourceEnabled(src) {
				continue
			}
			c.collectAndStore(ctx, src.Name, sc.ResolvedURL(src), sc.SourceTimeout(src))
		}
	}
	return c.benignResult(), nil
}

func (c *component) collectAndStore(ctx context.Context, name, url string, timeout time.Duration) {
	res := collector.Collect(ctx, name, url, timeout)
	c.outputsMtx.Lock()
	c.outputs[name] = res
	c.outputsMtx.Unlock()
	if res.Status != collector.StatusOK {
		logrus.WithField("component", "sysinfo").Warnf("source %q collect failed: %s", name, res.Error)
	}
}

func (c *component) benignResult() *common.Result {
	return &common.Result{
		Item:   c.componentName,
		Status: consts.StatusNormal,
		Level:  consts.LevelInfo,
		Time:   time.Now(),
	}
}

// Start spawns one goroutine per enabled source: run immediately, then loop on
// the source's own interval. Each run emits a benign result on the channel so
// the daemon picks up LastInfo() and updates the snapshot.
func (c *component) Start() <-chan *common.Result {
	c.runMtx.Lock()
	if c.running {
		c.runMtx.Unlock()
		return c.resultCh
	}
	c.running = true
	c.runMtx.Unlock()
	c.startSources()
	return c.resultCh
}

func (c *component) startSources() {
	c.cfgMutex.Lock()
	sc := c.cfg.Sysinfo
	c.cfgMutex.Unlock()

	sctx, scancel := context.WithCancel(c.ctx)
	c.srcCancel = scancel
	if !sc.Enabled() {
		return
	}
	for _, src := range sc.Sources {
		if !sc.SourceEnabled(src) {
			continue
		}
		c.srcWG.Add(1)
		go c.runSource(sctx, sc, src)
	}
}

func (c *component) runSource(ctx context.Context, sc *config.SysinfoConfig, src config.SourceSpec) {
	defer c.srcWG.Done()
	defer func() {
		if r := recover(); r != nil {
			logrus.WithField("component", "sysinfo").Errorf("panic in source %q: %v", src.Name, r)
		}
	}()
	url := sc.ResolvedURL(src)
	timeout := sc.SourceTimeout(src)
	ticker := time.NewTicker(sc.SourceInterval(src))
	defer ticker.Stop()

	// immediate first run
	c.collectAndStore(ctx, src.Name, url, timeout)
	if !c.send(ctx) {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.collectAndStore(ctx, src.Name, url, timeout)
			if !c.send(ctx) {
				return
			}
		}
	}
}

// send delivers a benign result unless the context is cancelled; the ctx guard
// prevents a send on a closed channel during Stop().
func (c *component) send(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case c.resultCh <- c.benignResult():
		return true
	}
}

func (c *component) Stop() error {
	if c.srcCancel != nil {
		c.srcCancel()
	}
	c.cancel()
	c.srcWG.Wait()
	c.runMtx.Lock()
	if c.running {
		close(c.resultCh)
		c.running = false
	}
	c.runMtx.Unlock()
	return nil
}

// Update swaps config and, if running, cancels + respawns the source goroutines
// so enable/disable, added/removed sources, and interval changes take effect
// without a restart.
func (c *component) Update(cfg common.ComponentUserConfig) error {
	newCfg, ok := cfg.(*config.SysinfoUserConfig)
	if !ok {
		return fmt.Errorf("update wrong config type for sysinfo")
	}
	if c.srcCancel != nil {
		c.srcCancel()
	}
	c.srcWG.Wait()
	c.cfgMutex.Lock()
	c.cfg = newCfg
	c.cfgMutex.Unlock()
	c.runMtx.Lock()
	running := c.running
	c.runMtx.Unlock()
	if running {
		c.startSources()
	}
	return nil
}

func (c *component) Status() bool {
	c.runMtx.Lock()
	defer c.runMtx.Unlock()
	return c.running
}

func (c *component) LastInfo() (common.Info, error) {
	c.outputsMtx.RLock()
	defer c.outputsMtx.RUnlock()
	cp := make(map[string]*collector.SourceResult, len(c.outputs))
	for k, v := range c.outputs {
		cp[k] = v
	}
	return &SysinfoOutput{Sources: cp}, nil
}

func (c *component) CacheInfos() ([]common.Info, error) {
	info, _ := c.LastInfo()
	return []common.Info{info}, nil
}

func (c *component) CacheResults() ([]*common.Result, error) {
	return []*common.Result{c.benignResult()}, nil
}

func (c *component) LastResult() (*common.Result, error) {
	return c.benignResult(), nil
}

func (c *component) Metrics(ctx context.Context, since time.Time) (interface{}, error) {
	return nil, nil
}

// PrintInfo prints each source's status (and full KV when summaryPrint). It
// always returns true: this component makes no health verdict.
func (c *component) PrintInfo(info common.Info, result *common.Result, summaryPrint bool) bool {
	out, ok := info.(*SysinfoOutput)
	if !ok {
		logrus.WithField("component", "sysinfo").Errorf("invalid data type, expected *SysinfoOutput")
		return false
	}
	names := make([]string, 0, len(out.Sources))
	for name := range out.Sources {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		src := out.Sources[name]
		fmt.Printf("sysinfo source %q: status=%s keys=%d source=%s\n", name, src.Status, src.KeyCount, src.Source)
		if src.Status != collector.StatusOK {
			fmt.Printf("  error: %s\n", src.Error)
		}
		if summaryPrint {
			keys := make([]string, 0, len(src.Raw))
			for k := range src.Raw {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Printf("  %s=%s\n", k, src.Raw[k])
			}
		}
	}
	return true
}

// CollectOne loads config, runs a single named source once, and returns its
// result. Used by the CLI `--source` flag.
func CollectOne(cfgFile, name string) (*collector.SourceResult, error) {
	cfg, err := config.NewSysinfoUserConfig(cfgFile)
	if err != nil {
		return nil, err
	}
	sc := cfg.Sysinfo
	for _, src := range sc.Sources {
		if src.Name == name {
			return collector.Collect(context.Background(), name, sc.ResolvedURL(src), sc.SourceTimeout(src)), nil
		}
	}
	return nil, fmt.Errorf("no sysinfo source named %q", name)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./components/sysinfo/ -v`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add components/sysinfo/sysinfo.go components/sysinfo/sysinfo_test.go
git commit -m "feat(sysinfo): per-source engine component"
```

---

### Task 4: Wiring (factory, CLI, config, daemon)

**Files:**
- Modify: `cmd/command/component/all.go` (import + switch case + default `-I`)
- Create: `cmd/command/component/sysinfo.go` (CLI subcommand)
- Modify: `cmd/command/command.go` (register subcommand)
- Modify: `config/default_user_config.yaml` (add `sysinfo:` section)

**Interfaces:**
- Consumes: `sysinfo.NewComponent`, `sysinfo.CollectOne`, `consts.ComponentNameSysinfo`, `RunComponentCheck`, `PrintCheckResults`.
- Produces: `func NewSysinfoCmd() *cobra.Command`.

- [ ] **Step 1: Add the factory case + exclude from `all` default**

In `cmd/command/component/all.go`, add the import (in the component import group):

```go
	"github.com/scitix/sichek/components/sysinfo"
```

Add a case to the `NewComponent` switch (before `default:`):

```go
	case consts.ComponentNameSysinfo:
		return sysinfo.NewComponent(cfgFile, specFile)
```

Change the `all` command's default ignore flag from:

```go
	allCmd.Flags().StringVarP(&ignoreComponents, "ignore-components", "I", "podlog,gpuevents,syslog", "Ignored components")
```
to:
```go
	allCmd.Flags().StringVarP(&ignoreComponents, "ignore-components", "I", "podlog,gpuevents,syslog,sysinfo", "Ignored components")
```

- [ ] **Step 2: Create the CLI subcommand**

Create `cmd/command/component/sysinfo.go`:

```go
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
package component

import (
	"context"
	"fmt"

	"github.com/scitix/sichek/components/sysinfo"
	"github.com/scitix/sichek/components/sysinfo/collector"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewSysinfoCmd() *cobra.Command {
	var source string
	sysinfoCmd := &cobra.Command{
		Use:     "sysinfo",
		Aliases: []string{"si"},
		Short:   "Collect host/OS configuration via OSS KV scripts",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), consts.CmdTimeout)
			defer cancel()
			verbos, _ := cmd.Flags().GetBool("verbos")
			if !verbos {
				logrus.SetLevel(logrus.ErrorLevel)
			}
			cfgFile, _ := cmd.Flags().GetString("cfg")

			// Single-source path.
			if source != "" {
				res, err := sysinfo.CollectOne(cfgFile, source)
				if err != nil {
					logrus.WithField("component", "sysinfo").Error(err)
					return
				}
				printSource(source, res, verbos)
				return
			}

			// All-sources path (mirrors other component subcommands).
			comp, err := sysinfo.NewComponent(cfgFile, "")
			if err != nil {
				logrus.WithField("component", "sysinfo").Error(err)
				return
			}
			result, err := RunComponentCheck(ctx, comp, consts.CmdTimeout)
			if err != nil {
				return
			}
			PrintCheckResults(true, result)
		},
	}
	sysinfoCmd.Flags().StringP("cfg", "c", "", "Path to the user config file")
	sysinfoCmd.Flags().StringVar(&source, "source", "", "Run only the named source")
	sysinfoCmd.Flags().BoolP("verbos", "v", false, "Enable verbose output (dump all key=value)")
	return sysinfoCmd
}

func printSource(name string, res *collector.SourceResult, verbose bool) {
	fmt.Printf("sysinfo source %q: status=%s keys=%d source=%s\n", name, res.Status, res.KeyCount, res.Source)
	if res.Status != collector.StatusOK {
		fmt.Printf("  error: %s\n", res.Error)
		return
	}
	if verbose {
		for k, v := range res.Raw {
			fmt.Printf("  %s=%s\n", k, v)
		}
	}
}
```

- [ ] **Step 3: Register the subcommand**

In `cmd/command/command.go`, add next to the other `rootCmd.AddCommand(component.New...Cmd())` lines:

```go
	rootCmd.AddCommand(component.NewSysinfoCmd())
```

- [ ] **Step 4: Add the config section**

In `config/default_user_config.yaml`, append a top-level section (match the file's existing 2-space indentation style):

```yaml
sysinfo:
  enable: true
  base_url: ""            # empty → derived from SICHEK_SPEC_URL (strip /specs)
  query_interval: 24h
  timeout: 60s
  sources:
    - name: os_config
      path: scripts/os/collect-config.sh
    # future collectors: append entries, e.g.
    # - name: gpu_config
    #   path: scripts/gpu/collect-gpu.sh
    #   interval: 12h
```

- [ ] **Step 5: Build and smoke-test the CLI**

Run:
```bash
go build ./... && go run ./cmd sysinfo --source os_config -v
```
Expected: builds; prints `sysinfo source "os_config": status=... keys=... source=https://.../scripts/os/collect-config.sh`. On a non-root/offline dev box it prints `status=failed` with an error — that is correct behavior, not a plan failure. `go run ./cmd --help` lists `sysinfo`.

- [ ] **Step 6: Commit**

```bash
git add cmd/command/component/all.go cmd/command/component/sysinfo.go cmd/command/command.go config/default_user_config.yaml
git commit -m "feat(sysinfo): wire factory, CLI subcommand, and default config"
```

---

### Task 5: Docs + full verification

**Files:**
- Modify: `docs/write-operations.md` (record the root script-exec)

**Interfaces:** none.

- [ ] **Step 1: Document the write/exec operation**

Append a section to `docs/write-operations.md` noting: the `sysinfo` component downloads and executes, as root, the shell scripts listed in `sysinfo.sources`; no checksum is verified; the trust boundary is the same OSS host/HTTPS as spec downloads; the `sources` list is effectively an allowlist of root-executed URLs and edits to it are privileged. (Match the file's existing heading/format.)

- [ ] **Step 2: Run the full test + vet + build**

Run:
```bash
go vet ./... && go test ./components/sysinfo/... -v && go build ./...
```
Expected: vet clean; all sysinfo tests PASS; build succeeds.

- [ ] **Step 3: Commit**

```bash
git add docs/write-operations.md
git commit -m "docs(sysinfo): record root script-exec in write-operations"
```

---

## Notes for the implementer

- **Root & bash required at runtime.** `collect-config.sh` exits 1 if not root; the daemon (systemd/DaemonSet) runs as root, so production is fine. A non-root `sichek sysinfo` will show `status=failed` — expected.
- **No snapshot/reporter/annotation code changes.** The daemon's `monitorComponent` already calls `LastInfo()` → `snapshotMgr.Update("sysinfo", info)`; the section flows to the downstream project via the existing snapshot POST. Verified: `service/info.go setAnnotationsByItem` is a silent no-op for the unknown `sysinfo` item, so no annotation is written.
- **Adding a future collector** = append a `{name, path}` (plus optional `interval`/`timeout`/`enable`) entry to `sysinfo.sources` in config. No Go changes.
- The `sysinfo` component is a `sync.Once` singleton (like `cpu`); `NewComponent` ignores `specFile`.
```
