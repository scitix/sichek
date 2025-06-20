# Configuration System Overview

This document describes the configuration system used in this project, which is designed to support modular health checking, extensibility across hardware components, and environment-aware fallback mechanisms.

---

## Configuration Types

There are **three types** of configuration files in this system:

| Type            | Purpose                                                      | Maintained By    |
| --------------- | ------------------------------------------------------------ | ---------------- |
| **User Config** | Controls runtime behavior (intervals, metrics, toggles)      | End users        |
| **Spec Config** | Describes hardware-specific characteristics (e.g., HCA, GPU) | End users & Developers |
| **Event Rules** | Defines rules for abnormal event detection (logs & metrics)  | End users & Developers |

---

## 1. User Configuration

Defines runtime control for each component or module.

### Example (`user_config.yaml`)

```yaml
cpu:
  query_interval: 10s
  cache_size: 1024
  enable_metrics: true
```


### Load Priority

1. user-specified file
2. Production default: `/var/sicheck/config/default_user_config.yaml`
3. Dev default: 
  - for production env: `/var/sichek/config/xx-component/default_user_config.yaml`
  - for development env: based on `runtime.Caller` → `path-to-sichek/component/config/default_user_config.yaml`

### Example Loader

```go
err := LoadUserConfig(configFilePath, &cfg)
```

---

## 2. Spec Configuration

Spec files define expected hardware layouts and characteristics. Each component (e.g., HCA, GPU) has a dedicated Spec module that:

- Loads hardware specifications (spec) from:

  - User-provided YAML files (`tryLoadFromFile(file string) error`)

  - Production default locations (`tryLoadFromDefault() error`)

  - Developer config directories (`tryLoadFromDevConfig() error`)
    - for production env: `/var/sichek/config/xx-component/*_spec.yaml`
    - for development env: based on `runtime.Caller` → `path-to-sichek/component/config/*_spec.yaml`

  - Cloud-based OSS fallback (e.g., when spec for a specific board ID is missing)

- Filters the loaded specs to include only those applicable to the local host hardware


### Spec Loading Framework

Each component defines a `Spec` module responsible for loading and filtering relevant spec entries based on the **local host hardware**.

#### Spec Load Flow (Fallback Strategy)
The spec loading process follows a multi-stage fallback strategy:

1. **User-provided YAML file**: If a spec file path is explicitly provided, attempt to parse and load it.

2. **Production default spec**: If no file is provided or the previous step fails, load from a default production path (e.g., `/var/sichek/config/default_spec.yaml`).

3. **Developer config directory**: 
  - If running in a production environment, scan a predefined config directory for `_spec.yaml` files(e.g., `/var/sichek/config/xx-component/*_spec.yaml`).
  - If running in a development environment, scan a predefined config directory for `_spec.yaml` files(e.g., `path-to-sichek/component/xx=component/config/*_spec.yaml`).

4. **OSS fallback for missing hardware specs**: If the local hardware (e.g., a specific HCA board ID) is not found in the current spec set, attempt to load it from an OSS (Object Storage Service) by fetching a YAML file for that specific ID.

#### Local Filtering

After loading the complete set of available specs, the system calls `FilterSpecForLocalHost`(...) to:

- Detect locally installed hardware (e.g., via /sys/class/infiniband/*/board_id)

- Retain only the relevant specs based on the hardware's unique identifiers (e.g., board IDs)

- Attempt to auto-fill missing entries from cloud sources (e.g., OSS)

This ensures that each node only loads specs that are directly applicable to its own devices.


---

## 3. Event Rules Configuration

This config defines how to detect abnormal events using either:

* **Log matching**
* **Metric combinations**
* (Future: state transitions, tracing anomalies)

We call these **event rules** (instead of just log rules) for generality.

### Example (`event_rules.yaml`)

```yaml
cpu:  
  event_checkers:
    kernel_panic:
      name: "kernel_panic"
      description: "kernel panic are alerted"
      log_file: "/var/log/syslog"
      regexp: "kernel panic"
      level: critical
      suggestion: "restart node"
    cpu_overheating:
      name: "cpu_overheating"
      description: "CPU Core temperature is above threshold, cpu clock is throttled"
      log_file: "/var/log/syslog"
      regexp: "temperature above threshold"
      level: warning
      suggestion: ""
    cpu_lockup:
      name: "cpu_lockup"
      description: "CPU lockup occurs indicating the CPU cannot execute scheduled tasks due to software or hardware issues"
      log_file: "/var/log/syslog"
      regexp: "(soft lockup)|(hard LOCKUP)"
      level: warning
      suggestion: ""
```

### Load Priority

1. Production default: `/var/sicheck/config/<component>/default_event_rules.yaml`
2. Dev fallback: based on `runtime.Caller` → `<repo>/<component>/config/default_event_rules.yaml`

### Example Loader

```go
err := LoadDefaultEventRules(&rules, "cpu")
```

---

## Directory Structure
### For production environment
```
/var/sichek/
├── config/
│   └── default_user_config.yaml        # global default user config
│   └── default_spec.yaml               # global default specification
│   ├── nvidia/
│   │   ├── *_spec.yaml                  # GPU hardware spec
│   └── hca/
│   |   ├── *_spec.yaml                  # HCA hardware spec
│   └── hang/
│       └── event_rules.yaml             # Event rules for GPU hang detection
```

### For development environment
```
project-root/
├── config/
│   └── user_config.yaml                     # Runtime control (global)
├── components/
│   ├── nvidia/
│   │   └── config/
│   │       ├── *_spec.yaml                  # GPU hardware spec
│   └── hca/
│   |   └── config/
│   |       ├── *_spec.yaml                  # HCA hardware spec
│   └── hang/
│       └── config/
│           └── event_rules.yaml             # Event rules for GPU hang detection
```

---
