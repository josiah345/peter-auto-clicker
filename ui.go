// ui.go — main window: layout, original arena background, owner-drawn controls.
package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
	"unsafe"
)

const (
	appVersion = "1.2"
	repoURL    = "https://github.com/josiah345/peter-auto-clicker"
	logoInner  = 74
)

var (
	pShellExecuteW  = shell32.NewProc("ShellExecuteW")
	pSetTimer       = user32.NewProc("SetTimer")
	pKillTimer      = user32.NewProc("KillTimer")
	pGetTickCount64 = kernel32.NewProc("GetTickCount64")
	pSetCursor      = user32.NewProc("SetCursor")
	pScreenToClient = user32.NewProc("ScreenToClient")
	pSetFocus       = user32.NewProc("SetFocus")
	pGetKeyState    = user32.NewProc("GetKeyState")
	pMapVirtualKey  = user32.NewProc("MapVirtualKeyW")
	pGetKeyNameText = user32.NewProc("GetKeyNameTextW")
)

// ───────────────────────────── Layout (logical px @96 dpi) ─────────────────────────────

const (
	winW = 440
	winH = 540
)

var (
	lyHeader = box{0, 0, winW, 108}
	lyLogo   = box{16, 14, 80, 80}
	lyPin    = box{328, 10, 28, 28}
	lyMoon   = box{362, 10, 28, 28}
	lyGear   = box{396, 10, 28, 28}
	lyStatus = box{16, 120, 408, 56}

	lyRec  = box{16, 188, 128, 84}
	lyPlay = box{156, 188, 128, 84}
	lyStop = box{296, 188, 128, 84}

	lyCellRepeat = box{16, 286, 128, 64}
	lyCellPause  = box{156, 286, 128, 64}
	lyCellSpeed  = box{296, 286, 128, 64}
	lyFCount     = box{54, 314, 54, 28}
	lyInf        = box{114, 314, 26, 28}
	lyFGap       = box{194, 314, 56, 28}
	lyFSpeed     = box{334, 314, 56, 28}

	lyMode    = box{16, 362, 408, 48}
	lyTrack   = box{88, 368, 330, 36}
	lyPrecise = box{92, 371, 159, 30}
	lySmart   = box{255, 371, 159, 30}

	lyAutoStop = box{16, 442, 80, 56}
	lyHide     = box{104, 442, 80, 56}
	lySave     = box{200, 442, 69, 56}
	lyLoad     = box{277, 442, 69, 56}
	lyClear    = box{354, 442, 70, 56}

	lyFooter = box{0, 510, winW, 20}
)

// ───────────────────────────── Original background ─────────────────────────────

type spark struct {
	x, y, r int32
	a       uint8
	gold    bool
}

var sparks []spark

func makeSparks() {
	rng := rand.New(rand.NewSource(7)) // fixed seed: same pattern every time
	for i := 0; i < 70; i++ {
		sparks = append(sparks, spark{
			x: int32(rng.Intn(winW)), y: int32(20 + rng.Intn(winH-20)),
			r:    int32(1 + rng.Intn(3)),
			a:    uint8(25 + rng.Intn(90)),
			gold: rng.Intn(3) > 0,
		})
	}
}

// paintBackdrop: gradient, light rays from above, blue/gold glows, sparkles, fine diagonal grain.
func paintBackdrop(p *painter, clip rect, w, h int32) {
	pl := pal()
	{
		var b uintptr
		p1, p2 := point{0, 0}, point{0, h}
		gpCreateLineBrushI.Call(uintptr(unsafe.Pointer(&p1)), uintptr(unsafe.Pointer(&p2)), uintptr(pl.bg1), uintptr(pl.bg2), 3, uintptr(unsafe.Pointer(&b)))
		gpFillRectI.Call(p.g, b, i32(clip.L), i32(clip.T), i32(clip.R-clip.L), i32(clip.B-clip.T))
		gpDeleteBrush.Call(b)
	}
	// light rays fanning down from above the window
	rayCol := A(6, 255, 255, 255)
	if !darkMode {
		rayCol = A(26, 255, 236, 180)
	}
	src := point{w / 2, -sc(120)}
	for i, ang := range []float64{-38, -22, -9, 4, 17, 31} {
		half := 2.2 + float64(i%2)*1.6
		var path uintptr
		gpCreatePath.Call(0, uintptr(unsafe.Pointer(&path)))
		L := float64(h) * 1.4
		pts := []point{src,
			{src.X + int32(L*sinDeg(ang-half)), src.Y + int32(L*cosDeg(ang-half))},
			{src.X + int32(L*sinDeg(ang+half)), src.Y + int32(L*cosDeg(ang+half))}}
		gpAddPathLine2I.Call(path, uintptr(unsafe.Pointer(&pts[0])), 3)
		gpClosePathFigure.Call(path)
		var br uintptr
		gpCreateSolidFill.Call(uintptr(rayCol), uintptr(unsafe.Pointer(&br)))
		gpFillPath.Call(p.g, br, path)
		gpDeleteBrush.Call(br)
		gpDeletePath.Call(path)
	}
	// glows
	radial := func(r rect, c argb) {
		if !hit(r, clip) {
			return
		}
		var path, br uintptr
		gpCreatePath.Call(0, uintptr(unsafe.Pointer(&path)))
		gpAddPathEllipseI.Call(path, i32(r.L), i32(r.T), i32(r.R-r.L), i32(r.B-r.T))
		gpPathGradFromPath.Call(path, uintptr(unsafe.Pointer(&br)))
		gpPathGradCenter.Call(br, uintptr(c))
		cols := []uint32{0}
		n := int32(1)
		gpPathGradSurround.Call(br, uintptr(unsafe.Pointer(&cols[0])), uintptr(unsafe.Pointer(&n)))
		gpFillPath.Call(p.g, br, path)
		gpDeleteBrush.Call(br)
		gpDeletePath.Call(path)
	}
	radial(rect{-sc(40), sc(110), w + sc(40), sc(400)}, pl.glow)
	radial(rect{w / 2, h - sc(170), w + sc(140), h + sc(120)}, gold1.withA(map[bool]uint8{true: 34, false: 50}[darkMode]))
	// sparkles
	for _, s := range sparks {
		x, y, r := sc(int(s.x)), sc(int(s.y)), sc(int(s.r))
		if x+r < clip.L || x-r > clip.R || y+r < clip.T || y-r > clip.B {
			continue
		}
		a := uint8(int(s.a) * 3 / 5) // softer sparkles
		c := A(a, 255, 255, 255)
		if s.gold {
			c = gold1.withA(a)
		}
		if !darkMode {
			c = gold2.withA(s.a / 2)
		}
		var br uintptr
		gpCreateSolidFill.Call(uintptr(c), uintptr(unsafe.Pointer(&br)))
		gpFillEllipseI.Call(p.g, br, i32(x-r), i32(y-r), i32(2*r), i32(2*r))
		gpDeleteBrush.Call(br)
	}
	// fine diagonal grain
	var pen uintptr
	gpCreatePen1.Call(uintptr(pl.line), f32(1), 2, uintptr(unsafe.Pointer(&pen)))
	for k := -h; k < w; k += sc(14) {
		if k+h < clip.L || k > clip.R {
			continue
		}
		gpDrawLineI.Call(p.g, pen, i32(k), i32(h), i32(k+h), 0)
	}
	gpDeletePen.Call(pen)
}

