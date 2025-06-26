#!/bin/bash

set -e

SICL_INSTALL_PATH="/usr/local/sihpc"
SICL_INSTALLER_URL="https://oss-ap-southeast.scitix.ai/scitix-release/sicl-24.11-1.cuda1262.ubuntu2204.run"
SICL_INSTALLER_LOCAL="/tmp/sicl.run"

echo "[sichek preinstall] Checking for SICL..."

if [ -d "$SICL_INSTALL_PATH" ]; then
    echo "[sichek preinstall] SICL already installed at $SICL_INSTALL_PATH."
    exit 0
fi

echo "[sichek preinstall] SICL not found, downloading from ${SICL_INSTALLER_URL}..."

if ! curl -fsSL -o "$SICL_INSTALLER_LOCAL" "$SICL_INSTALLER_URL"; then
    echo "[sichek preinstall] Failed to download SICL installer."
    exit 1
fi

chmod +x "$SICL_INSTALLER_LOCAL"

echo "[sichek preinstall] Running installer..."
bash "$SICL_INSTALLER_LOCAL"

echo "[check_sicl] Cleaning up installer..."
rm -f "$SICL_INSTALLER_LOCAL"

if [ $? -ne 0 ]; then
    echo "[sichek preinstall] SICL installation failed."
    exit 1
fi

echo "[sichek preinstall] SICL installed successfully."
