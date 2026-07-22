# Design: `sysinfo` — config-driven OS/host KV-script collector

- **Date:** 2026-07-13
- **Status:** Approved design, pending implementation plan
- **Component name / snapshot key / CLI:** `sysinfo`
- **Module:** `github.com/scitix/sichek`

## 1. Goal

Embed a **generic KV-script collector engine** into sichek. It fetches
OSS-hosted shell scripts that emit `key=value` records describing host/OS
configuration, runs them, and folds the results into `snapshot.json` for a
**downstream sibling project** to consume.

The first script is `collect-config.sh` (OS config: kernel cmdline, sysctl,
ulimits, kpatch, loaded/blacklisted modules, apt sources/timers, kdump, package
versions). Crucially, the engine is **data-driven**: the set of scripts is a
config list, so **future collectors are added by editing config, not by writing
Go code or recompiling**. `collect-config.sh` is simply the first entry.

This is a **collection-only** feature: it never produces a health verdict, never
writes the K8s node annotation, and never exports Prometheus metrics. The only
failure surface is a per-source `status` field inside its own snapshot section.

### The upstream script output contract (fixed, shared by all such scripts)

`collect-config.sh` documents a contract that all KV scripts under this engine
are expected to follow:

- One `key=value` per line; split on the **first** `=`.
- Missing values recorded as the literal `NA`.
- Newlines inside a value folded to `; ` so every record stays on one line.
- Must run as **root**; otherwise it prints an error to stderr and exits `1`.
- Dotted-namespace keys, unique by design.

First script URL (base auto-resolved by region — see §5):
`https://oss-cn-shanghai-2.siflow.cn/hisys:hisys-sichek-sh/scripts/os/collect-config.sh`

## 2. Confirmed decisions

| Item | Decision |
|---|---|
| Generalization ("写活") | A **single `sysinfo` engine component** runs **N scripts ("sources") declared in config**. Adding a future collector = one YAML entry; zero Go code, no factory-switch edit, no `DefaultComponents` change, no recompile. |
| Script acquisition | **Fetch fresh each run** (download to a temp file, then `bash` it). No persistent cache. On download/exec failure, skip that source's cycle and keep its previous snapshot value. |
| Run cadence / modes | In the daemon each source runs **once immediately on startup, then on its own interval** (default 24h, per-source override). Also exposed as a one-shot `sichek sysinfo` CLI (`--source` to pick one). |
| Snapshot placement | Nested: `components["sysinfo"]["sources"][<name>]` = `{raw, source, status, key_count, collected_at, error?}`. |
| Alerting / metrics | None. A source's collection failure is recorded only as its `status="failed"` + `error`. No annotation, no Prometheus. |
| Everything tunable at runtime | Enable, base URL, per-source URL/path, interval, and exec timeout are all resolved from user config, overridable by environment variables, and hot-reloadable via the daemon `Update()` path. Only last-resort fallbacks are hardcoded. |

## 3. Architecture

Chosen route (confirmed): **one generic engine component** under
`components/sysinfo/`, mirroring the `components/cpu/` layout, but with a
**self-managed loop that runs one goroutine per configured source** instead of
embedding `common.CommonService`.

Why not `CommonService`: its ticker fires the first check only *after* one full
interval (`component.go` `Start()` selects on `ticker.C`) — with a 24h interval
that means no data for a whole day. It also assumes a single cadence, whereas
sources may each want their own. A small custom loop gives us "run immediately
then per-source interval" and re-reads config each cycle for hot-reload.

Why one engine over one-component-per-script: sichek's component factory
(`all.go` switch) and `DefaultComponents` are **compile-time**, so a
config-driven *set of components* would require plumbing. Folding all sources
into one component sidesteps that and makes adding a collector a pure config
change — the strongest form of "写活".

### Directory layout

```
components/sysinfo/
├── sysinfo.go               // implements common.Component + per-source loop
├── collector/
│   └── collector.go         // one script: resolve URL → download → bash → parse KV → *SourceResult
└── config/
    └── config.go            // SysinfoUserConfig: engine defaults + []SourceSpec
```

## 4. Data model