func sinDeg(d float64) float64 { return math.Sin(d * math.Pi / 180) }
func cosDeg(d float64) float64 { return math.Cos(d * math.Pi / 180) }

func headerBand(p *painter, clip rect, w int32, hr rect) {
	if !hit(hr, clip) {
		return
	}
	pl := pal()
	var b uintptr
	p1, p2 := point{0, hr.T}, point{0, hr.B}
	gpCreateLineBrushI.Call(uintptr(unsafe.Pointer(&p1)), uintptr(unsafe.Pointer(&p2)), uintptr(pl.head1.withA(240)), uintptr(pl.head2.withA(225)), 3, uintptr(unsafe.Pointer(&b)))
	gpFillRectI.Call(p.g, b, i32(hr.L), i32(hr.T), i32(hr.R-hr.L), i32(hr.B-hr.T))
	gpDeleteBrush.Call(b)

	var rule uintptr
	q1, q2 := point{0, 0}, point{w, 0}
	gpCreateLineBrushI.Call(uintptr(unsafe.Pointer(&q1)), uintptr(unsafe.Pointer(&q2)), uintptr(gold1.withA(0)), uintptr(gold1.withA(0)), 3, uintptr(unsafe.Pointer(&rule)))
	cols := []uint32{uint32(gold1.withA(0)), uint32(gold1), uint32(gold1.withA(0))}
	pos := []float32{0, 0.5, 1}
	gpSetLinePreset.Call(rule, uintptr(unsafe.Pointer(&cols[0])), uintptr(unsafe.Pointer(&pos[0])), 3)
	gpFillRectI.Call(p.g, rule, 0, i32(hr.B-sc(2)), i32(w), i32(sc(2)))
	gpDeleteBrush.Call(rule)
}

// caption-only "card" used for setting groups
func (p *painter) field(r rect) {
	pl := pal()
	p.fillRound(r, sc(8), pl.field, pl.field)
	p.strokeRound(r, sc(8), pl.fieldB, 1)
}

// paintScene draws everything in the main window that is not a control.
func paintScene(p *painter, clip rect) {
	pl := pal()
	gpSetClipRectI.Call(p.g, i32(clip.L), i32(clip.T), i32(clip.R-clip.L), i32(clip.B-clip.T), 0) // skip work outside
	W, H := sc(winW), sc(winH)
	paintBackdrop(p, clip, W, H)
	headerBand(p, clip, W, lyHeader.r())

	if lr := lyLogo.r(); hit(lr, clip) {
		p.glow(lr, sc(16), A(140, 240, 200, 90), int(sc(6)))
		p.fillRound(lr, sc(16), gold1, gold2)
		if logoBmp != 0 {
			in := rect{lr.L + sc(3), lr.T + sc(3), lr.L + sc(3) + sc(logoInner), lr.T + sc(3) + sc(logoInner)}
			clipPath := roundPath(in, sc(13))
			gpSetClipPath.Call(p.g, clipPath, 1) // CombineModeIntersect (keeps the button clip)
			gpDrawImageRectI.Call(p.g, logoBmp, i32(in.L), i32(in.T), i32(in.R-in.L), i32(in.B-in.T))
			gpResetClip.Call(p.g)
			gpDeletePath.Call(clipPath)
		}
	}

	if sr := lyStatus.r(); hit(sr, clip) {
		p.card(sr)
		dot := statusDot()
		cx, cy := sr.L+sc(21), sr.T+sc(18)
		rad := sc(5)
		glowA := uint8(255)
		if state != stIdle { // gentle pulse: 1.2 s cycle
			ph := (1 + math.Sin(float64(animTick())/1200*2*math.Pi)) / 2
			rad += int32(ph * float64(sc(2)))
			glowA = uint8(90 + ph*165)
		}
		dr := rect{cx - rad, cy - rad, cx + rad, cy + rad}
		p.glow(dr, rad, dot.withA(glowA), int(sc(5)))
		p.fillRound(dr, rad, dot, dot)
		p.icon("timer", rect{sr.L + sc(16), sr.T + sc(33), sr.L + sc(30), sr.T + sc(47)}, pl.muted)
		if cur != nil && len(cur.Ev) > 0 { // the click count only makes sense with a recording
			p.icon("clicks", rect{sr.L + sc(104), sr.T + sc(33), sr.L + sc(118), sr.T + sc(47)}, pl.muted)
		}
	}

	for _, c := range []struct {
		cell, f box
		ic      string
	}{{lyCellRepeat, lyFCount, "repeat"}, {lyCellPause, lyFGap, "gap"}, {lyCellSpeed, lyFSpeed, "speed"}} {
		cr := c.cell.r()
		if !hit(cr, clip) {
			continue
		}
		p.card(cr)
		fr := c.f.r()
		p.icon(c.ic, rect{cr.L + sc(12), fr.T + sc(5), cr.L + sc(30), fr.T + sc(23)}, pl.icon)
		p.field(fr)
	}

	if mr := lyMode.r(); hit(mr, clip) {
		p.card(mr)
		tr := lyTrack.r()
		p.fillRound(tr, sc(10), pl.field, pl.field)
		p.strokeRound(tr, sc(10), pl.fieldB, 1)
	}

	// divider between the OPTIONS and RECORDING groups
	if dv := (box{191, 446, 2, 48}).r(); hit(dv, clip) {
		p.fillRound(rect{dv.L, dv.T, dv.L + sc(1) + 1, dv.B}, 0, pl.border, pl.border)
	}
}

func statusDot() argb {
	switch statusKind {
	case cRed:
		return A(255, 236, 72, 72)
	case cGreen:
		return A(255, 46, 204, 113)
	case cBlue:
		return A(255, 80, 160, 255)
	case cOrange:
		return A(255, 245, 166, 35)
	}
	return gold1
}

// ───────────────────────────── Main window paint ─────────────────────────────

var (
	fTitle, fSub, fStatus, fInfo, fLabel, fKey, fTile, fField, fFoot, fUnit, fCap, fSmall uintptr

	statusText = ""
	statusKind = 0
	footerHot  = false
)

const (
	cNeutral = iota
	cRed
	cGreen
	cBlue
	cOrange
)

func capText(p *painter, s string, r rect) {
	p.texts = append(p.texts, textOp{s: s, r: r, font: fCap, col: pal().muted, flags: 0x20 | 0x4, extra: sc(1)})
}

func footerLinkRect() rect {
	f := lyFooter.r()
	return rect{sc(250), f.T, sc(winW - 16), f.B}
}

