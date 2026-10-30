package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"math"
	"math/rand"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// ─── SCREEN / PHYSICS CONSTANTS (from XOLO base) ──────────────────────────

const (
	SCREEN_WIDTH  = 800
	SCREEN_HEIGHT = 480
	TILE_SIZE     = 16
	SCALE         = 3
	GRAVITY       = 0.55
	JUMP_FORCE    = -10.0
	MOVE_SPEED    = 3.0
	MAX_FALL      = 11.0
	COYOTE_TIME   = 6
	JUMP_BUFFER   = 8

	DASH_SPEED       = 12.0
	DASH_DURATION    = 8
	DASH_COOLDOWN    = 45

	MAX_JUMPS        = 2
	WALL_SLIDE_SPEED = 2.0
	WALL_JUMP_X      = -6.0
	WALL_JUMP_Y      = -9.0

	SPRITE_W = 16
	SPRITE_H = 24

	SAMPLE_RATE = 44100
)

// ─── TILE TYPES ────────────────────────────────────────────────────────────

const (
	TILE_EMPTY  = 255
	TILE_DIRT   = 0
	TILE_STONE  = 1
	TILE_MOSS   = 2
	TILE_SPIKE  = 3
	TILE_GRASS  = 4
	TILE_WOOD   = 5
	TILE_LEAF   = 6
	TILE_BRICK  = 7
	TILE_DARK   = 8
	TILE_WATER  = 10
	TILE_LAVA   = 11
	TILE_PLAT   = 12
	TILE_MUD    = 13 // NEW: mud tile — slow movement
)

var tileAnimConfig = map[uint8]struct{ frames, speed int }{
	TILE_WATER: {frames: 4, speed: 12},
	TILE_LAVA:  {frames: 4, speed: 10},
	TILE_PLAT:  {frames: 2, speed: 20},
}

// ─── RHYTHM SYSTEM ─────────────────────────────────────────────────────────

type RhythmSystem struct {
	bpm                float64
	beatDuration       float64 // in seconds per beat
	beatInterval       int     // frames per beat (at 60 FPS)
	currentBeat        int
	beatTimer          int
	lastBeatTime       float64
	timingWindow       int // frames of grace for input
	gameStarted        bool
	songDuration       int // total beats in current song
	metronomeCounter   int
}

const (
	GAME_FPS = 60
	// Timing windows (in frames, at 60 FPS)
	TIMING_PERFECT = 5  // ±5 frames
	TIMING_GREAT   = 10 // ±10 frames
	TIMING_OKAY    = 20 // ±20 frames
)

func NewRhythmSystem(bpm float64) *RhythmSystem {
	return &RhythmSystem{
		bpm:          bpm,
		beatDuration: 60.0 / bpm,
		beatInterval: int(math.Round(60.0 / bpm * GAME_FPS)),
		timingWindow: TIMING_OKAY,
		gameStarted:  false,
	}
}

func (r *RhythmSystem) Start() {
	r.gameStarted = true
	r.currentBeat = 0
	r.beatTimer = 0
	r.lastBeatTime = 0
}

func (r *RhythmSystem) Update() {
	if !r.gameStarted {
		return
	}
	r.beatTimer++
	if r.beatTimer >= r.beatInterval {
		r.beatTimer = 0
		r.currentBeat++
		r.lastBeatTime = float64(r.currentBeat) * r.beatDuration
	}
}

// IsOnBeat returns the distance in frames from the nearest beat
func (r *RhythmSystem) BeatDistance() int {
	if r.beatInterval == 0 {
		return 0
	}
	half := r.beatInterval / 2
	if r.beatTimer > half {
		return r.beatTimer - r.beatInterval
	}
	return r.beatTimer
}

func (r *RhythmSystem) IsBeat() bool {
	return r.beatTimer == 0 && r.currentBeat > 0
}

