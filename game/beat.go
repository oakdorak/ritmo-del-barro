package game

import (
	"math"
	"time"
)

// Beat modela la cuadrícula rítmica del "barro". Todo lo sincronizado al beat
// entrena la Explícita (timing secuencial) — gap mapeado en VECTOR_MAPPING_V1.
type Beat struct {
	bpm     float64
	beatDur time.Duration
	startAt time.Time
	phase   float64 // 0..1 dentro del beat actual (0 = en el beat)
	beatIdx int
	just    bool // true sólo el frame en que arranca un nuevo beat
}

func NewBeat(bpm float64) *Beat {
	if bpm <= 0 {
		bpm = 120
	}
	return &Beat{bpm: bpm, beatDur: time.Duration(60.0/bpm*float64(time.Second))}
}

func (b *Beat) Update(now time.Time) {
	if b.startAt.IsZero() {
		b.startAt = now
	}
	elapsed := now.Sub(b.startAt)
	frac := float64(elapsed) / float64(b.beatDur)
	prev := b.beatIdx
	b.phase = frac - math.Floor(frac)
	b.beatIdx = int(math.Floor(frac))
	b.just = b.beatIdx != prev // cruzó la línea de beat
}

// JustTicked indica el frame exacto en que arranca un beat (para el beep).
func (b *Beat) JustTicked() bool { return b.just }

// InWindow indica si la fase actual está dentro de la ventana de timing
// (±BeatWindow alrededor del beat).
func (b *Beat) InWindow() bool {
	d := math.Min(b.phase, 1-b.phase) // distancia al beat más cercano
	return d*float64(b.beatDur) <= float64(BeatWindow)
}

const BeatWindow = 50 * time.Millisecond // ±50ms: retador pero alcanzable