func paintMain(hdc uintptr, clip rect) {
	var cr rect
	pGetClientRect.Call(hwndMain, uintptr(unsafe.Pointer(&cr)))
	w, h := cr.R, cr.B
	if clip.R <= clip.L || clip.B <= clip.T {
		clip = cr
	}
	mem, _, _ := pCreateCompatibleDC.Call(hdc)
	bmp, _, _ := pCreateCompatibleBmp.Call(hdc, uintptr(w), uintptr(h))
	old, _, _ := pSelectObject.Call(mem, bmp)

	p := newPainter(mem)
	paintScene(p, clip)
	pl := pal()

	// header text
	tr := rect{sc(112), sc(40), sc(winW - 8), sc(74)}
	p.text("Peter Auto Clicker", rect{tr.L + 1, tr.T + 2, tr.R + 1, tr.B + 2}, fTitle, A(255, 0, 0, 0), 0x20|0x4)
	p.text("Peter Auto Clicker", tr, fTitle, gold1, 0x20|0x4)
	p.texts = append(p.texts, textOp{s: "RECORD  ·  REPLAY  ·  REPEAT", r: rect{sc(113), sc(74), sc(winW - 8), sc(92)},
		font: fSub, col: A(255, 170, 186, 228), flags: 0x20 | 0x4, extra: sc(1)})

	// status card
	sr := lyStatus.r()
	p.text(statusText, rect{sr.L + sc(34), sr.T + sc(5), sr.R - sc(12), sr.T + sc(29)}, fStatus, pl.text, 0x20|0x4|0x8000) // END_ELLIPSIS
	t1, t2 := infoTexts()
	t1R := sr.L + sc(100)
	if t2 == "" {
		t1R = sr.R - sc(12) // no click count: let the text use the whole line
	}
	p.text(t1, rect{sr.L + sc(35), sr.T + sc(32), t1R, sr.T + sc(48)}, fInfo, pl.muted, 0x20|0x4)
	p.text(t2, rect{sr.L + sc(123), sr.T + sc(32), sr.R - sc(12), sr.T + sc(48)}, fInfo, pl.muted, 0x20|0x4)

	// cell captions + units
	for _, c := range []struct {
		b box
		s string
	}{{lyCellRepeat, "REPEAT"}, {lyCellPause, "PAUSE"}, {lyCellSpeed, "SPEED"}} {
		r := c.b.r()
		capText(p, c.s, rect{r.L + sc(13), r.T + sc(7), r.R, r.T + sc(23)})
	}
	g := lyFGap.r()
	p.text("ms", rect{g.R + sc(6), g.T, sc(lyCellPause.x + lyCellPause.w), g.B}, fUnit, pl.muted, 0x20|0x4)
	s := lyFSpeed.r()
	p.text("×", rect{s.R + sc(7), s.T, sc(lyCellSpeed.x + lyCellSpeed.w), s.B}, fUnit, pl.muted, 0x20|0x4)

	m := lyMode.r()
	capText(p, "MODE", rect{m.L + sc(14), m.T, sc(lyTrack.x), m.B})

	capText(p, "OPTIONS", rect{sc(18), sc(422), sc(190), sc(438)})
	capText(p, "RECORDING", rect{sc(202), sc(422), sc(424), sc(438)})

	// footer: version + link to the official page
	f := lyFooter.r()
	p.text("v"+appVersion, rect{f.L + sc(16), f.T, f.R, f.B}, fFoot, pl.muted, 0x20|0x4)
	lr := footerLinkRect()
	linkCol := gold2
	if darkMode {
		linkCol = gold1.mul(85)
	}
	if footerHot {
		linkCol = gold1
	}
	p.text("Official page on GitHub", rect{lr.L, lr.T, f.R - sc(16), lr.B}, fFoot, linkCol, 0x2|0x20|0x4) // right-aligned

	p.finish(mem)
	pBitBlt.Call(hdc, uintptr(clip.L), uintptr(clip.T), uintptr(clip.R-clip.L), uintptr(clip.B-clip.T), mem, uintptr(clip.L), uintptr(clip.T), 0xCC0020)
	pSelectObject.Call(mem, old)
	pDeleteObject.Call(bmp)
	pDeleteDC.Call(mem)
}

func infoTexts() (string, string) {
	if cur == nil || len(cur.Ev) == 0 {
		return "No recording yet", ""
	}
	clicks := 0
	for _, e := range cur.Ev {
		switch e.K {
		case kLDown, kRDown, kMDown, kXDown:
			clicks++
		}
	}
	unit := "clicks"
	if clicks == 1 {
		unit = "click"
	}
	return strconv.FormatFloat(float64(cur.Dur)/1e6, 'f', 1, 64) + " s", fmt.Sprintf("%d %s", clicks, unit)
}

// ───────────────────────────── Owner-drawn buttons ─────────────────────────────

const (
	roleTile = iota // icon (+ label) inline
	roleRec         // gem buttons
	rolePlay
	roleStop
	roleStack     // icon over a small label
	roleSeg       // segmented switch half
	roleHead      // small icon in the header
	roleChip      // small square toggle inside a cell
	roleKey       // hotkey capture field (settings)
	roleGold      // primary action (settings Save)
	roleSwitchRow // settings row: icon + label on the left, on/off switch on the right
)

type btnInfo struct {
	id           int
	pad          int
	danger       bool   // Delete: glows red on hover
	desc         string // second line (settings switch rows)
	role         int
	icon, iconOn string
	key          int // hotkey index for gem badges (-1 = none)
	label        string
	pos          rect // window rect in parent client coords (includes glow padding)
	scene        func(p *painter, clip rect)
}

var (
	btns    = map[uintptr]*btnInfo{}
	toggled = map[uintptr]bool{}
	hovered = map[uintptr]bool{}
	hTip    uintptr
	tipStrs [][]uint16
)

type drawItemStruct struct {
	CtlType, CtlID, ItemID, ItemAction, ItemState uint32
	Hwnd, Hdc                                     uintptr
	Rc                                            rect
	ItemData                                      uintptr
}

