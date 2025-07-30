#!/bin/bash

set -e

SICL_INSTALL_PATH="/usr/local/sihpc"
SICL_INSTALLER_URL1="https://oss-ap-southeast.scitix.ai/scitix-release/sicl-24.11-1.cuda1262.ubuntu2204.run"
SICL_INSTALLER_URL2="https://oss-cn-beijing.siflow.cn/scitix-release/sicl-24.11-1.cuda1262.ubuntu2204.run"
SICL_INSTALLER_URL3="https://oss-cn-shanghai.siflow.cn/scitix-release/sicl-24.11-1.cuda1262.ubuntu2204.run"
SICL_INSTALLER_LOCAL="/tmp/sicl.run"

echo "[sichek preinstall] Checking for SICL..."

if [ -d "$SICL_INSTALL_PATH" ]; then
    echo "[sichek preinstall] SICL already installed at $SICL_INSTALL_PATH."
    exit 0
fi

echo "[sichek preinstall] SICL not found, try to downloading from ${SICL_INSTALLER_URL1}..."

if ! curl --connect-timeout 5 -fsSL -o "$SICL_INSTALLER_LOCAL" "$SICL_INSTALLER_URL1"; then
    echo "[sichek preinstall] Failed to download SICL installe from $SICL_INSTALLER_URL1, try to downloading from ${SICL_INSTALLER_URL2}..."
    if ! curl --connect-timeout 5 -fsSL -o "$SICL_INSTALLER_LOCAL" "$SICL_INSTALLER_URL2"; then
        echo "[sichek preinstall] Failed to download SICL installe from $SICL_INSTALLER_URL2, try to downloading from ${SICL_INSTALLER_URL3}..."
        if ! curl --connect-timeout 5 -fsSL -o "$SICL_INSTALLER_LOCAL" "$SICL_INSTALLER_URL3"; then
            echo "[sichek preinstall] Failed to download SICL installe from all source."
            exit 1
        fi
    fi
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
