package game

import (
	"encoding/binary"
	"math"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

// Beeper genera un beep sinusoidal procedural (PCM raw 16-bit) que suena en
// cada tick de beat. NewPlayerFromBytes en v2.10 consume PCM int16 LE directo
// (no decodifica WAV), así que sinePCM devuelve samples sin header.

const sampleRate = 44100

var (
	audioCtx     *audio.Context
	audioCtxOnce sync.Once
)

type Beeper struct {
	player *audio.Player
}

func newAudioContext() *audio.Context {
	audioCtxOnce.Do(func() {
		audioCtx = audio.NewContext(sampleRate)
	})
	return audioCtx
}

func NewBeeper() *Beeper {
	ctx := newAudioContext()
	pcm := sinePCM(440, 70*time.Millisecond, 0.15)
	return &Beeper{player: ctx.NewPlayerFromBytes(pcm)}
}

// Tick reproduce el beep (rewind + play). Seguro de llamar cada beat.
func (b *Beeper) Tick() {
	_ = b.player.Rewind() // WAV-equivalent seek; ignore error (player seekable)
	b.player.Play()
}

// sinePCM produce PCM mono 16-bit little-endian (raw, sin header).
func sinePCM(freq float64, dur time.Duration, amp float64) []byte {
	n := int(float64(sampleRate) * dur.Seconds())
	if n < 1 {
		n = 1
	}
	buf := make([]int16, n)
	for i := 0; i < n; i++ {
		t := float64(i) / sampleRate
		// arrastre suave (attack rápido / release lento) evita clicks
		env := math.Sin(math.Pi * math.Min(1.0, t/dur.Seconds()))
		buf[i] = int16(amp * env * 32767 * math.Sin(2*math.Pi*freq*t))
	}
	out := make([]byte, 2*n)
	for i, s := range buf {
		binary.LittleEndian.PutUint16(out[i*2:], uint16(s))
	}
	return out
}
