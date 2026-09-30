#!/bin/bash

# Pre-uninstall script for sichek package
# This script removes /var/sichek/scripts from the system PATH

set -e

PROFILE_D_FILE="/etc/profile.d/sichek.sh"

# Remove the profile.d script if it exists
if [ -f "$PROFILE_D_FILE" ]; then
    rm -f "$PROFILE_D_FILE"
    echo "Removed sichek scripts from system PATH"
else
    echo "No sichek PATH configuration found"
fi
