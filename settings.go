// settings.go — hotkeys (customisable), sounds, saved settings, and the Settings window.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// ───────────────────────────── Hotkeys ─────────────────────────────

type hotkey struct{ mods, vk uint32 } // mods: MOD_ALT 1, MOD_CONTROL 2, MOD_SHIFT 4

var (
	defaultHK = [3]hotkey{{0, vkF6}, {0, vkF7}, {0, vkF8}} // record, play, stop
	hk        = defaultHK
	hkOK      = [3]bool{true, true, true}
)

func keyName(k hotkey) string {
	var b strings.Builder
	if k.mods&2 != 0 {
		b.WriteString("Ctrl+")
	}
	if k.mods&1 != 0 {
		b.WriteString("Alt+")
	}
	if k.mods&4 != 0 {
		b.WriteString("Shift+")
	}
	vk := k.vk
	switch {
	case vk >= 0x70 && vk <= 0x87:
		b.WriteString(fmt.Sprintf("F%d", vk-0x6F))
	case vk >= 0x30 && vk <= 0x39, vk >= 0x41 && vk <= 0x5A:
		b.WriteByte(byte(vk))
	case vk >= 0x60 && vk <= 0x69:
		b.WriteString(fmt.Sprintf("Num %d", vk-0x60))
	default:
		sc, _, _ := pMapVirtualKey.Call(uintptr(vk), 0)
		lp := sc << 16
		switch vk {
		case 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x2D, 0x2E, 0x6F, 0x90:
			lp |= 1 << 24 // extended key
		}
		buf := make([]uint16, 64)
		n, _, _ := pGetKeyNameText.Call(lp, uintptr(unsafe.Pointer(&buf[0])), 64)
		if n > 0 {
			b.WriteString(syscall.UTF16ToString(buf[:n]))
		} else {
			b.WriteString(fmt.Sprintf("Key %d", vk))
		}
	}
	return b.String()
}

func hkName(i int) string { return keyName(hk[i]) }

func registerHotkeys() {
	for i := range hk {
		r, _, _ := pRegisterHotKey.Call(hwndMain, uintptr(i+1), uintptr(hk[i].mods|0x4000), uintptr(hk[i].vk)) // MOD_NOREPEAT
		hkOK[i] = r != 0
	}
	hotkeysOK = hkOK[0] && hkOK[1] && hkOK[2]
}

func unregisterHotkeys() {
	for i := 1; i <= 3; i++ {
		pUnregisterHotKey.Call(hwndMain, uintptr(i))
	}
}

// ───────────────────────────── Saved settings ─────────────────────────────

func settingsPath() string { return filepath.Join(dataDir(), "settings.ini") }

func loadSettings() {
	for _, n := range []string{"settings.ini", "last_recording.pacr"} { // migrate from v1.0 location
		newP := filepath.Join(dataDir(), n)
		if _, err := os.Stat(newP); err != nil {
			if b, err := os.ReadFile(oldPath(n)); err == nil {
				_ = os.WriteFile(newP, b, 0644)
			}
		}
	}
	b, err := os.ReadFile(settingsPath())
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch k {
		case "dark":
			darkMode = v != "0"
		case "sound":
			soundOn = v != "0"
		case "countdown":
			countdownOn = v != "0"
		case "hk1", "hk2", "hk3":
			m, key, ok := strings.Cut(v, ",")
			mi, e1 := strconv.Atoi(m)
			ki, e2 := strconv.Atoi(key)
			if ok && e1 == nil && e2 == nil && ki > 0 && ki < 256 {
				hk[int(k[2]-'1')] = hotkey{uint32(mi) & 7, uint32(ki)}
			}
		}
	}
}

func saveSettings() {
	b01 := func(v bool) string {
		if v {
			return "1"
		}
		return "0"
	}
	s := "dark=" + b01(darkMode) + "\nsound=" + b01(soundOn) + "\ncountdown=" + b01(countdownOn) + "\n"
	for i, k := range hk {
		s += fmt.Sprintf("hk%d=%d,%d\n", i+1, k.mods, k.vk)
	}
	_ = os.WriteFile(settingsPath(), []byte(s), 0644)
}

