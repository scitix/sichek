# Transceiver Signal-Integrity Check (mlxlink Recommendation)

Date: 2026-06-16
Component: `components/transceiver/`
Target branch: `feat/snapshot-issues`

## Problem

`mlxlink` can report a physical-layer signal-integrity failure in its
`Troubleshooting Info → Recommendation` field (e.g. `Bad signal integrity`)
while the error counters (`Symbol Errors`, sysfs `symbol_error_counter`)
are still `0`. The eye diagram has collapsed and FEC is masking the damage;
the link is one step from mass RDMA packet loss / PFC storms.

Sichek currently parses `mlxlink -d <dev> -m` in the transceiver collector
(`mlxlink_dom.go`) but **discards the `Recommendation` line** — the parser
`switch` only extracts temperature/voltage/power/vendor. There is no
counter-independent alert for signal integrity.

## Placement decision: transceiver (光模块), NOT infiniband

- `mlxlink` is invoked **only** by the transceiver component. The IB component
  does no mlxlink and has no per-module DOM model. Putting this in IB would
  require a second mlxlink invocation (PCIe device lock contention, see
  `transceiver_info.go` worker-pool comment) and break IB's layering.
- The `Recommendation` line is **already in the output we capture** — `-m`
  only appends Module Info; the default Troubleshooting Info section still
  prints. No command change needed.
- Semantics match: "signal integrity / clean or replace optical module &
  fiber" is squarely a transceiver/cable physical-layer concern, alongside the
  existing tx/rx-power, bias, temperature, and link-error checkers.

## Design

Scope is confined to the transceiver component. **No service-layer changes**:
on `feat/snapshot-issues`, transceiver is already wired into the annotation
schema (`info.go` `nodeAnnotation.Transceiver` + both switches) and the
snapshot mirrors annotation issues (`snapshot.go` `Issues *nodeAnnotation` +
`SetIssues`). A well-formed `CheckerResult` therefore flows automatically to
CLI, Prometheus, the K8s annotation, and `snapshot.issues.transceiver`.

### 1. Collector — capture `Recommendation`
- `collector/transceiver_info.go`: add `Recommendation string \`json:"recommendation"\`` to `ModuleInfo`.
  This field auto-appears in `snapshot.components.transceiver.modules[].recommendation`.
- `collector/mlxlink_dom.go`: in `parseMLXLink` switch add
  `case key == "Recommendation": m.Recommendation = stripANSI(val)`.
  Add a `stripANSI` helper (regex `\x1b\[[0-9;]*m`) so the red-colored
  output (`\x1b[31mBad signal integrity\x1b[0m`) matches cleanly.

### 2. Checker — `checker/check_signal_integrity.go` (new)
- `SignalIntegrityChecker{spec *config.TransceiverSpec}`.
- Bad-value list (user-confirmed approach): package var
  `badRecommendationPatterns = []string{"bad signal integrity"}`, case-insensitive
  substring match, extensible (e.g. add `"cable replacement"` later).
- For each present module with a non-empty `Recommendation` matching the list:
  - `Status = Abnormal`, `ErrorName = "BadSignalIntegrity"`.
  - Level via `config.GetCheckItem(name, module.NetworkType)` — business→Critical,
    management→Warning (reuses existing split). Take the most severe across modules.
  - `Device` = failing interface(s) (joined). `Detail` includes interface +
    IBDev/BDF + the raw recommendation text. `Suggestion` = the remediation text.
- **Counter-independent**: never reads symbol-error counters, so it fires even
  when `Symbol Errors = 0`.

### 3. Config
- `config/config.go`: `SignalIntegrityCheckerName = "check_signal_integrity"`.
- `config/check_items.go`: add an item to `BusinessCheckItems` (LevelCritical)
  and `ManagementCheckItems` (LevelWarning):
  - `ErrorName: "BadSignalIntegrity"`
  - `Suggestion: "物理层信号完整性差，FEC 正在带病工作，请尽快在低峰期隔离节点并清洗/更换光模块与光纤"`

### 4. Registration
- `checker/checker.go`: add `&SignalIntegrityChecker{spec: spec}` to `NewCheckers`
  (disable-able via existing `IgnoredCheckers`).

## Severity / annotation schema notes
- Annotation / `snapshot.issues` entries store only `error_name` + `device`
  (`info.go` `annotation` struct). The suggestion/detail text reaches CLI,
  `CheckerResult`, and Prometheus — not the annotation. `Device` carries the
  port identity for the K8s consumer (e.g. TaskGuard).

## Testing
- `collector/mlxlink_dom_test.go`: extend sample with a Troubleshooting Info
  section (`Recommendation : Bad signal integrity`), an ANSI-colored variant,
  and `No issue was observed`; assert `m.Recommendation`. Unit-test `stripANSI`.
- `checker/check_signal_integrity_test.go`: table-driven —
  business bad → Critical+Abnormal+device; management bad → Warning;
  `No issue was observed` → Normal; `Present=false` → skipped;
  counter=0 + bad recommendation → still Abnormal.
- `go test ./components/transceiver/...` green.

## Out of scope (YAGNI)
- No per-network spec toggle (use `IgnoredCheckers`).
- No dedicated Prometheus gauge (the existing `ModuleGauge.ExportStruct`
  exports numeric fields only; revisit if a 0/1 signal-integrity gauge is wanted).
- No change to the alert schema (error_name + device stays).