func (r *RhythmSystem) GetTimingRating() string {
	dist := abs(r.BeatDistance())
	if dist <= TIMING_PERFECT {
		return "PERFECT"
	} else if dist <= TIMING_GREAT {
		return "GREAT"
	} else if dist <= TIMING_OKAY {
		return "OKAY"
	}
	return "MISS"
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ─── PROCEDURAL RHYTHM AUDIO ───────────────────────────────────────────────

func generateBassNote(freq float64, duration float64) []float64 {
	samples := int(float64(SAMPLE_RATE) * duration)
	result := make([]float64, samples)
	for i := 0; i < samples; i++ {
		t := float64(i) / float64(SAMPLE_RATE)
		tA := float64(i) / float64(samples)
		// Square wave with pulse-width modulation for a "muddy" character
		pw := 0.4 + 0.1*math.Sin(2*math.Pi*3*t)
		wave := 1.0
		if math.Sin(2*math.Pi*freq*t) < 0 {
			wave = -1.0
		}
		if math.Sin(2*math.Pi*freq*t) < pw-1 {
			wave = 0
		}
		// Exponential decay envelope
		envelope := math.Exp(-tA * 4.0)
		result[i] = wave * envelope * 0.35
	}
	return result
}

func generateHihat(duration float64) []float64 {
	samples := int(float64(SAMPLE_RATE) * duration)
	result := make([]float64, samples)
	for i := 0; i < samples; i++ {
		tA := float64(i) / float64(samples)
		// White noise with high-pass filter for hi-hat character
		noise := rand.Float64()*2 - 1
		envelope := math.Exp(-tA * 12.0)
		result[i] = noise * envelope * 0.15
	}
	return result
}

func generateSnare(duration float64) []float64 {
	samples := int(float64(SAMPLE_RATE) * duration)
	result := make([]float64, samples)
	for i := 0; i < samples; i++ {
		t := float64(i) / float64(SAMPLE_RATE)
		tA := float64(i) / float64(samples)
		// Snare = low noise burst + high ring
		noise := rand.Float64()*2 - 1
		ring := math.Sin(2 * math.Pi * 220 * t)
		envelope := math.Exp(-tA * 8.0)
		result[i] = (noise*0.7 + ring*0.3) * envelope * 0.25
	}
	return result
}

func generateKick(duration float64) []float64 {
	samples := int(float64(SAMPLE_RATE) * duration)
	result := make([]float64, samples)
	for i := 0; i < samples; i++ {
		t := float64(i) / float64(SAMPLE_RATE)
		tA := float64(i) / float64(samples)
		// Downward pitch sweep for kick drum
		freq := 80.0 * math.Exp(-tA * 3.0)
		wave := math.Sin(2 * math.Pi * freq * t)
		envelope := math.Exp(-tA * 6.0)
		result[i] = wave * envelope * 0.45
	}
	return result
}

func generateTick(duration float64) []float64 {
	samples := int(float64(SAMPLE_RATE) * duration)
	result := make([]float64, samples)
	for i := 0; i < samples; i++ {
		t := float64(i) / float64(SAMPLE_RATE)
		tA := float64(i) / float64(samples)
		freq := 1000.0
		wave := math.Sin(2 * math.Pi * freq * t)
		envelope := math.Exp(-tA * 20.0)
		result[i] = wave * envelope * 0.20
	}
	return result
}

// ─── SOUND MANAGER ─────────────────────────────────────────────────────────

type SoundManager struct {
	ctx     *audio.Context
	players map[string]*audio.Player
}

func NewSoundManager() *SoundManager {
	sm := &SoundManager{
		ctx:     audio.NewContext(SAMPLE_RATE),
		players: make(map[string]*audio.Player),
	}
	sm.generateSounds()
	return sm
}

func (sm *SoundManager) generateSounds() {
	sounds := map[string][]float64{
		"jump":      generateToneSlide(300, 600, 0.08),
		"djump":     generateToneSlide(600, 1200, 0.05),
		"dash":      generateNoise(0.06),
		"land":      generateToneSlide(150, 80, 0.1),
		"death":     generateToneSlide(400, 100, 0.25),
		"wallslide": generateToneSlide(200, 150, 0.05),
		"kick":      generateKick(0.08),
		"snare":     generateSnare(0.08),
		"hihat":     generateHihat(0.04),
		"bass":      generateBassNote(82.5, 0.15),  // E2 (low E)
		"bass_g":    generateBassNote(98.0, 0.15),  // G2
		"bass_d":    generateBassNote(73.5, 0.15),  // D2
		"bass_a":    generateBassNote(110.0, 0.15), // A2
		"tick":      generateTick(0.02),
	}

	for name, samples := range sounds {
		wav := encodeWAV(samples)
		reader := bytes.NewReader(wav)
		player, err := sm.ctx.NewPlayer(reader)
		if err == nil {
			sm.players[name] = player
		}
	}
}

func (sm *SoundManager) Play(name string) {
	if p, ok := sm.players[name]; ok {
		p.Rewind()
		p.Play()
	}
}

func (sm *SoundManager) PlayLoop(name string) {
	if p, ok := sm.players[name]; ok {
		p.Rewind()
		p.SetVolume(0.3)
		p.Play()
	}
}

func generateTone(startFreq, endFreq float64, duration float64) []float64 {
	return generateToneSlide(startFreq, endFreq, duration)
}

func generateToneSlide(startFreq, endFreq float64, duration float64) []float64 {
	samples := int(float64(SAMPLE_RATE) * duration)
	result := make([]float64, samples)
	for i := 0; i < samples; i++ {
		t := float64(i) / float64(samples)
		freq := startFreq + (endFreq-startFreq)*t
		envelope := 1.0 - t
		if i < samples/10 {
			envelope = float64(i) / float64(samples/10)
		}
		result[i] = math.Sin(2*math.Pi*freq*float64(i)/float64(SAMPLE_RATE)) * envelope * 0.3
	}
	return result
}

func generateNoise(duration float64) []float64 {
	samples := int(float64(SAMPLE_RATE) * duration)
	result := make([]float64, samples)
	for i := 0; i < samples; i++ {
		t := float64(i) / float64(samples)
		envelope := 1.0 - t
		if i < samples/10 {
			envelope = float64(i) / float64(samples/10)
		}
		result[i] = (rand.Float64()*2 - 1) * envelope * 0.2
	}
	return result
}

func encodeWAV(samples []float64) []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, []byte("RIFF"))
	binary.Write(buf, binary.LittleEndian, uint32(36+len(samples)*2))
	binary.Write(buf, binary.LittleEndian, []byte("WAVEfmt "))
	binary.Write(buf, binary.LittleEndian, uint32(16))
	binary.Write(buf, binary.LittleEndian, uint16(1))
	binary.Write(buf, binary.LittleEndian, uint16(1))
	binary.Write(buf, binary.LittleEndian, uint32(SAMPLE_RATE))
	binary.Write(buf, binary.LittleEndian, uint32(SAMPLE_RATE*2))
	binary.Write(buf, binary.LittleEndian, uint16(2))
	binary.Write(buf, binary.LittleEndian, uint16(16))
	binary.Write(buf, binary.LittleEndian, []byte("data"))
	binary.Write(buf, binary.LittleEndian, uint32(len(samples)*2))
	for _, s := range samples {
		val := int16(s * 32767)
		binary.Write(buf, binary.LittleEndian, val)
	}
	return buf.Bytes()
}

// ─── PLAYER ────────────────────────────────────────────────────────────────

type Player struct {
	x, y           float64
	vx, vy         float64
	width, height  float64
	onGround       bool
	coyoteTimer    int
	jumpBuffer     int
	facingRight    bool
	jumpHeld       bool

	jumpsLeft    int
	dashTimer    int
	dashCooldown int
	dashDirX     float64
	dashDirY     float64
	isDashing    bool
	wasOnGround  bool

	animFrame int
	animTimer int
	dead      bool
	deathTimer int
	respawnX  float64
	respawnY  float64

	inMud bool
}

func NewPlayer(x, y float64) *Player {
	p := &Player{
		x:          x,
		y:          y,
		width:      TILE_SIZE - 2,
		height:     TILE_SIZE * 2,
		respawnX:   x,
		respawnY:   y,
	}
	p.wasOnGround = false
	return p
}

func (p *Player) Update(keys map[ebiten.Key]bool, tiles []RhythmTile) {
	if p.dead {
		if p.deathTimer > 0 {
			p.deathTimer--
			return
		}
		p.respawn()
		return
	}

	p.handleInput(keys)
	p.applyPhysics()
	p.checkCollisions(tiles)
	p.updateTimers(keys)
	p.updateAnimation()
}

func (p *Player) handleInput(keys map[ebiten.Key]bool) {
	p.vx = 0
	p.jumpHeld = keys[ebiten.KeySpace] || keys[ebiten.KeyW] || keys[ebiten.KeyArrowUp]

	if p.isDashing {
		p.vx = p.dashDirX * DASH_SPEED
		p.vy = p.dashDirY * DASH_SPEED
		return
	}

	moveSpeed := MOVE_SPEED
	if p.inMud {
		moveSpeed *= 0.5
	}

	if keys[ebiten.KeyArrowRight] || keys[ebiten.KeyD] {
		p.vx = moveSpeed
		p.facingRight = true
	}
	if keys[ebiten.KeyArrowLeft] || keys[ebiten.KeyA] {
		p.vx = -moveSpeed
		p.facingRight = false
	}

	// Dash (Shift)
	if keys[ebiten.KeyShift] && p.dashCooldown <= 0 && p.dashTimer <= 0 {
		p.dashTimer = DASH_DURATION
		p.dashCooldown = DASH_COOLDOWN
		p.isDashing = true
		p.dashDirX = 0
		p.dashDirY = 0
		if keys[ebiten.KeyArrowRight] || keys[ebiten.KeyD] {
			p.dashDirX = 1
		}
		if keys[ebiten.KeyArrowLeft] || keys[ebiten.KeyA] {
			p.dashDirX = -1
		}
		if keys[ebiten.KeyW] || keys[ebiten.KeyArrowUp] || keys[ebiten.KeySpace] {
			p.dashDirY = -1
		} else if keys[ebiten.KeyS] || keys[ebiten.KeyArrowDown] {
			p.dashDirY = 1
		}
		if p.dashDirX == 0 && p.dashDirY == 0 {
			if p.facingRight {
				p.dashDirX = 1
			} else {
				p.dashDirX = -1
			}
		}
		norm := math.Hypot(p.dashDirX, p.dashDirY)
		if norm > 0 {
			p.dashDirX /= norm
			p.dashDirY /= norm
		}
	}

	// Jump buffer
	if keys[ebiten.KeySpace] || keys[ebiten.KeyW] || keys[ebiten.KeyArrowUp] {
		p.jumpBuffer = JUMP_BUFFER
	}
}