The component's `common.Info` is a composite keyed by source name:

```go
// One script's output.
type SourceResult struct {
    Raw         map[string]string `json:"raw"`             // parsed key=value; empty on failure
    Source      string            `json:"source"`          // resolved script URL
    Status      string            `json:"status"`          // "ok" | "failed"
    Error       string            `json:"error,omitempty"` // set when Status=="failed"
    KeyCount    int               `json:"key_count"`       // len(Raw)
    CollectedAt time.Time         `json:"collected_at"`
}

// The whole sysinfo component's snapshot payload.
type SysinfoOutput struct {
    Sources map[string]*SourceResult `json:"sources"`
}

func (o *SysinfoOutput) JSON() (string, error) { data, err := json.Marshal(o); return string(data), err }
```

Resulting `snapshot.json` (unchanged `SnapshotManager`, no schema edit):

```json
{
  "components": {
    "sysinfo": {
      "sources": {
        "os_config": {
          "raw": { "kernel.release": "5.15.0-...", "ulimit.open_files.hard": "1048576", "apt.source.fingerprint": "…", "...": "..." },
          "source": "https://oss-cn-shanghai-2.siflow.cn/hisys:hisys-sichek-sh/scripts/os/collect-config.sh",
          "status": "ok",
          "key_count": 312,
          "collected_at": "2026-07-13T09:00:00Z"
        }
      }
    }
  }
}
```

Failure of one source is isolated (`raw:{}`, structure preserved) and does not
affect sibling sources:

```json
"os_config": { "raw": {}, "source": "…", "status": "failed",
               "error": "download: GET …: status 503", "key_count": 0,
               "collected_at": "2026-07-13T09:00:00Z" }
```

**Adding a future collector** = one config entry → a new key appears under
`sources` automatically. Downstream consumes `sysinfo.sources.*`.

## 5. Configuration ("写活")

New config type, loaded via `common.LoadUserConfig` like every other component:

```go
type SysinfoUserConfig struct {
    Sysinfo *SysinfoConfig `json:"sysinfo" yaml:"sysinfo"`
}

type SysinfoConfig struct {
    Enable        bool            `json:"enable"         yaml:"enable"`         // default true
    BaseURL       string          `json:"base_url"       yaml:"base_url"`       // default: region-derived (below)
    QueryInterval common.Duration `json:"query_interval" yaml:"query_interval"` // default 24h (engine default cadence)
    Timeout       common.Duration `json:"timeout"        yaml:"timeout"`        // default 60s (engine default exec timeout)
    Sources       []SourceSpec    `json:"sources"        yaml:"sources"`
}

type SourceSpec struct {
    Name     string           `json:"name"     yaml:"name"`               // -> snapshot key; required, unique
    Path     string           `json:"path"     yaml:"path"`               // relative to BaseURL
    URL      string           `json:"url"      yaml:"url,omitempty"`      // absolute escape hatch (overrides Path)
    Interval *common.Duration `json:"interval" yaml:"interval,omitempty"` // per-source cadence override
    Timeout  *common.Duration `json:"timeout"  yaml:"timeout,omitempty"`  // per-source exec-timeout override
    Enable   *bool            `json:"enable"   yaml:"enable,omitempty"`   // per-source toggle, default true
}

func (c *SysinfoUserConfig) GetQueryInterval() common.Duration { return c.Sysinfo.QueryInterval }
func (c *SysinfoUserConfig) SetQueryInterval(d common.Duration) { c.Sysinfo.QueryInterval = d }
```

Example (default_user_config.yaml `sysinfo:` section, seeded with the first source):

```yaml
sysinfo:
  enable: true
  base_url: ""            # empty → region-derived from SICHEK_SPEC_URL (strip /specs)
  query_interval: 24h
  timeout: 60s
  sources:
    - name: os_config
      path: scripts/os/collect-config.sh
    # future collectors: just append entries, e.g.
    # - name: gpu_config
    #   path: scripts/gpu/collect-gpu.sh
    #   interval: 12h
```

**Resolution precedence (highest first) — every knob live-tunable:**

*Per source field* (interval / timeout / enable): source-level override →
engine-level default (`query_interval` / `timeout` / `enable`).

