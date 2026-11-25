// sound.go — short UI sound effects, synthesized at start-up (no audio files, nothing to license).
package main

import (
	"encoding/binary"
	"math"
	"unsafe"
)

var pPlaySoundW = winmm.NewProc("PlaySoundW")

const sampleRate = 44100

type tone struct {
	freq  float64 // Hz (0 = silence)
	ms    float64 // length
	decay float64 // higher = shorter, snappier
}

// synth builds a 16-bit mono WAV from a list of tones with a soft attack and
// exponential decay, plus a quiet octave overtone for a "glassy" feel.
func synth(vol float64, tones ...tone) []byte {
	var pcm []int16
	for _, t := range tones {
		n := int(t.ms * sampleRate / 1000)
		for i := 0; i < n; i++ {
			x := float64(i) / sampleRate
			attack := math.Min(1, float64(i)/(0.004*sampleRate))
			env := attack * math.Exp(-t.decay*x)
			v := 0.0
			if t.freq > 0 {
				v = math.Sin(2*math.Pi*t.freq*x) + 0.25*math.Sin(2*math.Pi*t.freq*2*x)
			}
			pcm = append(pcm, int16(v/1.25*env*vol*32767))
		}
	}
	// fade the very end to avoid a click
	for i := 0; i < 200 && i < len(pcm); i++ {
		pcm[len(pcm)-1-i] = int16(float64(pcm[len(pcm)-1-i]) * float64(i) / 200)
	}
	data := make([]byte, 44+len(pcm)*2)
	copy(data[0:], "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(36+len(pcm)*2))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1) // PCM
	binary.LittleEndian.PutUint16(data[22:], 1) // mono
	binary.LittleEndian.PutUint32(data[24:], sampleRate)
	binary.LittleEndian.PutUint32(data[28:], sampleRate*2)
	binary.LittleEndian.PutUint16(data[32:], 2)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], uint32(len(pcm)*2))
	for i, s := range pcm {
		binary.LittleEndian.PutUint16(data[44+i*2:], uint16(s))
	}
	return data
}

var (
	sndTick   = synth(0.22, tone{1850, 40, 90})
	sndToggle = synth(0.22, tone{1320, 30, 70}, tone{1760, 45, 80})
	sndRec    = synth(0.26, tone{660, 70, 25}, tone{990, 120, 22})
	sndFinish = synth(0.26, tone{990, 70, 25}, tone{660, 130, 20})
	sndPlay   = synth(0.24, tone{523, 55, 30}, tone{659, 55, 30}, tone{784, 130, 18})
	sndStop   = synth(0.26, tone{392, 70, 28}, tone{294, 140, 20})

	sndCount = synth(0.22, tone{880, 70, 35})
	sndGo    = synth(0.24, tone{1320, 140, 22})

	soundOn     = true
	countdownOn = true
)

func play(wav []byte) {
	if !soundOn || len(wav) == 0 {
		return
	}
	// SND_MEMORY | SND_ASYNC | SND_NODEFAULT; the slices live for the whole program
	pPlaySoundW.Call(uintptr(unsafe.Pointer(&wav[0])), 0, 0x4|0x1|0x2)
}