// ───────────────────────────── Settings window ─────────────────────────────

const (
	setW = 360
	setH = 394
)

var (
	hwndSettings             uintptr
	appIconBig, appIconSmall uintptr
	settingsClassOK          bool
	capturing                uintptr // key button currently waiting for a key
	keyBtns                  [3]uintptr
	settingsBtns             []uintptr
	hSoundBtn, hCountBtn     uintptr
	tmpCount                 bool
	tmpHK                    [3]hotkey
	tmpSound                 bool
	settingsHint             = ""
	settingsHintWarn         = false

	sySound  = box{16, 234, 328, 44}
	syCount  = box{16, 286, 328, 44}
	syReset  = box{16, 342, 100, 36}
	syCancel = box{128, 342, 100, 36}
	sySave   = box{240, 342, 104, 36}
	syCard   = box{16, 62, 328, 160}
)

func syKey(i int) box { return box{170, 92 + i*36, 160, 30} }

const (
	idKey0 = 301 + iota
	idKey1
	idKey2
	idSound
	idCount
	idReset
	idCancel
	idSaveSet
)

func settingsScene(p *painter, clip rect) {
	W, H := sc(setW), sc(setH)
	paintBackdrop(p, clip, W, H)
	headerBand(p, clip, W, rect{0, 0, W, sc(52)})
	p.icon("gear", rect{sc(16), sc(16), sc(36), sc(36)}, gold1)
	if c := syCard.r(); hit(c, clip) {
		p.card(c)
	}
}

func paintSettings(hdc uintptr) {
	var cr rect
	pGetClientRect.Call(hwndSettings, uintptr(unsafe.Pointer(&cr)))
	mem, _, _ := pCreateCompatibleDC.Call(hdc)
	bmp, _, _ := pCreateCompatibleBmp.Call(hdc, uintptr(cr.R), uintptr(cr.B))
	old, _, _ := pSelectObject.Call(mem, bmp)
	p := newPainter(mem)
	settingsScene(p, cr)
	pl := pal()
	p.text("Settings", rect{sc(46), 0, sc(setW), sc(52)}, fLabel, gold1, 0x20|0x4)
	c := syCard.r()
	p.texts = append(p.texts, textOp{s: "HOTKEYS", r: rect{c.L + sc(14), c.T + sc(8), c.R, c.T + sc(24)}, font: fCap, col: pl.muted, flags: 0x20 | 0x4, extra: sc(1)})
	for i, name := range []string{"Record / Finish", "Play", "Stop"} {
		k := syKey(i).r()
		p.text(name, rect{c.L + sc(14), k.T, k.L - sc(8), k.B}, fTile, pl.text, 0x20|0x4)
	}
	hint := settingsHint
	hc := pl.muted
	if hint == "" {
		hint = "Click a box, then press a key. Esc cancels."
	} else if settingsHintWarn {
		hc = A(255, 245, 166, 35)
	}
	p.text(hint, rect{c.L + sc(14), c.B - sc(26), c.R - sc(10), c.B - sc(8)}, fInfo, hc, 0x20|0x4|0x8000)
	p.finish(mem)
	pBitBlt.Call(hdc, 0, 0, uintptr(cr.R), uintptr(cr.B), mem, 0, 0, 0xCC0020)
	pSelectObject.Call(mem, old)
	pDeleteObject.Call(bmp)
	pDeleteDC.Call(mem)
}

func setHint(s string, warn bool) {
	settingsHint, settingsHintWarn = s, warn
	r := syCard.r()
	pInvalidateRect.Call(hwndSettings, uintptr(unsafe.Pointer(&r)), 0)
}