*Script URL for a source*:
1. `SourceSpec.URL` if non-empty (absolute escape hatch).
2. `BaseURL` + `/` + `SourceSpec.Path`, where `BaseURL` is resolved as:
   config `base_url` → env `SICHEK_SYSINFO_BASE_URL` → **region-derived**
   (`httpclient.GetSichekSpecURL()` e.g. `.../hisys:hisys-sichek-sh/specs`, strip
   the trailing `/specs`) → hardcoded fallback constant (domestic base). Region
   handling therefore lives in exactly one place and every source inherits it.

*Engine-level env overrides* (no redeploy): `SICHEK_SYSINFO_ENABLE`
(`false` disables the whole component), `SICHEK_SYSINFO_BASE_URL`,
`SICHEK_SYSINFO_INTERVAL` (engine default cadence).

Defaults (`ComponentNameSysinfo`, 24h interval, 60s timeout, fallback base URL,
the seed `os_config` source) live in `consts/consts.go` / the config package, so
the component works even with no `sysinfo:` section present.

## 6. Collector behaviour (`collector.go`)

The collector operates on **one source** at a time: `Collect(ctx, name, url, timeout) *SourceResult`.

1. `http.Get(url)` with a short timeout (reuse `pkg/httpclient` where possible);
   write the body to `os.CreateTemp("", "sichek-<name>-*.sh")`; `defer os.Remove`.
2. `exec.CommandContext(ctx, "bash", tmpPath)` bounded by `timeout`; capture
   stdout (and stderr for diagnostics).
3. Parse stdout line-by-line: skip blank lines; split each on the **first** `=`
   into `Raw[key]=value` (dup keys → last wins; scripts emit unique keys).
4. Return `*SourceResult`. All failure modes → `Status="failed"` + `Error`, **no
   alert, no panic**: download failure (network / non-200), non-root (script
   exits `1`, captured from exit code + stderr), script non-zero exit, timeout.

The collector performs **no** system mutation beyond writing/deleting its own
temp script file; the scripts themselves are read-only (collection only). This
keeps sichek's read-only contract intact.

## 7. Component behaviour (`sysinfo.go`)

- Implements the full `common.Component` interface.
- **Per-source loop** (the key deviation from `CommonService`):
  - `Start()` returns a `chan *common.Result` and spawns **one goroutine per
    enabled source** under a parent context. Each goroutine: run its source
    immediately, merge the `*SourceResult` into a mutex-guarded
    `outputs map[string]*SourceResult`, send one benign result to the channel;
    then loop on `time.NewTicker(source interval)`, re-collecting each tick.
    Each goroutine is `recover()`-guarded so one bad source can't crash others.
  - `HealthCheck(ctx)` (used by the one-shot CLI path) runs **all** enabled
    sources sequentially and returns the composite as `LastInfo()`.
  - `LastInfo()` returns a snapshot copy of `SysinfoOutput{Sources: outputs}`.
  - The Result pushed on the channel is **always benign**
    (`Item:"sysinfo", Status:normal, Level:info, Checkers:nil`) — a source
    failure lives only in its `SourceResult.Status`, so the node annotation stays
    clean.
- **Lifecycle / concurrency**:
  - Source goroutines track a `sync.WaitGroup`. `Stop()` cancels the parent
    context, waits for the WaitGroup, **then** closes the channel — so no
    goroutine ever sends on a closed channel. Sends also `select` on
    `ctx.Done()` to avoid blocking after cancel.
  - `Update(cfg)` swaps the config under a mutex, then **cancels and respawns**
    the source goroutines from the new config. This hot-reloads the full source
    set: enable/disable, added/removed sources, and changed intervals all take
    effect without a daemon restart.
- The daemon's existing `monitorComponent` consumes each benign result, then
  calls `LastInfo()` → `snapshotMgr.Update("sysinfo", info)`. So
  `components["sysinfo"].sources` is populated moments after startup and each
  source refreshes on its own cadence. **No change to `SnapshotManager` or the
  reporter**; the section flows to the downstream project via the existing
  `snapshot.json` POST.

