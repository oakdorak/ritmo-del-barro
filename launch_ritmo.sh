#!/bin/bash
# XOLO Game Launcher — Ebitengine v2.9.9
# Run this on Beta's physical display

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# Allow root to connect to X11
xhost +si:localuser:root 2>/dev/null
xhost +si:localuser:oscar 2>/dev/null

sudo -u oscar nix-shell -p gcc xorg.libX11 xorg.libXcursor xorg.libXrandr xorg.libXi xorg.libXinerama xorg.libXxf86vm mesa mesa.drivers libGL libGLU --run "
    export SDL_VIDEODRIVER=x11
    export DISPLAY=:0
    ./ritmo-del-barro
"
