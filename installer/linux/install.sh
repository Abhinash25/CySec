#!/usr/bin/env bash
# CySec.env Linux Installer

set -e

echo "=================================================="
echo "          CySec.env Setup (Linux)                 "
echo "=================================================="

# Detect OS
if [[ "$OSTYPE" != "linux-gnu"* ]]; then
    echo "Error: This script is for Linux only."
    exit 1
fi

INSTALL_DIR="/usr/local/bin"
DATA_DIR="$HOME/.local/share/cysec"

# Ensure directories
mkdir -p "$DATA_DIR/workspace"
mkdir -p "$DATA_DIR/config"
mkdir -p "$DATA_DIR/registry/tools"

echo "[1/4] Copying core binary..."
# Assuming running from the build directory
if [[ -f "./cysec" ]]; then
    sudo cp ./cysec "$INSTALL_DIR/cysec"
    sudo chmod +x "$INSTALL_DIR/cysec"
else
    echo "Error: cysec binary not found in current directory."
    exit 1
fi

echo "[2/4] Copying registry..."
if [[ -d "../../registry" ]]; then
    cp -r ../../registry/* "$DATA_DIR/registry/"
fi

echo "[3/4] Creating launcher application..."
DESKTOP_FILE="$HOME/.local/share/applications/cysec-terminal.desktop"
mkdir -p "$(dirname "$DESKTOP_FILE")"
cat > "$DESKTOP_FILE" << EOF
[Desktop Entry]
Name=CySec Terminal
Comment=Unified Cybersecurity Environment
Exec=cysec
Icon=utilities-terminal
Terminal=true
Type=Application
Categories=System;Utility;Security;
EOF
chmod +x "$DESKTOP_FILE"

echo "[4/4] Provisioning Full Environment..."
export CYSEC_HOME="$DATA_DIR"

echo ""
echo "Do you want to run 'cysec setup --full' now? (This will download ~1GB of tools and require ~5GB of space)"
read -p "Install Full Profile? [Y/n] " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Nn]$ ]]
then
    echo "Skipping full provisioning."
    echo "You can run 'cysec setup --full' later."
else
    cysec setup --full
fi

echo ""
echo "Installation Complete."
echo "You can launch CySec.env from your application menu as 'CySec Terminal',"
echo "or type 'cysec' in your terminal."
