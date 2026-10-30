#!/bin/bash
# Ritmo del Barro — Game Launcher (Ebitengine v2.9.9)
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

export DISPLAY=:0
export SDL_VIDEODRIVER=x11

# Run with nix-shell for proper library paths
nix-shell --run "
    export CGO_ENABLED=1
    export DISPLAY=:0
    export SDL_VIDEODRIVER=x11
    ./ritmo-del-barro
"
