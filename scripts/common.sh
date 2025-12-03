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

# Temporary label related variables
TEMP_LABEL_KEY="sichek-temp-test"
TEMP_LABEL_VALUE="$(date +%s)-$$"  # Use timestamp and process ID to ensure uniqueness
declare -a LABELED_NODES=()  # Store labeled nodes
declare -a HOSTNAMES=()      # Global array to store parsed hostnames
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

# Function: Set temporary labels on nodes
label_nodes() {
  local hostnames=("$@")

  if [ ${#hostnames[@]} -eq 0 ]; then
    return 0
  fi

  echo_info "Setting temporary labels on nodes..."
  for hostname in "${hostnames[@]}"; do
    #echo_info "  Labeling node: $hostname"
      if kubectl label node "$hostname" "$TEMP_LABEL_KEY=$TEMP_LABEL_VALUE" --overwrite > /dev/null 2>&1; then
      LABELED_NODES+=("$hostname")
      #echo_info "    ✓ Successfully labeled $hostname"
    else
      echo_warn "    ✗ Failed to label $hostname"
    fi
  done

  # Update NODE_SELECTOR to use temporary label
  NODE_SELECTOR="$TEMP_LABEL_KEY=$TEMP_LABEL_VALUE"
  echo_info "Updated nodeSelector to: $NODE_SELECTOR"
}

# Function: Clean up temporary labels
cleanup_labels() {
  if [ ${#LABELED_NODES[@]} -gt 0 ]; then
    echo_info "Cleaning up temporary labels..."
    for hostname in "${LABELED_NODES[@]}"; do
      #echo_info "  Removing label from node: $hostname"
      kubectl label node "$hostname" "$TEMP_LABEL_KEY-" > /dev/null 2>&1 || echo_warn "    Failed to remove label from $hostname"
    done
  fi
}

# Function: Process hostfile and host parameters, set temporary labels
setup_host_labels() {
  local hostfile="$1"
  local host="$2"
  local node_selector="$3"

  # Parse hostname list into global array
  parse_hostnames "$hostfile" "$host"

  # If hostfile or host parameter is provided, set temporary labels
  if [ ${#HOSTNAMES[@]} -gt 0 ]; then
    echo_info "Found ${#HOSTNAMES[@]} hostname(s) to test: ${HOSTNAMES[*]}"
    label_nodes "${HOSTNAMES[@]}"
  else
    echo_info "No specific hostnames provided, exiting..."
    exit 1
  fi
}
