{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  buildInputs = with pkgs; [
    gcc
    pkg-config
    xorg.libX11
    xorg.libXcursor
    xorg.libXrandr
    xorg.libXi
    xorg.libXinerama
    xorg.libXxf86vm
    mesa
    mesa.drivers
    libGL
    libGLU
    alsa-lib
    go
  ];

  shellHook = ''
    export CGO_ENABLED=1
    export SDL_VIDEODRIVER=x11
    export DISPLAY=:0
    echo "XOLO Game dev environment ready (Ebitengine v2.9.9)"
    echo "Run: go build -o xolo-game && ./xolo-game"
  '';
}