func refreshSettingsButtons() {
	for i, b := range keyBtns {
		setLabel(b, keyName(tmpHK[i]), "")
	}
	setCheck(hSoundBtn, tmpSound)
	if tmpSound {
		setLabel(hSoundBtn, "Click sounds", "vol")
	} else {
		setLabel(hSoundBtn, "Click sounds", "mute")
	}
	setCheck(hCountBtn, tmpCount)
}

func isModifier(vk uint32) bool {
	switch vk {
	case 0x10, 0x11, 0x12, 0x5B, 0x5C, 0xA0, 0xA1, 0xA2, 0xA3, 0xA4, 0xA5, 0x14, 0x90, 0x91:
		return true
	}
	return false
}

func captureKey(btn uintptr, vk uint32) {
	if isModifier(vk) {
		return // wait for the real key
	}
	down := func(k uintptr) bool { r, _, _ := pGetKeyState.Call(k); return int16(r) < 0 }
	var mods uint32
	if down(0x11) {
		mods |= 2
	}
	if down(0x12) {
		mods |= 1
	}
	if down(0x10) {
		mods |= 4
	}
	if vk == 0x1B && mods == 0 { // Esc cancels
		capturing = 0
		pInvalidateRect.Call(btn, 0, 0)
		setHint("", false)
		return
	}
	isF := vk >= 0x70 && vk <= 0x87
	if !isF && mods&3 == 0 {
		// a plain letter, Space, Enter… would stop you typing anywhere else
		setHint("Use an F-key, or add Ctrl or Alt (e.g. Ctrl+Shift+R)", true)
		play(sndStop)
		return
	}
	for i, b := range keyBtns {
		if b == btn {
			tmpHK[i] = hotkey{mods, vk}
		}
	}
	capturing = 0
	play(sndTick)
	setHint("", false)
	refreshSettingsButtons()
}

func settingsProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case 0x002B: // WM_DRAWITEM
		drawButton((*drawItemStruct)(unsafe.Pointer(lParam)))
		return 1
	case 0x0014:
		return 1
	case wmPaint:
		var ps paintStruct
		hdc, _, _ := pBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		paintSettings(hdc)
		pEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case wmCommand:
		code := hiword(wParam)
		if code != 0 && code != 5 {
			break
		}
		switch id := int(loword(wParam)); id {
		case idKey0, idKey1, idKey2:
			b := keyBtns[id-idKey0]
			prev := capturing
			capturing = b
			if prev != 0 {
				pInvalidateRect.Call(prev, 0, 0)
			}
			pSetFocus.Call(b)
			pInvalidateRect.Call(b, 0, 0)
			setHint("Now press the key you want. Esc cancels.", false)
		case idSound:
			tmpSound = !tmpSound
			prev := soundOn
			soundOn = true
			if tmpSound {
				play(sndToggle)
			}
			soundOn = prev
			refreshSettingsButtons()
		case idCount:
			tmpCount = !tmpCount
			if tmpSound {
				prev := soundOn
				soundOn = true
				play(sndToggle)
				soundOn = prev
			}
			refreshSettingsButtons()
		case idReset:
			play(sndTick)
			tmpHK = defaultHK
			setHint("Back to F6 / F7 / F8. Press Save to keep it.", false)
			refreshSettingsButtons()
		case idCancel:
			play(sndTick)
			closeSettings(false)
		case idSaveSet:
			saveSettingsWindow()
		}
		return 0
	case wmClose:
		closeSettings(false)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return r
}

func saveSettingsWindow() {
	capturing = 0
	for i := 0; i < 3; i++ {
		for j := i + 1; j < 3; j++ {
			if tmpHK[i] == tmpHK[j] {
				setHint(keyName(tmpHK[i])+" is set twice. Give each action its own key", true)
				play(sndStop)
				return
			}
		}
	}
	// try the new keys before keeping them
	for i := range tmpHK {
		r, _, _ := pRegisterHotKey.Call(hwndMain, uintptr(i+1), uintptr(tmpHK[i].mods|0x4000), uintptr(tmpHK[i].vk))
		if r == 0 {
			for j := 0; j < i; j++ {
				pUnregisterHotKey.Call(hwndMain, uintptr(j+1))
			}
			setHint(keyName(tmpHK[i])+" is used by another app. Pick a different key", true)
			play(sndStop)
			return
		}
	}
	hk = tmpHK
	hkOK = [3]bool{true, true, true}
	hotkeysOK = true
	soundOn = tmpSound
	countdownOn = tmpCount
	saveSettings()
	play(sndToggle)
	closeSettings(true)
	refreshKeyHints()
	if state == stIdle {
		idleStatus()
	}
}