func drawButton(d *drawItemStruct) {
	bi := btns[d.Hwnd]
	if bi == nil {
		return
	}
	w, h := d.Rc.R-d.Rc.L, d.Rc.B-d.Rc.T
	mem, _, _ := pCreateCompatibleDC.Call(d.Hdc)
	bmp, _, _ := pCreateCompatibleBmp.Call(d.Hdc, uintptr(w), uintptr(h))
	old, _, _ := pSelectObject.Call(mem, bmp)

	p := newPainter(mem)
	gpTranslate.Call(p.g, f32(float32(-bi.pos.L)), f32(float32(-bi.pos.T)), 0)
	bi.scene(p, bi.pos)
	gpResetTransform.Call(p.g)
	gpResetClip.Call(p.g)

	pressed := d.ItemState&0x1 != 0
	disabled := d.ItemState&0x4 != 0
	hover := hovered[d.Hwnd] && !disabled
	on := toggled[d.Hwnd]
	in := sc(bi.pad)
	vr := rect{in, in, w - in, h - in}
	if pressed {
		vr = rect{vr.L + 1, vr.T + 1, vr.R + 1, vr.B + 1}
	}
	pl := pal()
	icon := bi.icon
	if on && bi.iconOn != "" {
		icon = bi.iconOn
	}
	cx, cy := (vr.L+vr.R)/2, (vr.T+vr.B)/2

	switch bi.role {
	case roleRec, rolePlay, roleStop:
		var c1, c2 argb
		switch bi.role {
		case roleRec:
			c1, c2 = A(255, 232, 80, 80), A(255, 150, 28, 44)
		case rolePlay:
			c1, c2 = A(255, 52, 200, 120), A(255, 16, 120, 70)
		default:
			c1, c2 = A(255, 90, 106, 150), A(255, 44, 54, 88)
		}
		fg := A(255, 255, 255, 255)
		border := gold1.withA(230)
		if disabled {
			if darkMode {
				c1, c2, fg = A(255, 44, 52, 82), A(255, 32, 38, 64), A(255, 110, 120, 150)
			} else {
				c1, c2, fg = A(255, 226, 220, 204), A(255, 214, 206, 186), A(255, 170, 160, 140)
			}
			border = pl.border
		} else if pressed {
			c1, c2 = c1.mul(88), c2.mul(88)
		} else if hover {
			c1, c2 = c1.mul(110), c2.mul(110)
		}
		rad := sc(14)
		if hover {
			p.glow(vr, rad, gold1.withA(200), int(in))
		}
		p.fillRound(vr, rad, c1, c2)
		gloss := A(28, 255, 255, 255)
		if disabled {
			gloss = A(8, 255, 255, 255)
		}
		p.fillRound(rect{vr.L + sc(3), vr.T + sc(3), vr.R - sc(3), vr.T + (vr.B-vr.T)*45/100}, sc(11), gloss, gloss)
		p.strokeRound(vr, rad, border, float32(sc(3))/2)
		p.icon(icon, iconRect(cx, vr.T+sc(24), sc(24)), fg)
		p.text(bi.label, rect{vr.L, vr.T + sc(40), vr.R, vr.T + sc(60)}, fLabel, fg, 0x1|0x20|0x4)
		if bi.key >= 0 {
			name := hkName(bi.key)
			kw := textWidth(mem, fKey, name) + sc(14)
			pill := rect{cx - kw/2, vr.T + sc(63), cx - kw/2 + kw, vr.T + sc(77)}
			pc := A(70, 0, 0, 0)
			kc := fg.withA(230)
			if disabled { // keep the key readable on a greyed-out button
				if darkMode {
					pc, kc = A(40, 0, 0, 0), A(255, 140, 150, 180)
				} else {
					pc, kc = A(22, 60, 40, 0), A(255, 140, 126, 100)
				}
			}
			if !hkOK[bi.key] { // that key is taken by another app
				pc, kc = A(230, 245, 166, 35), navy
			}
			p.fillRound(pill, sc(7), pc, pc)
			p.text(name, pill, fKey, kc, 0x1|0x20|0x4)
		}

	case roleSeg:
		fg := pl.muted
		if on {
			p.fillRound(vr, sc(8), gold1, gold2)
			p.strokeRound(vr, sc(8), A(255, 255, 236, 170), 1)
			fg = navy
		} else if hover || pressed {
			hc := pl.card1.withA(200)
			p.fillRound(vr, sc(8), hc, hc)
			fg = pl.text
		}
		if disabled {
			fg = fg.withA(120)
		}
		tw := textWidth(mem, fTile, bi.label)
		isz := sc(18)
		x0 := cx - (isz+sc(7)+tw)/2
		p.icon(icon, rect{x0, cy - isz/2, x0 + isz, cy - isz/2 + isz}, fg)
		p.text(bi.label, rect{x0 + isz + sc(7), vr.T, vr.R, vr.B}, fTile, fg, 0x20|0x4)

	case roleHead:
		bg := A(26, 255, 255, 255)
		fg := A(255, 232, 214, 165)
		if on {
			p.fillRound(vr, sc(8), gold1, gold2)
			fg = navy
		} else {
			if hover {
				bg = A(50, 255, 255, 255)
			}
			p.fillRound(vr, sc(8), bg, bg)
			if hover {
				p.strokeRound(vr, sc(8), gold1.withA(200), 1)
			}
		}
		p.icon(icon, iconRect(cx, cy, sc(16)), fg)

	case roleKey:
		fill, bc, fg := pl.field, pl.fieldB, pl.text
		label := bi.label
		if capturing == d.Hwnd {
			bc, fg, label = gold1, gold1, "Press a key…"
			if !darkMode {
				fg = gold2
			}
		} else if hover {
			bc = gold1
		}
		p.fillRound(vr, sc(8), fill, fill)
		p.strokeRound(vr, sc(8), bc, float32(sc(3))/2)
		p.text(label, vr, fTile, fg, 0x1|0x20|0x4)

	case roleSwitchRow:
		rad := sc(12)
		c1, c2 := pl.card1, pl.card2
		if hover {
			c1, c2 = c1.mul(106), c2.mul(106)
		}
		p.fillRound(vr, rad, c1, c2)
		bc := pl.border
		if hover {
			bc = gold1
		}
		p.strokeRound(vr, rad, bc, float32(sc(3))/2)
		p.icon(icon, iconRect(vr.L+sc(26), cy, sc(20)), pl.icon)
		if bi.desc == "" {
			p.text(bi.label, rect{vr.L + sc(46), vr.T, vr.R - sc(70), vr.B}, fTile, pl.text, 0x20|0x4)
		} else {
			p.text(bi.label, rect{vr.L + sc(46), vr.T + sc(8), vr.R - sc(70), cy + sc(1)}, fTile, pl.text, 0x20|0x8)
			p.text(bi.desc, rect{vr.L + sc(46), cy + sc(3), vr.R - sc(70), vr.B - sc(6)}, fInfo, pl.muted, 0x20)
		}
		// the switch
		tw, th := sc(42), sc(22)
		tr := rect{vr.R - sc(14) - tw, cy - th/2, vr.R - sc(14), cy - th/2 + th}
		ks := th - sc(6)
		if on {
			p.fillRound(tr, th/2, gold1, gold2)
			k := rect{tr.R - sc(3) - ks, tr.T + sc(3), tr.R - sc(3), tr.T + sc(3) + ks}
			p.fillRound(k, ks/2, navy, navy)
		} else {
			p.fillRound(tr, th/2, pl.field, pl.field)
			p.strokeRound(tr, th/2, pl.fieldB, 1)
			k := rect{tr.L + sc(3), tr.T + sc(3), tr.L + sc(3) + ks, tr.T + sc(3) + ks}
			p.fillRound(k, ks/2, pl.muted, pl.muted)
		}

	default: // roleTile, roleStack, roleChip, roleGold
		rad := sc(12)
		if bi.role == roleChip {
			rad = sc(7)
		}
		hoverCol := gold1
		if bi.danger {
			hoverCol = A(255, 236, 72, 72)
		}
		if hover {
			p.glow(vr, rad, hoverCol.withA(170), int(in))
		}
		var fg argb
		if on || bi.role == roleGold {
			c1, c2 := gold1, gold2
			if pressed {
				c1, c2 = c1.mul(90), c2.mul(90)
			} else if hover && bi.role == roleGold {
				c1, c2 = c1.mul(106), c2.mul(106)
			}
			p.fillRound(vr, rad, c1, c2)
			p.strokeRound(vr, rad, A(255, 255, 236, 170), float32(sc(3))/2)
			fg = navy
		} else {
			c1, c2 := pl.card1, pl.card2
			if pressed {
				c1, c2 = c1.mul(92), c2.mul(92)
			} else if hover {
				c1, c2 = c1.mul(108), c2.mul(108)
			}
			p.fillRound(vr, rad, c1, c2)
			bc := pl.border
			if hover {
				bc = hoverCol
			}
			p.strokeRound(vr, rad, bc, float32(sc(3))/2)
			fg = pl.icon
			if hover && bi.danger {
				fg = A(255, 255, 120, 120)
				if !darkMode {
					fg = A(255, 200, 40, 40)
				}
			}
		}
		if disabled {
			fg = pl.muted.withA(140)
		}
		switch {
		case bi.role == roleStack:
			p.icon(icon, iconRect(cx, vr.T+sc(19), sc(20)), fg)
			lc := fg
			if !on {
				lc = pl.text.withA(220)
			}
			p.text(bi.label, rect{vr.L, vr.T + sc(32), vr.R, vr.B - sc(4)}, fSmall, lc, 0x1|0x20|0x4)
		case bi.role == roleChip:
			p.icon(icon, iconRect(cx, cy, sc(16)), fg)
		case bi.label == "":
			p.icon(icon, iconRect(cx, cy, sc(22)), fg)
		case icon == "":
			p.text(bi.label, vr, fTile, fg, 0x1|0x20|0x4)
		default:
			tw := textWidth(mem, fTile, bi.label)
			isz := sc(20)
			x0 := cx - (isz+sc(8)+tw)/2
			p.icon(icon, rect{x0, cy - isz/2, x0 + isz, cy - isz/2 + isz}, fg)
			p.text(bi.label, rect{x0 + isz + sc(8), vr.T, vr.R, vr.B}, fTile, fg, 0x20|0x4)
		}
	}

	if d.ItemState&0x10 != 0 && d.ItemState&0x200 == 0 && !disabled { // ODS_FOCUS, not ODS_NOFOCUSRECT
		fr := rect{vr.L + 1, vr.T + 1, vr.R - 1, vr.B - 1}
		p.strokeRound(fr, sc(12), A(255, 120, 180, 255), float32(sc(2)))
	}

	p.finish(mem)
	pBitBlt.Call(d.Hdc, 0, 0, uintptr(w), uintptr(h), mem, 0, 0, 0xCC0020)
	pSelectObject.Call(mem, old)
	pDeleteObject.Call(bmp)
	pDeleteDC.Call(mem)
}