### Annotation-path safety (verified)

- `service/info.go setAnnotationsByItem` falls through to `return nil` for an
  unknown item, so `SetNodeAnnotation` is a silent no-op for `sysinfo`.
- `AppendNodeAnnotation` (only on a health-check *timeout*) calls
  `getAnnotationsByItem`, which returns an error for unknown items — logged and
  harmless, and not normally reached since sources are individually
  timeout-bounded and Results stay benign. No `nodeAnnotation` schema change.

## 8. Wiring / touchpoints

One-time wiring (does **not** recur per future collector):

1. `consts/consts.go`
   - `ComponentNameSysinfo = "sysinfo"`.
   - append `ComponentNameSysinfo` to `DefaultComponents` (so the daemon loop in
     `cmd/command/daemon/run.go` instantiates it).
   - default interval (24h), timeout (60s), fallback base URL, seed source.
2. `cmd/command/component/all.go`
   - add `case consts.ComponentNameSysinfo: return sysinfo.NewComponent(cfgFile, specFile)`.
   - **add `sysinfo` to the `all` command's default `-I ignore-components`**
     (currently `"podlog,gpuevents,syslog"`), so a plain `sichek all` stays a
     pure, root-free, network-free local health check. `sichek all -E sysinfo`
     opts in explicitly.
3. `cmd/command/component/sysinfo.go` — `NewSysinfoCmd`: one-shot run of all
   sources (`--source <name>` to select one), prints each source's KV block +
   status, mirroring `cpu.go`.
4. `cmd/command/command.go` — `rootCmd.AddCommand(component.NewSysinfoCmd())`.
5. `service/snapshot.go`, reporter, metrics — **no change**.

**Adding a future collector** thereafter: append one entry to the `sysinfo.sources`
list in config (shipped via config-sync / `default_user_config.yaml`). No Go
change, no recompile.

## 9. Security / trust note (explicit)

The daemon downloads and executes, **as root**, shell scripts hosted on OSS, and
— per the confirmed "fetch fresh each run" decision — does **not** verify a
checksum. The trust boundary is the same OSS host + HTTPS already used for spec
downloads (`DomesticSpecURL` / `OverseasSpecURL`). This is an accepted,
documented risk, and it now applies to **every** script the config lists, so the
`sources` list is effectively an allowlist of root-executed URLs — treat edits to
it as privileged. Future hardening (out of scope): per-source sha256 pin,
vendored fallback copies, signature verification.

This is the one place sichek deviates from a strictly read-only posture; recorded
here and to be added to `docs/write-operations.md` during implementation.

## 10. Testing

- **Collector** (`collector_test.go`), table-driven, `t.TempDir()` isolated,
  `httptest.Server` for downloads:
  - normal multi-line `key=value` → correct map, `Status="ok"`, `KeyCount`.
  - value containing `=` → split on first `=` only.
  - blank lines skipped.
  - script exits non-zero → `Status="failed"`, `Error` set, `Raw` empty.
  - download 5xx / connection refused → `Status="failed"`.
  - context timeout → `Status="failed"`.
- **Config** (`config_test.go`): URL resolution precedence
  (source URL > base+path; base from config > env > derived > fallback);
  per-source interval/timeout/enable override vs engine default; env overrides.
- **Component** (`sysinfo_test.go`):
  - `Start()` with two fake sources yields both under `LastInfo().Sources`
    promptly (verifies "collect immediately", not after 24h).
  - one source failing does not affect the other.
  - disabled engine / disabled source → skipped.
  - `Update()` adding/removing a source and changing an interval takes effect.
  - `Stop()` exits all goroutines and closes the channel cleanly (no send-on-closed).
- `go vet ./...` and `go test ./components/sysinfo/...` green; full build via `make`.

## 11. Out of scope

- No health verdict / thresholds / spec baseline for these values (pure inventory).
- No Prometheus metrics.
- No K8s annotation schema change.
- No checksum pinning / offline fallback copy (may revisit later).
- No dynamic registration of sources as first-class `common.Component`s / CLI
  subcommands (all sources live under the one `sysinfo` component and the one
  `sichek sysinfo` command by design).