func (p *Player) applyPhysics() {
	p.vy += GRAVITY
	if p.vy > MAX_FALL {
		p.vy = MAX_FALL
	}

	// Wall slide
	if !p.onGround && p.vy > 0 {
		for _, t := range gameTiles {
			if !t.solid || t.death {
				continue
			}
			if p.isWallSliding(t) {
				p.vy = WALL_SLIDE_SPEED
				break
			}
		}
	}

	p.x += p.vx
	p.y += p.vy
}

func (p *Player) isWallSliding(t RhythmTile) bool {
	touching := p.x < t.x+TILE_SIZE && p.x+p.width > t.x &&
		p.y < t.y+TILE_SIZE && p.y+p.height > t.y
	if !touching {
		return false
	}
	left := p.x < t.x+TILE_SIZE/2 && p.vx < 0
	right := p.x+p.width > t.x+TILE_SIZE/2 && p.vx > 0
	return left || right
}

func (p *Player) checkCollisions(tiles []RhythmTile) {
	p.wasOnGround = p.onGround
	p.onGround = false
	p.inMud = false
	for _, t := range tiles {
		if !t.solid {
			continue
		}
		if p.overlaps(t) {
			overlapX, overlapY := p.overlapDepth(t)
			if overlapX < overlapY {
				if p.vx > 0 {
					p.x = t.x - p.width
				} else if p.vx < 0 {
					p.x = t.x + TILE_SIZE
				}
				p.vx = 0
			} else {
				if p.vy > 0 {
					p.y = t.y - p.height
					p.vy = 0
					p.onGround = true
				} else if p.vy < 0 {
					p.y = t.y + TILE_SIZE
					p.vy = 0
				}
			}
			// Check for mud (slow tiles)
			if t.tileType == TILE_MUD {
				p.inMud = true
			}
		}
	}

	// Death tiles
	for _, t := range tiles {
		if !t.death {
			continue
		}
		if p.overlaps(t) {
			p.die()
			return
		}
	}

	// Respawn if fell off map
	if p.y > float64(len(levelMap)*TILE_SIZE) + 64 {
		p.die()
	}
}

func (p *Player) die() {
	p.dead = true
	p.deathTimer = 30
	p.vx = 0
	p.vy = 0
	p.dashTimer = 0
	p.isDashing = false
	p.jumpsLeft = MAX_JUMPS
}

func (p *Player) respawn() {
	p.dead = false
	p.deathTimer = 0
	p.x = p.respawnX
	p.y = p.respawnY
	p.vx = 0
	p.vy = 0
	p.onGround = false
	p.coyoteTimer = 0
	p.jumpBuffer = 0
	p.jumpsLeft = MAX_JUMPS
	p.dashTimer = 0
	p.dashCooldown = 0
	p.isDashing = false
	p.animFrame = 0
	p.animTimer = 0
}

func (p *Player) updateTimers(keys map[ebiten.Key]bool) {
	if p.onGround {
		p.coyoteTimer = COYOTE_TIME
		p.jumpsLeft = MAX_JUMPS
	} else {
		if p.coyoteTimer > 0 {
			p.coyoteTimer--
		}
	}

	if p.jumpBuffer > 0 {
		p.jumpBuffer--
	}

	if p.dashTimer > 0 {
		p.dashTimer--
		if p.dashTimer <= 0 {
			p.isDashing = false
		}
	}
	if p.dashCooldown > 0 {
		p.dashCooldown--
	}

	// Jump execution
	if p.jumpBuffer > 0 && p.coyoteTimer > 0 {
		p.vy = JUMP_FORCE
		p.coyoteTimer = 0
		p.jumpBuffer = 0
		p.onGround = false
		p.jumpsLeft--
	} else if p.jumpBuffer > 0 && p.jumpsLeft > 0 && !p.isDashing {
		p.vy = JUMP_FORCE * 0.9
		p.jumpBuffer = 0
		p.jumpsLeft--
	}

	// Variable jump height
	if !p.jumpHeld && p.vy < -2 {
		p.vy *= 0.7
	}

	// Wall jump
	if !p.onGround && p.vy > 0 {
		for _, t := range gameTiles {
			if !t.solid || t.death {
				continue
			}
			if p.isWallSliding(t) && p.jumpBuffer > 0 {
				p.vy = WALL_JUMP_Y
				p.vx = WALL_JUMP_X
				if p.x+p.width/2 < t.x+TILE_SIZE/2 {
					p.vx = -WALL_JUMP_X
				}
				p.jumpBuffer = 0
				p.jumpsLeft = MAX_JUMPS - 1
				p.isDashing = false
				p.dashTimer = 0
				break
			}
		}
	}
}

func (p *Player) updateAnimation() {
	p.animTimer++
	if p.animTimer > 8 {
		p.animTimer = 0
		p.animFrame++
		if p.animFrame > 3 {
			p.animFrame = 0
		}
	}
}

func (p *Player) overlaps(t RhythmTile) bool {
	return p.x < t.x+TILE_SIZE &&
		p.x+p.width > t.x &&
		p.y < t.y+TILE_SIZE &&
		p.y+p.height > t.y
}

func (p *Player) overlapDepth(t RhythmTile) (float64, float64) {
	dx := math.Min(p.x+p.width-t.x, t.x+TILE_SIZE-p.x)
	dy := math.Min(p.y+p.height-t.y, t.y+TILE_SIZE-p.y)
	return dx, dy
}

// ─── RHYTHM TILE ───────────────────────────────────────────────────────────

type RhythmTile struct {
	x, y     float64
	solid    bool
	death    bool
	tileType uint8
	col      color.RGBA
	hasDetail bool

	// Rhythm-specific
	beatSync  bool   // activates on beat
	beatPhase int    // which beat in a cycle it activates
	beatTimer int    // frames until next state change
	active    bool   // current state (for beatSync tiles)
	cycleBeat int    // beat cycle length (e.g., every 4 beats)
}