func textWidth(hdc, font uintptr, s string) int32 {
	old, _, _ := pSelectObject.Call(hdc, font)
	var sz struct{ cx, cy int32 }
	u := syscall.StringToUTF16(s)
	pGetTextExtent.Call(hdc, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), uintptr(unsafe.Pointer(&sz)))
	pSelectObject.Call(hdc, old)
	return sz.cx
}

// pressSounded marks buttons whose sound already played on mouse-down,
// so the click that follows doesn't play it a second time.
var pressSounded = map[uintptr]bool{}

func soundFor(h uintptr) *sound {
	bi := btns[h]
	if bi == nil {
		return nil
	}
	switch bi.id {
	case idRecord:
		if state == stRecording {
			return sndFinish
		}
		return sndRec
	case idPlay:
		return sndPlay
	case idStop:
		return sndStop
	case idInf, idPrecise, idSmart, idAutoStop, idHide, idDark, idPin:
		return sndToggle
	case idGear, idSave, idLoad, idClear, idReset, idCancel:
		return sndTick
	}
	return nil
}

// clickSound plays s for a button click unless it already played on mouse-down
// (keyboard Space/Enter clicks still get their sound here).
func clickSound(ctl uintptr, s *sound) {
	if pressSounded[ctl] {
		pressSounded[ctl] = false
		return
	}
	play(s)
}

// hover tracking + key capture via subclassing
var subclassCB = syscall.NewCallback(func(hwnd, msg, wp, lp, id, ref uintptr) uintptr {
	switch msg {
	case 0x0200: // WM_MOUSEMOVE
		if !hovered[hwnd] {
			hovered[hwnd] = true
			tme := struct {
				Size, Flags uint32
				Hwnd        uintptr
				Hover       uint32
				_           uint32
			}{Flags: 0x2, Hwnd: hwnd} // TME_LEAVE
			tme.Size = uint32(unsafe.Sizeof(tme))
			pTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tme)))
			pInvalidateRect.Call(hwnd, 0, 0)
		}
	case 0x02A3: // WM_MOUSELEAVE
		hovered[hwnd] = false
		pInvalidateRect.Call(hwnd, 0, 0)
	case 0x0014: // WM_ERASEBKGND
		return 1
	case 0x0201: // WM_LBUTTONDOWN: click sound the moment you press
		if r, _, _ := pIsWindowEnabled.Call(hwnd); r != 0 {
			if s := soundFor(hwnd); s != nil {
				play(s)
				pressSounded[hwnd] = true
			}
		}
	case 0x0087: // WM_GETDLGCODE
		if capturing == hwnd {
			return 0x4 // DLGC_WANTALLKEYS
		}
	case 0x0100, 0x0104: // WM_KEYDOWN, WM_SYSKEYDOWN
		if capturing == hwnd {
			captureKey(hwnd, uint32(wp))
			return 0
		}
	case 0x0101, 0x0105, 0x0102, 0x0106: // key up / char messages while capturing
		if capturing == hwnd {
			return 0
		}
	case 0x0008: // WM_KILLFOCUS
		if capturing == hwnd {
			capturing = 0
			pInvalidateRect.Call(hwnd, 0, 0)
		}
	}
	r, _, _ := pDefSubclassProc.Call(hwnd, msg, wp, lp)
	return r
})

func mkButtonIn(parent uintptr, scene func(*painter, rect), b box, id int, role int, icon, label, tip string) uintptr {
	pad := 3
	switch role {
	case roleRec, rolePlay, roleStop:
		pad = 4
	case roleHead, roleSeg, roleChip, roleKey:
		pad = 0
	}
	x, y, w, h := b.x-pad, b.y-pad, b.w+2*pad, b.h+2*pad
	hw, _, _ := pCreateWindowExW.Call(0, up("BUTTON"), up(label),
		uintptr(wsChild|wsVisible|wsTabStop|0xB), // BS_OWNERDRAW
		uintptr(sc(x)), uintptr(sc(y)), uintptr(sc(x+w)-sc(x)), uintptr(sc(y+h)-sc(y)),
		parent, uintptr(id), hInst, 0)
	btns[hw] = &btnInfo{id: id, pad: pad, role: role, icon: icon, key: -1, label: label,
		pos: rect{sc(x), sc(y), sc(x + w), sc(y + h)}, scene: scene}
	pSetWindowSubcls.Call(hw, subclassCB, 1, 0)
	addTip(parent, hw, tip)
	return hw
}

func mkButton(b box, id int, role int, icon, label, tip string) uintptr {
	return mkButtonIn(hwndMain, paintScene, b, id, role, icon, label, tip)
}

var tipOwner = map[uintptr]uintptr{}

func toolInfo(owner, ctl uintptr, t *uint16) unsafe.Pointer {
	ti := &struct {
		Size, Flags uint32
		Hwnd, ID    uintptr
		Rc          rect
		Inst        uintptr
		Text        *uint16
		LParam      uintptr
		Reserved    uintptr
	}{Flags: 0x1 | 0x10, Hwnd: owner, ID: ctl, Text: t} // IDISHWND | SUBCLASS
	ti.Size = uint32(unsafe.Sizeof(*ti))
	return unsafe.Pointer(ti)
}

func addTip(owner, ctl uintptr, text string) {
	if hTip == 0 || text == "" {
		return
	}
	t := syscall.StringToUTF16(text)
	tipStrs = append(tipStrs, t)
	tipOwner[ctl] = owner
	pSendMessageW.Call(hTip, 0x432, 0, uintptr(toolInfo(owner, ctl, &t[0]))) // TTM_ADDTOOLW
}

