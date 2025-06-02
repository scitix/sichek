# Configuration System Overview
This document describes the configuration system used in this project, which is designed to support modular health checking, extensibility across hardware components, and environment-aware fallback mechanisms.


This document describes a unified and extensible mechanism for loading hardware specifications (spec) used by different components (e.g., HCA, GPU). Each component maintains a structured spec file describing hardware-specific characteristics, which are dynamically loaded and filtered based on the local host.

## Overview
Each component has a dedicated Spec module that:

- Loads hardware specifications (spec) from:

  - User-provided YAML files (`tryLoadFromFile(file string) error`)

  - Production default locations (`tryLoadFromDefault() error`)

  - Developer config directories (`tryLoadFromDevConfig() error`)

  - Cloud-based OSS fallback (e.g., when spec for a specific board ID is missing)

- Filters the loaded specs to include only those applicable to the local host hardware


## Spec Loading Workflow
The spec loading process follows a multi-stage fallback strategy:

1. **User-provided YAML file**: If a spec file path is explicitly provided, attempt to parse and load it.

2. **Production default spec**: If no file is provided or the previous step fails, load from a default production path (e.g., /var/sichek/config).

3. **Developer config directory**: If running in a development environment or test setup, scan a predefined config directory for `_spec.yaml` files(e.g., `path-to-sichek/component/hca/config/_spec.yaml`).

4. **OSS fallback for missing hardware specs**: If the local hardware (e.g., a specific HCA board ID) is not found in the current spec set, attempt to load it from an OSS (Object Storage Service) by fetching a YAML file for that specific ID.

## Local Filtering
After loading the complete set of available specs, the system calls `FilterSpecForLocalHost`(...) to:

- Detect locally installed hardware (e.g., via /sys/class/infiniband/*/board_id)

- Retain only the relevant specs based on the hardware's unique identifiers (e.g., board IDs)

- Attempt to auto-fill missing entries from cloud sources (e.g., OSS)

This ensures that each node only loads specs that are directly applicable to its own devices.

