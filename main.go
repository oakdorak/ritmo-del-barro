package main

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/oakdorak/ritmo-del-barro/game"
)

// Ritmo del Barro — prototipo platformer+ritmo para niños neurodivergentes.
// Movement reusado de XOLO (coyote / jump-buffer / variable height).
func main() {
	ebiten.SetWindowSize(game.ScreenW*2, game.ScreenH*2) // 640x360
	ebiten.SetWindowTitle("Ritmo del Barro  (tap/click: izq•der salto)")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	g := game.NewGame()
	if err := ebiten.RunGame(g); err != nil {
		panic(err)
	}
}