func setTip(ctl uintptr, text string) {
	t := syscall.StringToUTF16(text)
	tipStrs = append(tipStrs, t)
	pSendMessageW.Call(hTip, 0x439, 0, uintptr(toolInfo(tipOwner[ctl], ctl, &t[0]))) // TTM_UPDATETIPTEXTW
}

func setLabel(h uintptr, label, icon string) {
	if bi := btns[h]; bi != nil {
		bi.label = label
		if icon != "" {
			bi.icon = icon
		}
		pInvalidateRect.Call(h, 0, 0)
	}
}

// ───────────────────────────── Controls & state helpers ─────────────────────────────

var (
	hRecord, hPlay, hStop            uintptr
	hEditCount, hEditGap, hEditSpeed uintptr
	hInf, hPrecise, hSmart           uintptr
	hAutoStop, hHide, hPin, hDark    uintptr
	hGear, hSave, hLoad, hClear      uintptr
	allInputs                        []uintptr
	edits                            = map[uintptr]bool{}
	fieldBrush                       uintptr
)

const (
	idRecord = 101 + iota
	idPlay
	idStop
	idEditCount
	idInf
	idEditGap
	idEditSpeed
	idPrecise
	idSmart
	idAutoStop
	idHide
	idPin
	idDark
	idGear
	idSave
	idLoad
	idClear
)

func mkEdit(f box, id int, text string, numeric bool, tip string) uintptr {
	eh := 20
	style := uint32(wsChild | wsVisible | wsTabStop | esAutoH | esCenter)
	if numeric {
		style |= esNumber
	}
	x, y := f.x+4, f.y+(f.h-eh)/2
	hw, _, _ := pCreateWindowExW.Call(0, up("EDIT"), up(text), uintptr(style),
		uintptr(sc(x)), uintptr(sc(y)), uintptr(sc(x+f.w-8)-sc(x)), uintptr(sc(y+eh)-sc(y)),
		hwndMain, uintptr(id), hInst, 0)
	pSendMessageW.Call(hw, wmSetFont, fField, 1)
	pSendMessageW.Call(hw, 0x00C5, 5, 0) // EM_LIMITTEXT
	edits[hw] = true
	addTip(hwndMain, hw, tip)
	return hw
}

func setText(h uintptr, s string) { pSetWindowTextW.Call(h, up(s)) }
func getText(h uintptr) string {
	buf := make([]uint16, 256)
	pGetWindowTextW.Call(h, uintptr(unsafe.Pointer(&buf[0])), 256)
	return syscall.UTF16ToString(buf)
}
func isChecked(h uintptr) bool { return toggled[h] }
func setCheck(h uintptr, on bool) {
	toggled[h] = on
	pInvalidateRect.Call(h, 0, 0)
}
func enable(h uintptr, on bool) {
	v := uintptr(0)
	if on {
		v = 1
	}
	pEnableWindow.Call(h, v)
}

func msgBox(text string, flags uintptr) uintptr {
	owner := hwndMain
	if hwndSettings != 0 {
		owner = hwndSettings
	}
	r, _, _ := pMessageBoxW.Call(owner, up(text), up("Peter Auto Clicker"), flags)
	return r
}

func invalidateBox(b box) {
	r := b.r()
	pInvalidateRect.Call(hwndMain, uintptr(unsafe.Pointer(&r)), 0)
}

func setStatus(s string, kind int) {
	statusText, statusKind = s, kind
	invalidateBox(lyStatus)
}

func refreshInfo() { invalidateBox(lyStatus) }

// statusLightRect is the small area repainted by the pulse timer.
func statusLightRect() rect {
	sr := lyStatus.r()
	return rect{sr.L + sc(6), sr.T + sc(4), sr.L + sc(36), sr.T + sc(32)}
}

func animTick() int64 {
	t, _, _ := pGetTickCount64.Call()
	return int64(t)
}

func setBusy(busy bool) {
	if busy {
		pSetTimer.Call(hwndMain, 7, 90, 0) // pulse the status light
	} else {
		pKillTimer.Call(hwndMain, 7)
	}
	for _, h := range allInputs {
		enable(h, !busy)
	}
	enable(hStop, busy)
	enable(hPlay, !busy && cur != nil && len(cur.Ev) > 0)
	enable(hRecord, state != stPlaying)
	if state == stRecording {
		setLabel(hRecord, "FINISH", "stop")
	} else {
		setLabel(hRecord, "RECORD", "rec")
	}
	if !busy {
		enable(hEditCount, !isChecked(hInf))
	}
}

func idleStatus() {
	for i := range hk {
		if !hkOK[i] {
			setStatus(hkName(i)+" is taken. Click the gear to change it", cOrange)
			return
		}
	}
	if cur != nil && len(cur.Ev) > 0 {
		setStatus("Ready. Press Play to replay", cGreen)
	} else {
		setStatus("Press Record to start", cNeutral)
	}
}

func minimizeIfWanted() {
	if isChecked(hHide) {
		pShowWindow.Call(hwndMain, 6)
	}
}
func restoreIfMin() {
	if r, _, _ := pIsIconic.Call(hwndMain); r != 0 {
		pShowWindow.Call(hwndMain, 9)
	}
}

func refreshKeyHints() {
	for _, h := range []uintptr{hRecord, hPlay, hStop} {
		pInvalidateRect.Call(h, 0, 0)
	}
	setTip(hRecord, "Record your mouse  ("+hkName(0)+")\nPress again to finish")
	setTip(hPlay, "Play the recording  ("+hkName(1)+")")
	setTip(hStop, "Stop  ("+hkName(2)+")")
	setTip(hInf, "Loop forever (until you press "+hkName(2)+")")
}

func applyTheme() {
	if fieldBrush != 0 {
		pDeleteObject.Call(fieldBrush)
	}
	fieldBrush, _, _ = pCreateSolidBrush.Call(pal().field.ref())
	for _, w := range []uintptr{hwndMain, hwndSettings} {
		if w == 0 {
			continue
		}
		on := int32(1)
		pDwmSetAttr.Call(w, 20, uintptr(unsafe.Pointer(&on)), 4)
		pDwmSetAttr.Call(w, 19, uintptr(unsafe.Pointer(&on)), 4)
		capCol, txt, brd := uint32(lightPal.head1.ref()), uint32(gold1.ref()), uint32(gold2.ref())
		pDwmSetAttr.Call(w, 35, uintptr(unsafe.Pointer(&capCol)), 4)
		pDwmSetAttr.Call(w, 36, uintptr(unsafe.Pointer(&txt)), 4)
		pDwmSetAttr.Call(w, 34, uintptr(unsafe.Pointer(&brd)), 4)
		pRedrawWindow.Call(w, 0, 0, 0x1|0x4|0x80|0x100)
	}
}

func cursorIn(r rect) bool {
	var pt point
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	pScreenToClient.Call(hwndMain, uintptr(unsafe.Pointer(&pt)))
	return pt.X >= r.L && pt.X < r.R && pt.Y >= r.T && pt.Y < r.B
}

// ───────────────────────────── Window procedure ─────────────────────────────

func wndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case 0x002B: // WM_DRAWITEM
		drawButton((*drawItemStruct)(unsafe.Pointer(lParam)))
		return 1

	case 0x0014: // WM_ERASEBKGND
		return 1

	case 0x0113: // WM_TIMER: pulse animation (tiny area only)
		if wParam == 7 {
			r := statusLightRect()
			pInvalidateRect.Call(hwndMain, uintptr(unsafe.Pointer(&r)), 0)
		}
		return 0

	case wmPaint:
		var ps paintStruct
		hdc, _, _ := pBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		paintMain(hdc, ps.RcPaint)
		pEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0

	case 0x0020: // WM_SETCURSOR — hand over the footer link
		if loword(lParam) == 1 && cursorIn(footerLinkRect()) {
			c, _, _ := pLoadCursorW.Call(0, 32649) // IDC_HAND
			pSetCursor.Call(c)
			return 1
		}

	case 0x0200: // WM_MOUSEMOVE — link hover colour
		hot := cursorIn(footerLinkRect())
		if hot != footerHot {
			footerHot = hot
			invalidateBox(lyFooter)
			if hot {
				tme := struct {
					Size, Flags uint32
					Hwnd        uintptr
					Hover       uint32
					_           uint32
				}{Flags: 0x2, Hwnd: hwnd}
				tme.Size = uint32(unsafe.Sizeof(tme))
				pTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tme)))
			}
		}
		return 0

	case 0x02A3: // WM_MOUSELEAVE
		if footerHot {
			footerHot = false
			invalidateBox(lyFooter)
		}
		return 0

	case 0x0202: // WM_LBUTTONUP — open the official page
		if cursorIn(footerLinkRect()) {
			play(sndTick)
			pShellExecuteW.Call(0, up("open"), up(repoURL), 0, 0, 1)
		}
		return 0

	case wmCtlColorEdit, wmCtlColorStatic:
		if edits[lParam] {
			hdc := wParam
			pl := pal()
			c := pl.text
			if msg == wmCtlColorStatic { // disabled edit
				c = pl.muted
			}
			pSetTextColor.Call(hdc, c.ref())
			pSetBkColor.Call(hdc, pl.field.ref())
			return fieldBrush
		}

	case wmCommand:
		id := int(loword(wParam))
		code := hiword(wParam)
		if code != 0 && code != 5 { // BN_CLICKED / BN_DOUBLECLICKED
			break
		}
		switch id {
		case idRecord:
			if state == stRecording {
				clickSound(lParam, sndFinish)
				stopRecording(true)
			} else {
				clickSound(lParam, sndRec)
				startRecording()
			}
		case idPlay:
			clickSound(lParam, sndPlay)
			startPlaying()
		case idStop:
			clickSound(lParam, sndStop)
			stopAll(true)
		case idInf:
			clickSound(lParam, sndToggle)
			setCheck(hInf, !isChecked(hInf))
			enable(hEditCount, !isChecked(hInf))
		case idPrecise, idSmart:
			clickSound(lParam, sndToggle)
			setCheck(hPrecise, id == idPrecise)
			setCheck(hSmart, id == idSmart)
		case idAutoStop:
			clickSound(lParam, sndToggle)
			setCheck(hAutoStop, !isChecked(hAutoStop))
		case idHide:
			clickSound(lParam, sndToggle)
			setCheck(hHide, !isChecked(hHide))
		case idDark:
			clickSound(lParam, sndToggle)
			darkMode = !darkMode
			setCheck(hDark, darkMode)
			saveSettings()
			applyTheme()
		case idPin:
			clickSound(lParam, sndToggle)
			setCheck(hPin, !isChecked(hPin))
			after := ^uintptr(1) // HWND_NOTOPMOST
			if isChecked(hPin) {
				after = ^uintptr(0) // HWND_TOPMOST
			}
			pSetWindowPos.Call(hwndMain, after, 0, 0, 0, 0, 0x1|0x2|0x10)
		case idGear:
			clickSound(lParam, sndTick)
			openSettings()
		case idSave:
			clickSound(lParam, sndTick)
			if cur == nil || len(cur.Ev) == 0 {
				setStatus("Record something first", cOrange)
				break
			}
			if path := fileDialog(true); path != "" {
				if err := cur.save(path); err != nil {
					msgBox("Could not save: "+err.Error(), 0x10)
				} else {
					setStatus("Saved as "+filepath.Base(path), cGreen)
				}
			}
		case idLoad:
			clickSound(lParam, sndTick)
			if path := fileDialog(false); path != "" {
				r, err := loadRecording(path)
				if err != nil {
					msgBox("Could not open that file: "+err.Error(), 0x10)
					break
				}
				cur = r
				refreshInfo()
				setBusy(false)
				setStatus("Loaded. Press Play to replay", cGreen)
			}
		case idClear:
			clickSound(lParam, sndTick)
			if cur == nil || len(cur.Ev) == 0 {
				break
			}
			if msgBox("Delete this recording?", 0x4|0x20|0x100) != 6 { // IDYES
				break
			}
			cur = nil
			os.Remove(lastRecPath())
			refreshInfo()
			setBusy(false)
			idleStatus()
		}
		return 0

	case wmHotkey:
		switch wParam {
		case 1: // record / finish
			if state == stRecording {
				play(sndFinish)
				stopRecording(false)
			} else if state == stIdle {
				play(sndRec)
				startRecording()
			}
		case 2: // play (or stop if playing)
			if state == stPlaying {
				play(sndStop)
				stopAll(false)
			} else if state == stIdle {
				play(sndPlay)
				startPlaying()
			}
		case 3: // stop
			if state != stIdle {
				play(sndStop)
			}
			stopAll(false)
		}
		return 0

	case wmAppLoop:
		if state == stPlaying {
			mode := "Precise"
			if smartMode {
				mode = "Smart"
			}
			if playTotal > 0 {
				setStatus(fmt.Sprintf("Playing loop %d of %d  (%s mode)", wParam, playTotal, mode), cBlue)
			} else {
				setStatus(fmt.Sprintf("Playing loop %d, forever  (%s mode)", wParam, mode), cBlue)
			}
		}
		return 0

	case wmAppStopPlay:
		return 0

	case wmAppCount: // countdown before playback
		if state == stPlaying {
			if wParam > 0 {
				play(sndCount)
				setStatus(fmt.Sprintf("Starting in %d…  press %s to cancel", wParam, hkName(2)), cBlue)
			} else {
				play(sndGo)
				setStatus("Playing…  press "+hkName(2)+" to stop", cBlue)
			}
		}
		return 0

	case wmAppDone:
		wasStopped := stopFlag.Load()
		movedStop := stopOnMove && userMoved.Load() > moveStopEvents
		state = stIdle
		removeHook()
		restoreIfMin()
		setBusy(false)
		switch {
		case movedStop:
			setStatus("Stopped because you moved the mouse", cOrange)
		case wasStopped:
			setStatus("Stopped", cOrange)
		default:
			setStatus("Done!", cGreen)
		}
		return 0

	case wmClose:
		stopFlag.Store(true)
		if state == stPlaying && playDone != nil {
			select { // let playback release any held mouse button first
			case <-playDone:
			case <-time.After(time.Second):
			}
		}
		pDestroyWindow.Call(hwnd)
		return 0

	case wmDestroy:
		closeAudio()
		removeHook()
		unregisterHotkeys()
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return r
}

// ───────────────────────────── Build the window ─────────────────────────────

func mkFont(pt float64, weight int, face string) uintptr {
	h := -int(pt*float64(dpi)/72 + 0.5)
	f, _, _ := pCreateFontW.Call(uintptr(h), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, up(face))
	return f
}