// ─── PARTICLE ──────────────────────────────────────────────────────────────

type Particle struct {
	x, y      float64
	vx, vy    float64
	life      int
	maxLife   int
	col       color.RGBA
	size      float64
}

// ─── LEVEL DATA ────────────────────────────────────────────────────────────

const (
	LEVEL_WIDTH  = 80
	LEVEL_HEIGHT = 40
)

var levelMap = []string{
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"........................................................~~~~~...................",
	"....................................................................~~~~~~...",
	".......##.....................##.....##...............##...................",
	"................................................#...#.......#...#.............",
	"..............................................................#...#..........",
	".............................###........###........###......................",
	"................................................................................",
	"========..........######.......#######........#######........==========....",
}

var deathMap = []string{
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"VVVVV.........................................VVVVV......................",
	"................................................................................",
	"................................................................................",
}

// NEW: Rhythm levels — each has a BPM and special beat-sync tiles
type RhythmLevel struct {
	name         string
	bpm          float64
	author       string
	levelMap     []string
	deathMap     []string
	rhythmMap    []string // special tiles for rhythm mechanics
	mudMap       []string // mud tiles
	startX       float64
	startY       float64
	goalX        float64
	goalY        float64
	song         []BeatEvent
}

type BeatEvent struct {
	beat    int     // which beat this event occurs
	char    rune    // symbol in rhythmMap
	freq    float64 // frequency for bass note on this beat
}

// Rhythm level definitions
var rhythmLevels = []RhythmLevel{
	{
		name:     "Charco Congelado",
		bpm:      120.0,
		author:   "oakdorak",
		startX:   50,
		startY:   100,
		goalX:    1240,
		goalY:    240,
		rhythmMap: []string{
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"........................................................................~~~~~~~",
			"....................................................................~~~~......",
			".......##.....................##.....##...............##...................",
			"................................................#...#.......#...#.............",
			"..............................................................#...#..........",
			".............................###........###........###......................",
			"................................................................................",
			"===M===M===M===M===M===M===M===M===M===M===M===M===M===M===M===M===M===",
		},
		mudMap: []string{
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			".......##.....................##.....##...............##...................",
			"................................................#...#.......#...#.............",
			"..............................................................#...#..........",
			".............................###........###........###......................",
			"................................................................................",
			"MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM",
		},
		levelMap: levelMap,
		deathMap: deathMap,
		song: []BeatEvent{
			{beat: 0, char: 'K', freq: 82.5},   // kick
			{beat: 1, char: 'S', freq: 110.0},  // snare
			{beat: 2, char: 'H', freq: 0},     // hihat
			{beat: 3, char: 'S', freq: 110.0},  // snare
			{beat: 4, char: 'K', freq: 82.5},   // kick
			{beat: 5, char: 'H', freq: 0},
			{beat: 6, char: 'K', freq: 98.0},  // kick on G
			{beat: 7, char: 'H', freq: 0},
			{beat: 8, char: 'K', freq: 110.0}, // kick on A
			{beat: 9, char: 'S', freq: 110.0},
			{beat: 10, char: 'H', freq: 0},
			{beat: 11, char: 'S', freq: 110.0},
			{beat: 12, char: 'K', freq: 82.5},
			{beat: 13, char: 'K', freq: 98.0},
			{beat: 14, char: 'H', freq: 0},
			{beat: 15, char: 'S', freq: 110.0},
		},
	},
	{
		name:     "Resbaladilla",
		bpm:      140.0,
		author:   "oakdorak",
		startX:   50,
		startY:   100,
		goalX:    1180,
		goalY:    260,
		rhythmMap: []string{
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"...................................~~~~~~~....~~~~~~~...~~~~~~~.............",
			"....#...#.......#...#.......#...#.......#...#.......#...#.......#...#.....",
			"............................................................................",
			"............R.......R................R.......R................R..........",
			"............................................................................",
			"===M===M===M===M===M===M===M===M===M===M===M===M===M===M===M===M===M===",
		},
		mudMap: []string{
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"....#...#.......#...#.......#...#.......#...#.......#...#.......#...#.....",
			"MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM",
			"............R.......R................R.......R................R..........",
			"MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM",
			"..................................................................~~~~~~~",
			"MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM",
		},
		levelMap: levelMap,
		deathMap: deathMap,
		song: []BeatEvent{
			{beat: 0, char: 'K', freq: 130.8},
			{beat: 1, char: 'S', freq: 0},
			{beat: 2, char: 'H', freq: 0},
			{beat: 3, char: 'S', freq: 0},
			{beat: 4, char: 'K', freq: 130.8},
			{beat: 5, char: 'K', freq: 146.8},
			{beat: 6, char: 'H', freq: 0},
			{beat: 7, char: 'K', freq: 164.8},
			{beat: 8, char: 'S', freq: 0},
			{beat: 9, char: 'H', freq: 0},
			{beat: 10, char: 'K', freq: 174.6},
			{beat: 11, char: 'S', freq: 0},
			{beat: 12, char: 'K', freq: 130.8},
			{beat: 13, char: 'H', freq: 0},
			{beat: 14, char: 'K', freq: 164.8},
			{beat: 15, char: 'S', freq: 0},
		},
	},
	{
		name:     "Barro Vivo",
		bpm:      160.0,
		author:   "oakdorak",
		startX:   50,
		startY:   100,
		goalX:    1260,
		goalY:    220,
		rhythmMap: []string{
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"~~~~~~~.........................................................................",
			"~~~~~~~...#...#...#...#...#...#...#...#...#...#...#...#...#...#...#.......",
			"............................................................................",
			"........................P...P...P...P...P...P...P...P...................",
			"............................................................................",
			"===M===M===M===M===M===M===M===M===M===M===M===M===M===M===M===M===M===",
		},
		mudMap: []string{
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"................................................................................",
			"MMMMMMM.......................................................................",
			"MMMMMMM...M...M...M...M...M...M...M...M...M...M...M...M...M...M...M.....",
			"MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM",
			"........................P...P...P...P...P...P...P...P...................",
			"MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM",
			"..................................................................~~~~~~~",
			"MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM",
		},
		levelMap: levelMap,
		deathMap: deathMap,
		song: []BeatEvent{
			{beat: 0, char: 'K', freq: 65.4},  // F2
			{beat: 0, char: 'H', freq: 0},
			{beat: 1, char: 'K', freq: 73.4},  // G2
			{beat: 1, char: 'H', freq: 0},
			{beat: 2, char: 'K', freq: 65.4},
			{beat: 2, char: 'H', freq: 0},
			{beat: 2, char: 'S', freq: 0},
			{beat: 3, char: 'K', freq: 82.5},  // E2
			{beat: 3, char: 'H', freq: 0},
			{beat: 3, char: 'S', freq: 0},
			{beat: 4, char: 'K', freq: 98.0},  // G2
			{beat: 4, char: 'H', freq: 0},
			{beat: 5, char: 'K', freq: 98.0},
			{beat: 5, char: 'S', freq: 0},
			{beat: 6, char: 'K', freq: 110.0}, // A2
			{beat: 6, char: 'H', freq: 0},
			{beat: 7, char: 'K', freq: 82.5},
			{beat: 7, char: 'S', freq: 0},
		},
	},
}

