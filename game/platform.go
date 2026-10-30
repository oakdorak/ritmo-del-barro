package game

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Platform oscila verticalmente al ritmo del beat. La "pickup window" —
// aterrizar encima dentro del beat — es el núcleo del reto Explícita (timing).
type Platform struct {
	x, w, h float64
	baseY   float64
	amp     float64 // px de desplazamiento (0.5*amp arriba / abajo del base)
}

func NewPlatform(x, baseY, w, h float64) *Platform {
	return &Platform{x: x, baseY: baseY, amp: 28, w: w, h: h}
}

// top devuelve la Y superior, oscilando con la fase del beat [0..1].
func (p *Platform) top(phase float64) float64 {
	return p.baseY - p.amp*math.Sin(2*math.Pi*phase)
}

func (p *Platform) Draw(screen *ebiten.Image, phase float64) {
	t := p.top(phase)
	vector.DrawFilledRect(screen, float32(p.x), float32(t), float32(p.w), float32(p.h),
		color.RGBA{110, 186, 222, 255}, false) // teal
}