func buildControls() {
	icc := [2]uint32{8, 0x8} // tooltips
	pInitCommonCtrls.Call(uintptr(unsafe.Pointer(&icc)))
	hTip, _, _ = pCreateWindowExW.Call(0x8, up("tooltips_class32"), 0, 0x80000000|0x1|0x2,
		0x80000000, 0x80000000, 0x80000000, 0x80000000, hwndMain, 0, hInst, 0)
	pSendMessageW.Call(hTip, 0x418, 0, uintptr(sc(260))) // TTM_SETMAXTIPWIDTH

	// header: small icons, top right
	hPin = mkButton(lyPin, idPin, roleHead, "pin", "", "Keep this window on top")
	hDark = mkButton(lyMoon, idDark, roleHead, "sun", "", "Dark mode")
	btns[hDark].iconOn = "moon"
	hGear = mkButton(lyGear, idGear, roleHead, "gear", "", "Settings: hotkeys and sounds")

	hRecord = mkButton(lyRec, idRecord, roleRec, "rec", "RECORD", "Record your mouse")
	hPlay = mkButton(lyPlay, idPlay, rolePlay, "play", "PLAY", "Play the recording")
	hStop = mkButton(lyStop, idStop, roleStop, "stop", "STOP", "Stop")
	btns[hRecord].key, btns[hPlay].key, btns[hStop].key = 0, 1, 2

	hEditCount = mkEdit(lyFCount, idEditCount, "10", true, "How many times to repeat")
	hInf = mkButton(lyInf, idInf, roleChip, "inf", "", "Loop forever")
	hEditGap = mkEdit(lyFGap, idEditGap, "0", true, "Pause between loops, in milliseconds")
	hEditSpeed = mkEdit(lyFSpeed, idEditSpeed, "1", false, "Speed:  1 = real pace,  2 = twice as fast,  0.5 = half speed")

	hPrecise = mkButton(lyPrecise, idPrecise, roleSeg, "precise", "Precise", "Replays every movement at the exact pace")
	hSmart = mkButton(lySmart, idSmart, roleSeg, "smart", "Smart", "Use if it lags.\nKeeps the rhythm, never misses a click")

	hAutoStop = mkButton(lyAutoStop, idAutoStop, roleStack, "hand", "Auto-stop", "Stop playback when you move the mouse yourself")
	hHide = mkButton(lyHide, idHide, roleStack, "hide", "Hide", "Minimize this window while recording or playing")
	hSave = mkButton(lySave, idSave, roleStack, "save", "Save", "Save this recording to a file")
	hLoad = mkButton(lyLoad, idLoad, roleStack, "open", "Open", "Open a saved recording")
	hClear = mkButton(lyClear, idClear, roleStack, "trash", "Delete", "Delete this recording")
	btns[hClear].danger = true

	toggled[hPrecise] = true
	toggled[hAutoStop] = true
	toggled[hDark] = darkMode

	allInputs = []uintptr{hPlay, hEditCount, hInf, hEditGap, hEditSpeed, hPrecise, hSmart,
		hAutoStop, hHide, hPin, hDark, hGear, hSave, hLoad, hClear}
}

func main() {
	if pSetDpiAwarenessCtx.Find() == nil {
		pSetDpiAwarenessCtx.Call(^uintptr(3))
	} else {
		pSetProcessDPIAware.Call()
	}
	if pGetDpiForSystem.Find() == nil {
		if d, _, _ := pGetDpiForSystem.Call(); d >= 96 {
			dpi = int(d)
		}
	}
	pQPF.Call(uintptr(unsafe.Pointer(&qpcFreq)))
	hInst, _, _ = pGetModuleHandleW.Call(0)

	// only one copy at a time (two copies would fight over the hotkeys)
	if w, _, _ := pFindWindowW.Call(up("PeterAutoClickerWnd"), 0); w != 0 {
		pShowWindow.Call(w, 9)
		pSetForegroundWindow.Call(w)
		return
	}

	var token uintptr
	in := struct {
		Version               uint32
		Callback              uintptr
		SuppressBgThread, Ext int32
	}{Version: 1}
	gpStartup.Call(uintptr(unsafe.Pointer(&token)), uintptr(unsafe.Pointer(&in)), 0)

	loadSettings()
	initAudio()
	loadIcons()
	loadLogo()
	makeSparks()

	fTitle = mkFont(20, 900, "Segoe UI")
	fSub = mkFont(8.5, 600, "Segoe UI")
	fStatus = mkFont(11.5, 700, "Segoe UI")
	fInfo = mkFont(9.5, 400, "Segoe UI")
	fLabel = mkFont(12, 800, "Segoe UI")
	fKey = mkFont(8, 700, "Segoe UI")
	fTile = mkFont(10.5, 700, "Segoe UI")
	fField = mkFont(11.5, 600, "Segoe UI")
	fFoot = mkFont(8.5, 600, "Segoe UI")
	fUnit = mkFont(9.5, 600, "Segoe UI")
	fCap = mkFont(7.5, 700, "Segoe UI")
	fSmall = mkFont(8.5, 600, "Segoe UI")

	var bigIcon, smallIcon uintptr
	if exe, err := os.Executable(); err == nil {
		pExtractIconExW.Call(up(exe), 0, uintptr(unsafe.Pointer(&bigIcon)), uintptr(unsafe.Pointer(&smallIcon)), 1)
	}
	appIconBig, appIconSmall = bigIcon, smallIcon
	cursor, _, _ := pLoadCursorW.Call(0, 32512)
	wc := wndClassEx{
		WndProc:   syscall.NewCallback(wndProc),
		Instance:  hInst,
		Icon:      bigIcon,
		IconSm:    smallIcon,
		Cursor:    cursor,
		ClassName: u16("PeterAutoClickerWnd"),
	}
	wc.Size = uint32(unsafe.Sizeof(wc))
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	style := uint32(wsOverlapped | wsCaption | wsSysMenu | wsMinimizeBox | 0x02000000) // + WS_CLIPCHILDREN
	cr := rect{0, 0, sc(winW), sc(winH)}
	pAdjustWindowRectEx.Call(uintptr(unsafe.Pointer(&cr)), uintptr(style), 0, 0)
	hwndMain, _, _ = pCreateWindowExW.Call(0, up("PeterAutoClickerWnd"), up("Peter Auto Clicker"),
		uintptr(style), 0x80000000, 0x80000000,
		uintptr(cr.R-cr.L), uintptr(cr.B-cr.T), 0, 0, hInst, 0)
	if hwndMain == 0 {
		return
	}
	buildControls()
	registerHotkeys()
	refreshKeyHints()

	if r, err := loadRecording(lastRecPath()); err == nil {
		cur = r
	}
	setBusy(false)
	idleStatus()
	applyTheme()

	pShowWindow.Call(hwndMain, 1)
	pUpdateWindow.Call(hwndMain)

	var m msgT
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		if capturing == 0 { // let the hotkey field see every key while capturing
			if d, _, _ := pIsDialogMessageW.Call(hwndMain, uintptr(unsafe.Pointer(&m))); d != 0 {
				continue
			}
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	stopFlag.Store(true)
}

var pIsWindowEnabled = user32.NewProc("IsWindowEnabled")