// ─── GAME STRUCT ───────────────────────────────────────────────────────────

type Game struct {
	player      *Player
	tiles       []RhythmTile
	particles   []Particle
	cameraX     float64
	cameraY     float64
	keys        map[ebiten.Key]bool
	frameCount  int
	animFrame   int
	sound       *SoundManager
	deaths      int

	// Rhythm system
	rhythm      *RhythmSystem
	level       *RhythmLevel
	levelIdx    int
	gameStarted bool
	score       int
	combo       int
	bestCombo    int
	lastRating    string
	ratingTimer   int
	goalReached   bool
	prevDead      bool
	prevJumpsLeft int
	spawnIdx      int

	tilesetImg  *ebiten.Image
	spritesImg  *ebiten.Image
}

var gameTiles []RhythmTile

func NewGame() *Game {
	g := &Game{
		keys:     make(map[ebiten.Key]bool),
		levelIdx: 0,
	}
	g.level = &rhythmLevels[0]
	g.rhythm = NewRhythmSystem(g.level.bpm)

	// Parse level
	g.parseLevel()
	gameTiles = g.tiles

	// Load assets
	img, _, err := ebitenutil.NewImageFromFile("tileset.png")
	if err != nil {
		// Generate a fallback tileset procedurally
		g.tilesetImg = generateFallbackTileset()
	} else {
		g.tilesetImg = img
	}

	sprImg, _, err := ebitenutil.NewImageFromFile("sprites.png")
	if err != nil {
		g.spritesImg = generateFallbackSprites()
	} else {
		g.spritesImg = sprImg
	}

	g.sound = NewSoundManager()
	g.player = NewPlayer(g.level.startX, g.level.startY)
	g.player.respawnX = g.level.startX
	g.player.respawnY = g.level.startY

	rand.Seed(time.Now().UnixNano())
	return g
}

func (g *Game) parseLevel() {
	// Parse from level-specific maps
	levelMap = g.level.levelMap
	deathMap = g.level.deathMap
	rhythmMap := g.level.rhythmMap
	mudMap := g.level.mudMap

	for y, row := range levelMap {
		for x, ch := range row {
			tx := float64(x * TILE_SIZE)
			ty := float64(y * TILE_SIZE)
			switch ch {
			case '#':
				g.tiles = append(g.tiles, RhythmTile{x: tx, y: ty, solid: true, tileType: TILE_DIRT, hasDetail: true, col: color.RGBA{76, 55, 94, 255}})
			case '~':
				g.tiles = append(g.tiles, RhythmTile{x: tx, y: ty, solid: false, tileType: TILE_WATER, death: true, hasDetail: true, col: color.RGBA{50, 100, 200, 255}})
			case '=':
				g.tiles = append(g.tiles, RhythmTile{x: tx, y: ty, solid: true, tileType: TILE_PLAT, hasDetail: true, col: color.RGBA{120, 90, 160, 255}})
			}
		}
	}

	// Parse death spikes from deathMap
	for y, row := range deathMap {
		for x, ch := range row {
			if ch != 'V' {
				continue
			}
			tx := float64(x * TILE_SIZE)
			ty := float64(y * TILE_SIZE)
			g.tiles = append(g.tiles, RhythmTile{x: tx, y: ty, solid: false, tileType: TILE_SPIKE, death: true, hasDetail: true, col: color.RGBA{220, 80, 80, 255}})
		}
	}

	// Parse mud tiles from mudMap
	for y, row := range mudMap {
		for x, ch := range row {
			if ch != 'M' {
				continue
			}
			tx := float64(x * TILE_SIZE)
			ty := float64(y * TILE_SIZE)
			g.tiles = append(g.tiles, RhythmTile{x: tx, y: ty, solid: true, tileType: TILE_MUD, hasDetail: true, col: color.RGBA{110, 80, 120, 255}})
		}
	}

	// Parse rhythm tiles (platforms that activate on beat: R)
	for y, row := range rhythmMap {
		for x, ch := range row {
			if ch != 'R' {
				continue
			}
			tx := float64(x * TILE_SIZE)
			ty := float64(y * TILE_SIZE)
			g.tiles = append(g.tiles, RhythmTile{
				x: tx, y: ty,
				solid: false,
				tileType: TILE_STONE,
				hasDetail: true,
				col: color.RGBA{180, 140, 200, 255},
				beatSync: true,
				cycleBeat: 2, // activate every 2 beats
				active: false,
			})
		}
	}
}

func (g *Game) tileFrame(tileType uint8) int {
	cfg, ok := tileAnimConfig[tileType]
	if !ok {
		return 0
	}
	return (g.animFrame / cfg.speed) % cfg.frames
}

func (g *Game) tileIndex(tileType uint8) int {
	switch tileType {
	case TILE_WATER:
		return 10 + g.tileFrame(tileType)
	case TILE_LAVA:
		return 14 + g.tileFrame(tileType)
	case TILE_PLAT:
		return 18 + g.tileFrame(tileType)
	case TILE_MUD:
		return 2 // reuse moss tile visually but with mud color
	case TILE_SPIKE:
		return 3
	default:
		return int(tileType)
	}
}

