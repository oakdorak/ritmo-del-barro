package game

import (
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Player: física de XOLO (gravity, coyote time, jump-buffer, altura variable)
// + dash. Input dual: teclado (desktop) y zonas táctiles (iPad → touch mapea
// a CursorPosition en WASM, donde touch=mouse move + click).
type Player struct {
	x, y, vx, vy float64
	w, h         float64
	onGround     bool
	coyote       time.Duration
	jumpBuf      time.Duration
	facing       int
	dashT        time.Duration
	landingBonus int // 1 si aterrizo sobre la plataforma dentro del beat
}

func NewPlayer() *Player {
	return &Player{x: 44, y: 120, w: 8, h: 12, facing: 1}
}

// frameMs es el paso de tiempo del physics (~1 frame @60Hz). Aritmética
// entera de Duration: evita el bug de convertir float64 constante a Duration.
const frameMs = 16 * time.Millisecond

func (p *Player) Update(b *Beat, plat *Platform, groundY float64) {
	const (
		gravity   = 0.48 // px/frame²
		maxFall   = 7.5
		accel     = 0.24
		spd       = 1.9 // px/frame
		jumpSpeed = 6.4
		dashSpeed = 11.0
	)

	// input: teclado
	move := 0.0
	if ebiten.IsKeyPressed(ebiten.KeyA) || ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
		move -= 1
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) || ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
		move += 1
	}
	// input: táctil (izq • der, top-center tap = salto)
	tMove, tapJump := touchInput()
	move += tMove

	// --- dash ---
	if p.dashT > 0 {
		p.dashT -= frameMs
		if p.dashT < 0 {
			p.dashT = 0
		}
	}
	if p.dashT > 0 {
		p.vx = float64(p.facing) * dashSpeed
	} else {
		if move != 0 {
			p.facing = int(math.Copysign(1, move))
			p.vx += move * accel * 6
			if p.vx > spd {
				p.vx = spd
			}
			if p.vx < -spd {
				p.vx = -spd
			}
		} else {
			p.vx *= 0.86
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyShift) && p.dashT <= 0 {
		p.dashT = 24 * frameMs
		p.vy = 0
	}

	// --- salto (jump-buffer + coyote) ---
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) ||
		inpututil.IsKeyJustPressed(ebiten.KeyW) ||
		inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) || tapJump {
		p.jumpBuf = 120 * time.Millisecond
	}
	if p.jumpBuf > 0 {
		p.jumpBuf -= frameMs
		if p.jumpBuf > 0 && (p.onGround || p.coyote > 0) {
			p.jumpBuf = 0
			p.coyote = 0
			p.vy = -jumpSpeed
			p.onGround = false
		}
	}

	// --- gravedad ---
	if !p.onGround {
		p.vy += gravity
	}
	if p.vy > maxFall {
		p.vy = maxFall
	}

	// --- integración (px/frame) ---
	p.x += p.vx
	p.y += p.vy
	// wrap horizontal simple
	if p.x < 0 {
		p.x = 0
		p.vx *= 0.5
	}
	if p.x+p.w > ScreenW {
		p.x = ScreenW - p.w
		p.vx *= 0.5
	}

	// --- colisión + bonus ---
	p.resolveCollision(groundY, plat, b)
}

// resolveCollision contra plataforma (cima) y suelo; otorga bonus si
// aterrizo sobre la plataforma dentro de la ventana de beat.
func (p *Player) resolveCollision(groundY float64, plat *Platform, b *Beat) {
	wasGrounded := p.onGround
	p.onGround = false

	// plataforma: caída cruzando su cima
	if plat != nil && p.vy >= 0 &&
		p.x+p.w > plat.x && p.x < plat.x+plat.w &&
		p.y+p.h >= plat.top(b.phase) && p.y < plat.top(b.phase) {
		p.y = plat.top(b.phase) - p.h
		p.vy = 0
		p.onGround = true
	}

	// suelo
	if p.y+p.h >= groundY && p.vy >= 0 {
		p.y = groundY - p.h
		p.vy = 0
		p.onGround = true
	}

	// coyote al dejar superficie
	if wasGrounded && !p.onGround {
		p.coyote = 120 * time.Millisecond
	}
	if p.onGround {
		p.coyote = 0
	}

	// bonus de ritmo
	if p.onGround && !wasGrounded && plat != nil &&
		p.x+p.w > plat.x && p.x < plat.x+plat.w && b.InWindow() {
		p.landingBonus = 1
	} else {
		p.landingBonus = 0
	}
}

func (p *Player) Draw(screen *ebiten.Image) {
	vector.DrawFilledRect(screen, float32(p.x), float32(p.y), float32(p.w), float32(p.h),
		color.RGBA{235, 230, 166, 255}, false) // oro
}

// touchInput divide la pantalla lógica (320x180): izq → -1, der → +1,
// top-center tap → salto. CursorPosition refleja el toque en WASM/iOS.
func touchInput() (move float64, jump bool) {
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		return 0, false
	}
	mx, my := ebiten.CursorPosition()
	if my < 70 && mx > 110 && mx < 210 { // top-center = salto
		return 0, true
	}
	if mx < 110 { // tercio izquierdo
		return -1, false
	}
	if mx > 210 { // tercio derecho
		return 1, false
	}
	return 0, false
}
