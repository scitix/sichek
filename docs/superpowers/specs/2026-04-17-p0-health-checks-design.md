# P0 Health Check Expansion Design

Extend existing CPU and Memory components with three new health check capabilities:
clock synchronization (PTP/NTP), CPU MCE detection, and memory ECC/EDAC monitoring.

## Scope

All changes are extensions to existing components, no new components created:

- **CPU component** (`components/cpu/`): add clock sync checker + MCE checker
- **Memory component** (`components/memory/`): add ECC checker + capacity checker

Data collection: sysfs/procfs reading + EventFilter log pattern matching.

## CPU Component Extension

### New Files

```
components/cpu/
├── collector/
│   ├── ptp_info.go        # PTP/NTP status collection
│   └── mce_info.go        # MCE counter collection
├── checker/
│   ├── clock_sync.go      # Clock sync checker
│   └── cpu_mce.go         # MCE checker
└── config/
    └── check_items.go     # Add new CheckerResult templates
```

### Collector: PTP/NTP (ptp_info.go)

Collects clock synchronization status:

1. Check `ptp4l` service status via `systemctl is-active ptp4l`
2. Parse ptp4l offset from `/var/log/ptp4l.log` or `journalctl -u ptp4l`
3. Check `phc2sys` service status
4. Fallback: check `chrony`/`ntpd`, parse `chronyc tracking` offset

Output structure:

```go
type PTPInfo struct {
    PTPServiceActive  bool
    PHC2SysActive     bool
    OffsetNs          float64   // Current offset in nanoseconds
    NTPServiceActive  bool
    NTPOffset         float64   // NTP offset if PTP unavailable
    LastSyncTime      time.Time
}
```

### Collector: MCE (mce_info.go)

Collects Machine Check Exception counters:

1. Read `/sys/devices/system/cpu/machinecheck/machinecheck*/` counters
2. Fallback: parse `/var/log/mcelog` for CE/UCE events
3. Track delta between collections for rate-based alerting

Output structure:

```go
type MCEInfo struct {
    CorrectedCount   int64
    UncorrectedCount int64
    LastEventTime    time.Time
    Available        bool      // false if sysfs path doesn't exist
}
```

### CPUOutput Extension

```go
type CPUOutput struct {
    Time        time.Time
    CPUArchInfo CPUArchInfo
    UsageInfo   Usage
    HostInfo    HostInfo
    Uptime      string
    PTPInfo     PTPInfo    // NEW
    MCEInfo     MCEInfo    // NEW
}
```

### Checker: clock_sync.go

Two sub-checkers in one file:

| Checker Name | Check | Abnormal Condition | Level |
|---|---|---|---|
| `clock-sync-service` | PTP/NTP service running | ptp4l not running AND chrony/ntpd not running | Warning |
| `clock-sync-offset` | Clock offset magnitude | >1ms: Warning, >10ms: Critical | Warning/Critical |

Logic:
- If PTP available, use PTP offset
- If PTP unavailable, fall back to NTP offset
- If neither available, report service checker as Warning
- Offset checker only runs if a sync service is active

### Checker: cpu_mce.go

Two sub-checkers in one file:

| Checker Name | Check | Abnormal Condition | Level |
|---|---|---|---|
| `cpu-mce-uncorrected` | UCE count | >0 | Critical |
| `cpu-mce-corrected` | CE count growth | >threshold (default 10) | Warning |

Logic:
- If MCE sysfs not available, return Info "MCE monitoring not available"
- UCE >0 is always Critical (hardware is degrading, may crash)
- CE threshold is configurable via spec

### CheckerResult Templates (check_items.go additions)

```go
"clock-sync-service": {
    Name:        "clock-sync-service",
    Description: "Check if PTP or NTP clock sync service is running",
    Spec:        "Running",
    Level:       consts.LevelWarning,
    ErrorName:   "ClockSyncServiceNotRunning",
    Suggestion:  "Start ptp4l/phc2sys or chrony/ntpd service",
},
"clock-sync-offset": {
    Name:        "clock-sync-offset",
    Description: "Check clock sync offset is within threshold",
    Spec:        "<1ms",
    Level:       consts.LevelWarning,
    ErrorName:   "ClockSyncOffsetHigh",
    Suggestion:  "Check PTP/NTP configuration and network connectivity to time source",
},
"cpu-mce-uncorrected": {
    Name:        "cpu-mce-uncorrected",
    Description: "Check for uncorrectable Machine Check Exceptions",
    Spec:        "0",
    Level:       consts.LevelCritical,
    ErrorName:   "CPUMCEUncorrected",
    Suggestion:  "CPU hardware error detected. Schedule maintenance and migrate workloads",
},
"cpu-mce-corrected": {
    Name:        "cpu-mce-corrected",
    Description: "Check correctable MCE count is below threshold",
    Spec:        "<threshold",
    Level:       consts.LevelWarning,
    ErrorName:   "CPUMCECorrectedHigh",
    Suggestion:  "Correctable CPU errors increasing. Monitor trend and plan maintenance",
},
```

## Memory Component Extension

### New Files

```
components/memory/
├── collector/
│   ├── edac_info.go       # EDAC sysfs counter collection
│   └── memory_capacity.go # Memory capacity collection
├── checker/
│   ├── checker.go         # Checker factory (new, memory currently has no checkers)
│   ├── memory_ecc.go      # ECC CE/UCE checker
│   └── memory_capacity.go # Memory capacity checker
└── config/
    └── check_items.go     # Add new CheckerResult templates
```