func (g *Game) Update() error {
	gameTiles = g.tiles
	g.frameCount++
	g.animFrame++

	// Track keys
	g.trackKeys()

	// Update rhythm system
	g.rhythm.Update()

	// Play rhythm audio on beat
	if g.rhythm.IsBeat() && g.gameStarted {
		g.playBeat()
	}

	// Update beat-sync tiles
	for i := range g.tiles {
		if g.tiles[i].beatSync {
			if g.rhythm.beatTimer == 0 {
				g.tiles[i].active = !g.tiles[i].active
			}
			// Update solid state based on active
			g.tiles[i].solid = g.tiles[i].active
		}
	}

	// Check for level start
	if !g.gameStarted {
		if inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			g.gameStarted = true
			g.rhythm.Start()
		}
	}

	// Check for level progression
	if g.goalReached {
		if inpututil.IsKeyJustPressed(ebiten.KeyN) || inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
			g.nextLevel()
		}
	}

	if g.gameStarted && !g.goalReached {
		// Update player
		g.player.Update(g.keys, g.tiles)

		// Check goal
		if g.player.x >= g.level.goalX && g.player.y <= g.level.goalY+32 {
			if abs(int(g.player.x-g.level.goalX)) < 64 {
				g.goalReached = true
				g.sound.Play("land")
			}
		}

		// Spawn particles on dash + play sound
		if g.player.isDashing && g.player.dashTimer == DASH_DURATION-1 {
			for i := 0; i < 6; i++ {
				g.particles = append(g.particles, Particle{
					x: g.player.x + g.player.width/2,
					y: g.player.y + g.player.height/2,
					vx: (rand.Float64() - 0.5) * 4,
					vy: (rand.Float64() - 0.5) * 4,
					life: 15,
					maxLife: 15,
					col: color.RGBA{200, 170, 220, 180},
					size: 2 + rand.Float64()*2,
				})
			}
			g.sound.Play("dash")
		}

		// Spawn particles on landing
		if g.player.onGround && !g.player.wasOnGround && !g.player.dead {
			for i := 0; i < 5; i++ {
				g.particles = append(g.particles, Particle{
					x: g.player.x + g.player.width/2,
					y: g.player.y + g.player.height,
					vx: (rand.Float64() - 0.5) * 3,
					vy: rand.Float64() * -2,
					life: 12,
					maxLife: 12,
					col: color.RGBA{160, 140, 140, 150},
					size: 1 + rand.Float64()*2,
				})
			}
			g.sound.Play("land")
		}

		// Death sound + count
		if g.player.dead && !g.prevDead {
			g.sound.Play("death")
			g.deaths++
			g.combo = 0
		}
		g.prevDead = g.player.dead

		// Jump sounds
		if g.player.jumpsLeft < g.prevJumpsLeft {
			if g.prevJumpsLeft == MAX_JUMPS {
				g.sound.Play("jump")
			} else {
				g.sound.Play("djump")
			}
		}
		g.prevJumpsLeft = g.player.jumpsLeft

		// Rhythm scoring — when player jumps on beat
		if (g.rhythm.beatTimer == 0 || abs(g.rhythm.beatTimer) <= TIMING_PERFECT) &&
			(g.player.wasOnGround && !g.player.onGround) {
			rating := g.rhythm.GetTimingRating()
			g.addScore(rating)
		}

		// Spawn particles on beat
		if g.rhythm.IsBeat() && g.gameStarted {
			for i := 0; i < 3; i++ {
				g.particles = append(g.particles, Particle{
					x: g.player.x + (rand.Float64()-0.5)*40,
					y: g.player.y,
					vx: (rand.Float64() - 0.5) * 2,
					vy: -2 - rand.Float64()*3,
					life: 20,
					maxLife: 20,
					col: color.RGBA{160, 120, 200, 200},
					size: 1 + rand.Float64()*2,
				})
			}
		}

		// Spawn mud particles when in mud
		if g.player.inMud && g.frameCount%5 == 0 {
			g.particles = append(g.particles, Particle{
				x: g.player.x + (rand.Float64()-0.5)*g.player.width,
				y: g.player.y + g.player.height,
				vx: (rand.Float64() - 0.5) * 1,
				vy: 0.5,
				life: 10,
				maxLife: 10,
				col: color.RGBA{120, 90, 140, 200},
				size: 1 + rand.Float64()*1.5,
			})
		}

		// Update particles
		for i := len(g.particles) - 1; i >= 0; i-- {
			p := &g.particles[i]
			p.x += p.vx
			p.y += p.vy
			p.vy += 0.1
			p.life--
			if p.life <= 0 {
				g.particles = append(g.particles[:i], g.particles[i+1:]...)
			}
		}

		// Update spawn point
		g.updateSpawn()

		// Camera
		g.updateCamera()
	}

	return nil
}

func (g *Game) playBeat() {
	// Find the beat event in the song
	beat := g.rhythm.currentBeat
	for _, event := range g.level.song {
		if event.beat == beat%16 {
			if event.char == 'K' {
				if event.freq > 0 {
					samples := generateBassNote(event.freq, 0.12)
					wav := encodeWAV(samples)
					reader := bytes.NewReader(wav)
					player, err := g.sound.ctx.NewPlayer(reader)
					if err == nil {
						player.Rewind()
						player.Play()
					}
				} else {
					g.sound.Play("kick")
				}
			} else if event.char == 'S' {
				g.sound.Play("snare")
			} else if event.char == 'H' {
				g.sound.Play("hihat")
			}
		}
	}
}

func (g *Game) addScore(rating string) {
	points := 0
	switch rating {
	case "PERFECT":
		points = 300
		g.combo++
	case "GREAT":
		points = 200
		g.combo++
	case "OKAY":
		points = 100
		g.combo = 0
	case "MISS":
		g.combo = 0
	}
	if g.combo > g.bestCombo {
		g.bestCombo = g.combo
	}
	g.score += points * g.combo / 2
	if rating != "OKAY" && rating != "MISS" {
		g.lastRating = rating
		g.ratingTimer = 30
	} else if rating != "MISS" {
		g.lastRating = rating
		g.ratingTimer = 20
	}
}


func (g *Game) updateSpawn() {
	px := int(g.player.x / float64(TILE_SIZE))
	for i := g.spawnIdx; i < len(levelMap); i++ {
		row := levelMap[i]
		if px > int(len(row))-10 {
			g.spawnIdx = i
		}
	}
	if g.spawnIdx >= len(levelMap) {
		g.spawnIdx = len(levelMap) - 1
	}
	g.player.respawnX = 50
	g.player.respawnY = float64(g.spawnIdx*TILE_SIZE - 4*TILE_SIZE)
	if g.player.respawnY < 0 {
		g.player.respawnY = 0
	}
}


func (g *Game) updateCamera() {
	targetCamX := g.player.x - SCREEN_WIDTH/(2*SCALE)
	targetCamY := g.player.y - SCREEN_HEIGHT/(2*SCALE)
	g.cameraX += (targetCamX - g.cameraX) * 0.1
	g.cameraY += (targetCamY - g.cameraY) * 0.08

	levelWidth := float64(len(levelMap[0]) * TILE_SIZE)
	levelHeight := float64(len(levelMap) * TILE_SIZE)
	if g.cameraX < 0 {
		g.cameraX = 0
	}
	if g.cameraY < 0 {
		g.cameraY = 0
	}
	if g.cameraX > levelWidth-float64(SCREEN_WIDTH)/SCALE {
		g.cameraX = levelWidth - float64(SCREEN_WIDTH)/SCALE
	}
	if g.cameraY > levelHeight-float64(SCREEN_HEIGHT)/SCALE {
		g.cameraY = levelHeight - float64(SCREEN_HEIGHT)/SCALE
	}
}

var gameWidth = float64(SCREEN_WIDTH)
var gameHeight = float64(SCREEN_HEIGHT)