func openSettings() {
	if hwndSettings != 0 {
		pSetForegroundWindow.Call(hwndSettings)
		return
	}
	if !settingsClassOK {
		cursor, _, _ := pLoadCursorW.Call(0, 32512)
		wc := wndClassEx{
			WndProc:   syscall.NewCallback(settingsProc),
			Instance:  hInst,
			Icon:      appIconBig,
			IconSm:    appIconSmall,
			Cursor:    cursor,
			ClassName: u16("PeterAutoClickerSettings"),
		}
		wc.Size = uint32(unsafe.Sizeof(wc))
		pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
		settingsClassOK = true
	}
	tmpHK, tmpSound, tmpCount = hk, soundOn, countdownOn
	settingsHint = ""
	unregisterHotkeys() // so the key you press reaches the capture box

	style := uint32(wsCaption | wsSysMenu | 0x02000000)
	cr := rect{0, 0, sc(setW), sc(setH)}
	pAdjustWindowRectEx.Call(uintptr(unsafe.Pointer(&cr)), uintptr(style), 0, 0)
	var mr rect
	pGetWindowRect.Call(hwndMain, uintptr(unsafe.Pointer(&mr)))
	ww, wh := cr.R-cr.L, cr.B-cr.T
	x := mr.L + (mr.R-mr.L-ww)/2
	y := mr.T + sc(60)
	hwndSettings, _, _ = pCreateWindowExW.Call(0, up("PeterAutoClickerSettings"), up("Peter Auto Clicker"),
		uintptr(style), uintptr(x), uintptr(y), uintptr(ww), uintptr(wh), hwndMain, 0, hInst, 0)
	if hwndSettings == 0 {
		registerHotkeys()
		return
	}
	mk := func(b box, id, role int, icon, label, tip string) uintptr {
		h := mkButtonIn(hwndSettings, settingsScene, b, id, role, icon, label, tip)
		settingsBtns = append(settingsBtns, h)
		return h
	}
	for i := range keyBtns {
		keyBtns[i] = mk(syKey(i), idKey0+i, roleKey, "", keyName(tmpHK[i]), "Click, then press the key you want")
	}
	hSoundBtn = mk(sySound, idSound, roleSwitchRow, "vol", "Click sounds", "Play a short sound when you press a button")
	hCountBtn = mk(syCount, idCount, roleSwitchRow, "timer", "Countdown (3 seconds)", "Gives you time to click into your game before playback starts")
	mk(syReset, idReset, roleTile, "", "Reset", "Go back to F6 / F7 / F8")
	mk(syCancel, idCancel, roleTile, "", "Cancel", "")
	mk(sySave, idSaveSet, roleGold, "", "Save", "")
	refreshSettingsButtons()

	pEnableWindow.Call(hwndMain, 0)
	applyTheme()
	pShowWindow.Call(hwndSettings, 1)
	pUpdateWindow.Call(hwndSettings)
}

func closeSettings(saved bool) {
	if hwndSettings == 0 {
		return
	}
	capturing = 0
	if !saved {
		registerHotkeys() // put the old keys back
	}
	pEnableWindow.Call(hwndMain, 1)
	for _, h := range settingsBtns { // forget the dialog's buttons
		delete(btns, h)
		delete(hovered, h)
		delete(toggled, h)
	}
	settingsBtns = nil
	w := hwndSettings
	hwndSettings = 0
	pDestroyWindow.Call(w)
	pSetForegroundWindow.Call(hwndMain)
}