### Collector: EDAC (edac_info.go)

Reads EDAC sysfs counters:

1. Enumerate `/sys/devices/system/edac/mc*/`
2. For each memory controller, read `ce_count`, `ue_count`
3. Enumerate `csrow*/` for per-DIMM detail

Output structure:

```go
type EDACInfo struct {
    Available   bool
    Controllers []MCInfo
    TotalCE     int64
    TotalUCE    int64
}

type MCInfo struct {
    ID      string
    CECount int64
    UCECount int64
    CSRows  []CSRowInfo
}

type CSRowInfo struct {
    ID      string
    CECount int64
    UCECount int64
}
```

### Collector: Memory Capacity (memory_capacity.go)

Reads memory capacity from `/proc/meminfo`:

```go
type MemoryCapacityInfo struct {
    TotalKB    int64
    TotalGB    float64
}
```

### Memory Component Modification

Current memory component has `checkers: nil`. Changes needed:

1. Create `checker/checker.go` with `NewCheckers()` factory
2. Extend collector output to include EDAC + capacity info
3. Register checkers in `memory.go` constructor

### Checker: memory_ecc.go

Two sub-checkers:

| Checker Name | Check | Abnormal Condition | Level |
|---|---|---|---|
| `memory-ecc-uncorrected` | EDAC UCE count | >0 | Critical |
| `memory-ecc-corrected` | EDAC CE count | >threshold (default 100) | Warning |

Logic:
- If EDAC sysfs not available, return Info "EDAC not available, ECC may not be enabled"
- UCE >0 is Critical (data corruption risk)
- CE threshold configurable via spec

### Checker: memory_capacity.go

| Checker Name | Check | Abnormal Condition | Level |
|---|---|---|---|
| `memory-capacity` | Total memory vs spec | Difference >5% | Critical |

Logic:
- Compare actual MemTotal with spec expected value
- Tolerance configurable (default 5%)
- If spec has no expected value, skip check (return normal)

### CheckerResult Templates (check_items.go)

```go
"memory-ecc-uncorrected": {
    Name:        "memory-ecc-uncorrected",
    Description: "Check for uncorrectable memory ECC errors",
    Spec:        "0",
    Level:       consts.LevelCritical,
    ErrorName:   "MemoryECCUncorrected",
    Suggestion:  "Uncorrectable memory error detected. Identify faulty DIMM and replace",
},
"memory-ecc-corrected": {
    Name:        "memory-ecc-corrected",
    Description: "Check correctable memory ECC error count is below threshold",
    Spec:        "<threshold",
    Level:       consts.LevelWarning,
    ErrorName:   "MemoryECCCorrectedHigh",
    Suggestion:  "Correctable memory errors increasing. Monitor DIMM health and plan replacement",
},
"memory-capacity": {
    Name:        "memory-capacity",
    Description: "Check total memory matches expected specification",
    Spec:        "matches spec",
    Level:       consts.LevelCritical,
    ErrorName:   "MemoryCapacityMismatch",
    Suggestion:  "Memory capacity does not match spec. Check for failed DIMMs",
},
```

## Event Rules

### cpu_rules.yaml additions

```yaml
mce_event:
  name: "mce_error"
  log_file: "/var/log/kern.log,/var/log/mcelog"
  regexp: "Machine check|MCE|mce:"
  level: "critical"
  description: "Machine Check Exception detected in kernel log"
  suggestion: "Check CPU/memory hardware health via mcelog or rasdaemon"
```

### memory_rules.yaml additions

```yaml
edac_event:
  name: "edac_error"
  log_file: "/var/log/kern.log"
  regexp: "EDAC.*CE|EDAC.*UE|corrected memory error|uncorrectable memory error"
  level: "critical"
  description: "EDAC memory error detected in kernel log"
  suggestion: "Check DIMM health, identify faulty module, consider replacement"
```

## Spec Configuration

### CPU spec additions

```yaml
cpu:
  clock_sync:
    offset_warning_ms: 1
    offset_critical_ms: 10
  mce:
    ce_threshold: 10
```

### Memory spec additions

```yaml
memory:
  ecc:
    ce_threshold: 100
  capacity:
    expected_total_gb: 1024
    tolerance_percent: 5
```

## Edge Cases

| Scenario | Behavior |
|---|---|
| EDAC sysfs not available | ECC checker returns Info "EDAC not available" |
| PTP not installed | Skip PTP, check NTP; both absent → Warning |
| MCE sysfs path varies by kernel | Collector tries multiple known paths |
| Container environment | sysfs mounted read-only is fine (read-only access) |
| Spec has no expected memory | Capacity checker skips, returns normal |
| No mcelog/rasdaemon installed | EventFilter rules won't match (graceful no-op) |

## Constants (consts/consts.go additions)

```go
// CPU checker IDs (extend existing range)
CheckerIDClockSyncService  = 1300
CheckerIDClockSyncOffset   = 1301
CheckerIDCPUMCEUncorrected = 1302
CheckerIDCPUMCECorrected   = 1303

// Memory checker IDs (extend existing range)
CheckerIDMemoryECCUncorrected = 2100
CheckerIDMemoryECCCorrected   = 2101
CheckerIDMemoryCapacity       = 2102
```