func (g *Game) nextLevel() {
	g.levelIdx = (g.levelIdx + 1) % len(rhythmLevels)
	g.level = &rhythmLevels[g.levelIdx]
	g.rhythm = NewRhythmSystem(g.level.bpm)
	g.tiles = nil
	g.parseLevel()
	gameTiles = g.tiles
	g.player = NewPlayer(g.level.startX, g.level.startY)
	g.player.respawnX = g.level.startX
	g.player.respawnY = g.level.startY
	g.goalReached = false
	g.score = 0
	g.combo = 0
	g.bestCombo = 0
	g.deaths = 0
	g.gameStarted = false
}

func (g *Game) trackKeys() {
	jumpKeys := []ebiten.Key{ebiten.KeySpace, ebiten.KeyW, ebiten.KeyArrowUp}
	for _, k := range jumpKeys {
		pressed := ebiten.IsKeyPressed(k)
		if pressed && !g.keys[k] {
			g.keys[k] = true
		} else if !pressed {
			g.keys[k] = false
		}
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{20, 18, 28, 255})
	g.drawBackground(screen)

	// Tiles
	for _, t := range g.tiles {
		sx := (t.x - g.cameraX) * SCALE
		sy := (t.y - g.cameraY) * SCALE
		if sx+float64(TILE_SIZE)*SCALE < 0 || sx > SCREEN_WIDTH || sy+float64(TILE_SIZE)*SCALE < 0 || sy > SCREEN_HEIGHT {
			continue
		}

		// Skip beat-sync tiles that aren't active
		if t.beatSync && !t.active {
			continue
		}

		idx := g.tileIndex(t.tileType)
		if t.tileType == TILE_MUD {
			// Use moss tile but draw with mud color
			idx = 2
		}
		subRect := image.Rect(idx*TILE_SIZE, 0, idx*TILE_SIZE+TILE_SIZE, TILE_SIZE)
		if subRect.Dx() <= 0 || subRect.Dy() <= 0 {
			continue
		}
		subImg := g.tilesetImg.SubImage(subRect).(*ebiten.Image)
		opts := &ebiten.DrawImageOptions{}
		opts.GeoM.Scale(SCALE, SCALE)
		opts.GeoM.Translate(sx/SCALE, sy/SCALE)
		// Tint mud tiles
		if t.tileType == TILE_MUD {
			opts.ColorM.Scale(0.7, 0.6, 0.9, 1.0)
		}
		screen.DrawImage(subImg, opts)
	}

	// Particles
	for _, p := range g.particles {
		alpha := uint8(255 * p.life / p.maxLife)
		col := color.RGBA{
			uint8(float64(p.col.R) * float64(alpha) / 255),
			uint8(float64(p.col.G) * float64(alpha) / 255),
			uint8(float64(p.col.B) * float64(alpha) / 255),
			alpha,
		}
		ebitenutil.DrawRect(screen, p.x*SCALE, p.y*SCALE, p.size*SCALE, p.size*SCALE, col)
	}

	// Player
	g.drawPlayer(screen)

	// HUD
	g.drawHUD(screen)

	// Draw start screen
	if !g.gameStarted && !g.goalReached {
		g.drawStartScreen(screen)
	}

	// Draw goal reached screen
	if g.goalReached {
		g.drawGoalScreen(screen)
	}
}

func (g *Game) drawBackground(screen *ebiten.Image) {
	bg1Color := color.RGBA{25, 20, 35, 255}
	screen.Fill(bg1Color)

	// Parallax layers
	g.drawParallaxShapes(screen, 0.2, color.RGBA{35, 28, 50, 255}, 40, 3, 30)
	g.drawParallaxShapes(screen, 0.5, color.RGBA{30, 24, 42, 255}, 20, 5, 20)
}

func (g *Game) drawParallaxShapes(screen *ebiten.Image, parallax float64, col color.Color, spacing, size, count int) {
	offsetX := g.cameraX * parallax
	offsetY := g.cameraY * parallax * 0.3
	for i := 0; i < count; i++ {
		x := float64(i*spacing*TILE_SIZE) - offsetX
		y := float64((i*7+3)*TILE_SIZE) - offsetY
		sx := x * SCALE
		sy := y * SCALE
		if sx < -float64(size)*SCALE*2 || sx > SCREEN_WIDTH+float64(size)*SCALE*2 {
			continue
		}
		ebitenutil.DrawRect(screen, sx, sy, float64(size)*SCALE, float64(size)*SCALE, col)
		ebitenutil.DrawRect(screen, sx+2, sy+2, float64(size-2)*SCALE, float64(size-2)*SCALE, color.RGBA{40, 32, 55, 255})
	}
}

func (g *Game) drawPlayer(screen *ebiten.Image) {
	p := g.player

	// Death animation
	if p.dead && p.deathTimer > 0 {
		px := (p.x - g.cameraX) * SCALE
		py := (p.y - g.cameraY) * SCALE
		pw := p.width * SCALE
		ph := p.height * SCALE
		flash := (g.animFrame / 3) % 2 == 0
		if flash {
			alpha := uint8(255 * p.deathTimer / 30)
			ebitenutil.DrawRect(screen, px, py, pw, ph, color.RGBA{255, 255, 255, alpha})
		}
		return
	}

	if p.dead {
		return
	}

	px := (p.x - g.cameraX) * SCALE
	py := (p.y - g.cameraY) * SCALE
	pw := p.width * SCALE
	ph := p.height * SCALE

	// Determine animation frame
	var animFrame int
	switch {
	case p.isDashing:
		animFrame = 10 + g.animFrame/6%2
	case !p.onGround && p.vy < 0:
		animFrame = 8 + g.animFrame/10%2
	case !p.onGround:
		animFrame = 9
	case p.vx != 0:
		animFrame = 4 + g.animFrame/8%4
	default:
		animFrame = g.animFrame/10%4
	}

	// Draw sprite
	if g.spritesImg != nil {
		sx := float64(animFrame * SPRITE_W)
		subRect := image.Rect(int(sx), 0, int(sx)+SPRITE_W, SPRITE_H)
		subImg := g.spritesImg.SubImage(subRect).(*ebiten.Image)
		opts := &ebiten.DrawImageOptions{}
		opts.GeoM.Scale(SCALE/2, SCALE/2)
		if !p.facingRight {
			opts.GeoM.Scale(-1, 1)
			opts.GeoM.Translate(SCREEN_WIDTH/SCALE, 0)
		}
		opts.GeoM.Translate(px/SCALE, py/SCALE)
		screen.DrawImage(subImg, opts)
	} else {
		// Fallback: draw a rectangle
		ebitenutil.DrawRect(screen, px, py, pw, ph, color.RGBA{170, 112, 220, 255})
	}

	// Dash trail
	if p.isDashing {
		for i := 1; i <= 2; i++ {
			trailX := px - p.dashDirX*float64(i)*12
			trailY := py - p.dashDirY*float64(i)*12
			alpha := 100 - uint8(i*40)
			if alpha > 255 {
				alpha = 255
			}
			ebitenutil.DrawRect(screen, trailX, trailY, pw, ph, color.RGBA{180, 140, 200, alpha})
		}
	}

	// Jump indicator
	for i := 0; i < MAX_JUMPS; i++ {
		jx := px + pw*0.1 + float64(i)*pw*0.25
		jy := py - ph*0.15
		if i < p.jumpsLeft || p.onGround {
			ebitenutil.DrawRect(screen, jx, jy, pw*0.15, ph*0.08, color.RGBA{150, 255, 180, 200})
		} else {
			ebitenutil.DrawRect(screen, jx, jy, pw*0.15, ph*0.08, color.RGBA{80, 80, 80, 150})
		}
	}
}

