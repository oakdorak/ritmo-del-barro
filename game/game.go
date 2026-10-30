package game

import (
	"fmt"
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	ScreenW = 320
	ScreenH = 180
	groundY = 156.0 // Y del suelo
)

type Game struct {
	player   *Player
	beat     *Beat
	beeper   *Beeper
	score    int
	combo    int
	platform *Platform
}

func NewGame() *Game {
	return &Game{
		beat:     NewBeat(120),
		beeper:   NewBeeper(),
		player:   NewPlayer(),
		platform: NewPlatform(120, 124, 48, 8),
	}
}

func (g *Game) Init() error { return nil }

func (g *Game) Update() error {
	g.beat.Update(time.Now())
	g.player.Update(g.beat, g.platform, groundY)
	// bonus de ritmo: aterrizo sobre la plataforma dentro del beat
	if g.player.landingBonus > 0 {
		g.score += g.player.landingBonus
		g.combo++
	}
	if g.beat.JustTicked() {
		g.beeper.Tick()
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{13, 11, 22, 255}) // fondo #0D0B16
	// suelo
	vector.DrawFilledRect(screen, 0, float32(groundY), ScreenW, 4,
		color.RGBA{74, 205, 123, 255}, false)
	// plataforma (oscila con el beat)
	g.platform.Draw(screen, g.beat.phase)
	// jugador
	g.player.Draw(screen)
	// HUD
	mx, my := ebiten.CursorPosition()
	ebitenutil.DebugPrint(screen, fmt.Sprintf(
		"ritmo-del-barro  score=%d  combo=%d  bpm=%.0f  phase=%.2f  tick=%v  mx=%d my=%d",
		g.score, g.combo, g.beat.bpm, g.beat.phase, g.beat.JustTicked(), mx, my))
}

func (g *Game) Layout(w, h int) (int, int) { return ScreenW, ScreenH }
