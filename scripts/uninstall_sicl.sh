#!/usr/bin/env bash

set -e

echo "[SICL Uninstall] Starting uninstallation of SICL components..."

SICL_DIR="/usr/local/sihpc"
SICL_LDCONF="/etc/ld.so.conf.d/sihpc.conf"

# Check and remove the main program directory
if [ -d "$SICL_DIR" ]; then
    echo "Removing $SICL_DIR ..."
    sudo rm -rf "$SICL_DIR"
else
    echo "Directory $SICL_DIR not found. Skipping."
fi
# Remove dynamic linker configuration
if [ -f "$SICL_LDCONF" ]; then
    echo "Removing $SICL_LDCONF ..."
    sudo rm -f "$SICL_LDCONF"
else
    echo "File $SICL_LDCONF not found. Skipping."
fi

# Regenerate dynamic linker cache
echo "Refreshing linker cache with ldconfig ..."
sudo ldconfig

echo "[SICL Uninstall] Uninstallation complete."