func (g *Game) drawHUD(screen *ebiten.Image) {
	// Top left
	ebitenutil.DebugPrint(screen, "RITMO DEL BARRO")
	ebitenutil.DebugPrintAt(screen, "A/D or Arrows: Move | W/Space: Jump | Shift: Dash | S: Fast Fall", 0, 12)

	// BPM and beat info
	bpmStr := formatInt(int(g.level.bpm))
	ebitenutil.DebugPrintAt(screen, "BPM: "+bpmStr, 0, 24)

	// Beat indicator
	beatDist := abs(g.rhythm.BeatDistance())
	beatPct := float64(0)
	if g.rhythm.beatInterval > 0 {
		beatPct = float64(beatDist) / float64(g.rhythm.beatInterval/2) * 100
	}
	ebitenutil.DebugPrintAt(screen, "Beat: "+formatInt(int(beatPct))+"%", 0, 36)

	// Score and combo
	ebitenutil.DebugPrintAt(screen, "Score: "+formatInt(g.score), SCREEN_WIDTH-90, 12)
	ebitenutil.DebugPrintAt(screen, "Combo: "+formatInt(g.combo), SCREEN_WIDTH-90, 24)
	ebitenutil.DebugPrintAt(screen, "Best: "+formatInt(g.bestCombo), SCREEN_WIDTH-90, 36)

	// Rating
	if g.ratingTimer > 0 {
		g.ratingTimer--
		ebitenutil.DebugPrintAt(screen, g.lastRating, SCREEN_WIDTH/2-50, 40)
	}

	// Deaths
	ebitenutil.DebugPrintAt(screen, "Deaths: "+formatInt(g.deaths), SCREEN_WIDTH-80, 50)

	// Level name
	ebitenutil.DebugPrintAt(screen, "Lvl: "+g.level.name+" ("+formatInt(g.levelIdx+1)+"/"+formatInt(len(rhythmLevels))+")", 0, SCREEN_HEIGHT-12)
}

func (g *Game) drawStartScreen(screen *ebiten.Image) {
	// Darken overlay
	ebitenutil.DrawRect(screen, 0, 0, float64(SCREEN_WIDTH), float64(SCREEN_HEIGHT), color.RGBA{0, 0, 0, 120})

	// Title
	ebitenutil.DebugPrintAt(screen, "RITMO DEL BARRO", SCREEN_WIDTH/2-80, SCREEN_HEIGHT/2-30)
	ebitenutil.DebugPrintAt(screen, "Nivel: "+g.level.name, SCREEN_WIDTH/2-60, SCREEN_HEIGHT/2-15)
	ebitenutil.DebugPrintAt(screen, "BPM: "+formatInt(int(g.level.bpm)), SCREEN_WIDTH/2-40, SCREEN_HEIGHT/2)
	ebitenutil.DebugPrintAt(screen, "Author: "+g.level.author, SCREEN_WIDTH/2-60, SCREEN_HEIGHT/2+15)
	ebitenutil.DebugPrintAt(screen, "Presiona SPACE para empezar", SCREEN_WIDTH/2-100, SCREEN_HEIGHT/2+40)
}

func (g *Game) drawGoalScreen(screen *ebiten.Image) {
	ebitenutil.DrawRect(screen, 0, 0, float64(SCREEN_WIDTH), float64(SCREEN_HEIGHT), color.RGBA{0, 0, 0, 180})
	ebitenutil.DebugPrintAt(screen, "¡NIVEL COMPLETADO!", SCREEN_WIDTH/2-80, SCREEN_HEIGHT/2-20)
	ebitenutil.DebugPrintAt(screen, "Puntuación: "+formatInt(g.score), SCREEN_WIDTH/2-70, SCREEN_HEIGHT/2)
	ebitenutil.DebugPrintAt(screen, "Mejor Combo: "+formatInt(g.bestCombo), SCREEN_WIDTH/2-70, SCREEN_HEIGHT/2+15)
	ebitenutil.DebugPrintAt(screen, "Muertes: "+formatInt(g.deaths), SCREEN_WIDTH/2-50, SCREEN_HEIGHT/2+30)
	ebitenutil.DebugPrintAt(screen, "Presiona N para el siguiente nivel", SCREEN_WIDTH/2-110, SCREEN_HEIGHT/2+60)
}

func formatInt(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	if n < 100 {
		return string(rune('0'+n/10)) + string(rune('0'+n%10))
	}
	if n < 1000 {
		return string(rune('0'+n/100)) + string(rune('0'+n/10%10)) + string(rune('0'+n%10))
	}
	return string(rune(n/1000)) + string(rune(n/100%10)) + string(rune(n/10%10)) + string(rune(n%10))
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return SCREEN_WIDTH, SCREEN_HEIGHT
}

func main() {
	ebiten.SetWindowSize(SCREEN_WIDTH*SCALE, SCREEN_HEIGHT*SCALE)
	ebiten.SetWindowTitle("Ritmo del Barro - Rhythm Platformer")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetTPS(GAME_FPS)

	game := NewGame()

	if err := ebiten.RunGame(game); err != nil {
		panic(err)
	}
}

// ─── FALLBACK ASSETS (generated if files missing) ─────────────────────────

func generateFallbackTileset() *ebiten.Image {
	img := ebiten.NewImage(16*13, 16)
	for i := 0; i < 13; i++ {
		col := color.RGBA{uint8(100 + i*10), uint8(80 + i*5), uint8(120 + i*3), 255}
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				img.Set(i*16+x, y, col)
			}
		}
	}
	return img
}

func generateFallbackSprites() *ebiten.Image {
	img := ebiten.NewImage(16*8, 24)
	for i := 0; i < 8; i++ {
		col := color.RGBA{170, 112, 220, 255}  // already uint8-compatible
		for y := 0; y < 24; y++ {
			for x := 0; x < 16; x++ {
				img.Set(i*16+x, y, col)
			}
		}
	}
	return img
}
