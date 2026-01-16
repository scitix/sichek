#!/bin/bash

set -e

SICL_VERSION=${1:-"sicl-25.11-1.cuda128.ubuntu2004.run"}
SICL_INSTALL_PATH="/usr/local/sihpc"
SICL_INSTALLER_URL1="https://oss-ap-southeast.scitix.ai/scitix-release/${SICL_VERSION}"
SICL_INSTALLER_LOCAL="/tmp/sicl.run"
SICL_PACKAGED_PATH="/var/sichek/sicl/${SICL_VERSION}"

echo "[sichek postinstall] Checking for SICL ${SICL_VERSION}..."

if [ -d "$SICL_INSTALL_PATH" ]; then
    echo "[sichek postinstall] SICL already installed at $SICL_INSTALL_PATH."
    exit 0
fi

# First, try to use the packaged SICL installer
if [ -f "$SICL_PACKAGED_PATH" ]; then
    echo "[sichek postinstall] Found packaged SICL installer at $SICL_PACKAGED_PATH"
    chmod +x "$SICL_PACKAGED_PATH"
    
    echo "[sichek postinstall] Running packaged installer..."
    bash "$SICL_PACKAGED_PATH"
    
    if [ $? -eq 0 ]; then
        echo "[sichek postinstall] SICL installed successfully from packaged installer."
        exit 0
    else
        echo "[sichek postinstall] Packaged installer failed, trying network download..."
    fi
fi

echo "[sichek postinstall] Packaged SICL not found, try to downloading from ${SICL_INSTALLER_URL1}..."

if ! curl --connect-timeout 5 -fsSL -o "$SICL_INSTALLER_LOCAL" "$SICL_INSTALLER_URL1"; then
    echo "[sichek postinstall] Failed to download SICL installe from $SICL_INSTALLER_URL1"
    exit 1
fi

chmod +x "$SICL_INSTALLER_LOCAL"

echo "[sichek postinstall] Running installer..."
bash "$SICL_INSTALLER_LOCAL"

echo "[check_sicl] Cleaning up installer..."
rm -f "$SICL_INSTALLER_LOCAL"

if [ $? -ne 0 ]; then
    echo "[sichek postinstall] SICL ${SICL_VERSION} installation failed."
    exit 1
fi

echo "[sichek postinstall] SICL ${SICL_VERSION} installed successfully."
