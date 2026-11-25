// sound.go — short UI sound effects, synthesized at start-up (no audio files, nothing to license).
//
// Playback keeps ONE audio output open for the whole session (waveOut) and plays
// pre-prepared buffers, so a sound starts immediately instead of waiting for
// Windows to open the speakers each time.
package main

import (
	"encoding/binary"
	"math"
	"unsafe"
)

var (
	pWaveOutOpen    = winmm.NewProc("waveOutOpen")
	pWaveOutPrepare = winmm.NewProc("waveOutPrepareHeader")
	pWaveOutWrite   = winmm.NewProc("waveOutWrite")
	pWaveOutReset   = winmm.NewProc("waveOutReset")
	pWaveOutClose   = winmm.NewProc("waveOutClose")
	pWaveOutUnprep  = winmm.NewProc("waveOutUnprepareHeader")
)

const sampleRate = 44100

type tone struct {
	freq  float64 // Hz (0 = silence)
	ms    float64 // length
	decay float64 // higher = shorter, snappier
}

// sound is a ready-to-play 16-bit mono PCM buffer with its waveOut header.
type sound struct {
	pcm []byte
	hdr *waveHdr
}

type waveHdr struct {
	Data          *byte
	BufferLength  uint32
	BytesRecorded uint32
	User          uintptr
	Flags         uint32
	Loops         uint32
	Next          uintptr
	Reserved      uintptr
}

// synth builds PCM from tones with a quick attack and exponential decay, plus a quiet
// octave overtone for a "glassy" feel.
func synth(vol float64, tones ...tone) *sound {
	var pcm []int16
	for _, t := range tones {
		n := int(t.ms * sampleRate / 1000)
		for i := 0; i < n; i++ {
			x := float64(i) / sampleRate
			attack := math.Min(1, float64(i)/(0.002*sampleRate))
			env := attack * math.Exp(-t.decay*x)
			v := 0.0
			if t.freq > 0 {
				v = math.Sin(2*math.Pi*t.freq*x) + 0.25*math.Sin(2*math.Pi*t.freq*2*x)
			}
			pcm = append(pcm, int16(v/1.25*env*vol*32767))
		}
	}
	for i := 0; i < 200 && i < len(pcm); i++ { // fade the very end to avoid a click
		pcm[len(pcm)-1-i] = int16(float64(pcm[len(pcm)-1-i]) * float64(i) / 200)
	}
	b := make([]byte, len(pcm)*2)
	for i, s := range pcm {
		binary.LittleEndian.PutUint16(b[i*2:], uint16(s))
	}
	return &sound{pcm: b}
}

var (
	sndTick   = synth(0.22, tone{1850, 40, 90})
	sndToggle = synth(0.22, tone{1320, 30, 70}, tone{1760, 45, 80})
	sndRec    = synth(0.26, tone{660, 70, 25}, tone{990, 120, 22})
	sndFinish = synth(0.26, tone{990, 70, 25}, tone{660, 130, 20})
	sndPlay   = synth(0.24, tone{523, 55, 30}, tone{659, 55, 30}, tone{784, 130, 18})
	sndStop   = synth(0.26, tone{392, 70, 28}, tone{294, 140, 20})
	sndCount  = synth(0.22, tone{880, 70, 35})
	sndGo     = synth(0.24, tone{1320, 140, 22})

	allSounds = []*sound{sndTick, sndToggle, sndRec, sndFinish, sndPlay, sndStop, sndCount, sndGo}

	soundOn     = true
	countdownOn = true

	waveOut uintptr // open output device, 0 if none
)

// initAudio opens the default output once and prepares every sound buffer.
func initAudio() {
	wfx := struct {
		FormatTag      uint16
		Channels       uint16
		SamplesPerSec  uint32
		AvgBytesPerSec uint32
		BlockAlign     uint16
		BitsPerSample  uint16
		CbSize         uint16
	}{1, 1, sampleRate, sampleRate * 2, 2, 16, 0}
	var h uintptr
	if r, _, _ := pWaveOutOpen.Call(uintptr(unsafe.Pointer(&h)), 0xFFFFFFFF, uintptr(unsafe.Pointer(&wfx)), 0, 0, 0); r != 0 {
		return // no audio device: stay silent
	}
	waveOut = h
	for _, s := range allSounds {
		s.hdr = &waveHdr{Data: &s.pcm[0], BufferLength: uint32(len(s.pcm))}
		pWaveOutPrepare.Call(waveOut, uintptr(unsafe.Pointer(s.hdr)), unsafe.Sizeof(*s.hdr))
	}
}

func closeAudio() {
	if waveOut == 0 {
		return
	}
	pWaveOutReset.Call(waveOut)
	for _, s := range allSounds {
		if s.hdr != nil {
			pWaveOutUnprep.Call(waveOut, uintptr(unsafe.Pointer(s.hdr)), unsafe.Sizeof(*s.hdr))
		}
	}
	pWaveOutClose.Call(waveOut)
	waveOut = 0
}

// play starts a sound right away, cutting off any sound still playing.
func play(s *sound) {
	if !soundOn || waveOut == 0 || s == nil || s.hdr == nil {
		return
	}
	pWaveOutReset.Call(waveOut) // returns any playing buffer so it can be reused
	pWaveOutWrite.Call(waveOut, uintptr(unsafe.Pointer(s.hdr)), unsafe.Sizeof(*s.hdr))
}
