#!/bin/bash

color_green="\033[1;32m"
color_yellow="\033[1;33m"
color_purple="\033[1;35m"
color_reset="\033[0m"

echo_back() {
    local _cmdLog=${1}
    printf "[${color_purple}EXEC${color_reset}] ${_cmdLog}\n"
    eval ${_cmdLog}
}

echo_info() {
    local _cmdLog=${1}
    printf "[${color_green}INFO${color_reset}] ${_cmdLog}\n"
}

echo_warn() {
    local _cmdLog=${1}
    printf "[${color_yellow}WARN${color_reset}] ${_cmdLog}\n"
}

# Global arrays
declare -a HOSTNAMES=()      # Global array to store parsed hostnames
declare -A CONFIG=()         # Global associative array to store config values
# Function: Parse hostname list
parse_hostnames() {
  local hostfile="$1"
  local host="$2"
  local hostnames=()

  # Prefer host parameter, ignore hostfile if host parameter is valid
  if [ "$host" != "None" ] && [ -n "$host" ]; then
    echo_info "Parsing hostnames from parameter: $host"
    IFS=',' read -ra HOST_ARRAY <<< "$host"
    for hostname in "${HOST_ARRAY[@]}"; do
      # Trim leading and trailing spaces
      hostname=$(echo "$hostname" | xargs)
      if [[ -n "$hostname" ]]; then
        hostnames+=("$hostname")
      fi
    done
  elif [ "$hostfile" != "None" ] && [ -n "$hostfile" ] && [ -f "$hostfile" ]; then
    echo_info "Reading hostnames from file: $hostfile"
    while IFS= read -r line; do
      # Skip empty lines and comment lines
      if [[ -n "$line" && ! "$line" =~ ^[[:space:]]*# ]]; then
        # Extract hostname (remove possible IP address and port number)
        hostname=$(echo "$line" | awk '{print $1}' | cut -d: -f1)
        if [[ -n "$hostname" ]]; then
          hostnames+=("$hostname")
        fi
      fi
    done < "$hostfile"
  fi

  # Store results in global array
  HOSTNAMES=("${hostnames[@]}")
}


# Function: Load user config from ~/.sichek/config.yaml
# Returns config as associative array (bash 4+)
# CONFIG must be declared globally before calling this function
load_user_config() {
  local config_path="${1:-$HOME/.sichek/config.yaml}"

  # Clear existing config
  CONFIG=()

  if [ ! -f "$config_path" ]; then
    return 0
  fi

  while IFS= read -r line; do
    line=$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
    # Skip empty lines and comments
    if [[ -z "$line" || "$line" =~ ^# ]]; then
      continue
    fi
    # Parse key: value
    if [[ "$line" =~ ^([^:]+):(.+)$ ]]; then
      local key=$(echo "${BASH_REMATCH[1]}" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
      local value=$(echo "${BASH_REMATCH[2]}" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
      # Remove surrounding quotes if present
      if [[ "$value" =~ ^[\'\"](.+)[\'\"]$ ]]; then
        value="${BASH_REMATCH[1]}"
      fi
      if [[ -n "$key" ]]; then
        CONFIG["$key"]="$value"
      fi
    fi
  done < "$config_path"
}

# Function: Pick value with priority: cli_value > config > default
# Usage: pick_value cli_value config_key default_value
pick_value() {
  local cli_value="$1"
  local config_key="$2"
  local default_value="$3"

  # If CLI value is provided and not empty, use it
  if [[ -n "$cli_value" && "$cli_value" != "None" ]]; then
    echo "$cli_value"
    return 0
  fi

  # Check config (skip if config value is empty string)
  local config_val="${CONFIG[$config_key]:-}"
  if [[ -n "$config_val" ]]; then
    echo "$config_val"
    return 0
  fi

  # Use default
  echo "$default_value"
}
