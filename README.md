# Ritmo del Barro

A challenging rhythm-platformer game built with [Go](https://go.dev) and [Ebitengine v2.9.9](https://github.com/hajimehoshi/ebiten).

**Ritmo del Barro** combines tight platformer mechanics (double-jump, dash, wall-slide) with rhythm-based gameplay — platforms activate on beat, mud tiles slow you down, and your score depends on timing your jumps to the BPM.

## Gameplay

- **Movement**: `A/D` or `←/→` to move | `W/Space` or `↑` to jump | `Shift` to dash | `S↓` for fast fall
- **Rhythm**: Jump in sync with the beat to earn PERFECT/GREAT/OKAY ratings
- **Beat platforms**: Purple `R` platforms appear/disappear on the beat
- **Mud tiles**: Brown `M` tiles reduce movement speed by 50%
- **Levels**: 3 rhythm levels with increasing BPM (120 → 140 → 160)

## Levels

| # | Name | BPM | Theme |
|---|------|-----|-------|
| 1 | Charco Congelado | 120 | Frozen mud pits, slow tempo |
| 2 | Resbaladilla | 140 | Slippery platforms, faster beats |
| 3 | Barro Vivo | 160 | Live mud, fastest tempo |

## Building

### Native

```bash
make native
./ritmo-del-barro
```

On NixOS, use the provided `shell.nix`:
```bash
nix-shell -p go --run "go build -o ritmo-del-barro && ./ritmo-del-barro"
```

### WebAssembly (iPad / Browser)

```bash
make wasm
make serve
```

Then open `http://localhost:8080/build/` in your browser.

## GitHub Actions

CI automatically builds both native and WASM binaries on every push to `main`. WASM deploys to GitHub Pages.

## Assets

Sprites and tilesets are sourced from the [XOLO Game Engine](https://github.com/oakdorak/xolo-game).

## License

MIT
