{ pkgs ? import <nixpkgs> {} }:

pkgs.stdenv.mkDerivation {
  pname = "xolo-game";
  version = "0.1.0";

  src = ./.;

  dontConfigure = true;
  dontBuild = true;

  installPhase = ''
    mkdir -p $out/bin
    mkdir -p $out/share/xolo-game

    cp xolo-game $out/bin/xolo-game
    cp tileset.png $out/share/xolo-game/
    cp sprites.png $out/share/xolo-game/
    cp launch_xolo_game.sh $out/share/xolo-game/
    cp shell.nix $out/share/xolo-game/

    chmod +x $out/bin/xolo-game
  '';

  meta = with pkgs.lib; {
    description = "XOLO - Pixel Platformer (Ebitengine)";
    license = licenses.mit;
  };
}
