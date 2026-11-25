// gfx.go — GDI+ drawing helpers, theme palette, icons and logo for the "Duel Arena" look.
// Icons: Lucide (ISC licence, see THIRD_PARTY_NOTICES.txt).
package main

import (
	"bytes"
	"embed"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

//go:embed icons/*.png
var iconFS embed.FS

// ───────────────────────────── GDI+ / extra bindings ─────────────────────────────

var (
	gdiplus   = syscall.NewLazyDLL("gdiplus.dll")
	comctl32  = syscall.NewLazyDLL("comctl32.dll")
	gpStartup = gdiplus.NewProc("GdiplusStartup")

	gpCreateFromHDC    = gdiplus.NewProc("GdipCreateFromHDC")
	gpDeleteGraphics   = gdiplus.NewProc("GdipDeleteGraphics")
	gpSetSmoothing     = gdiplus.NewProc("GdipSetSmoothingMode")
	gpSetPixelOffset   = gdiplus.NewProc("GdipSetPixelOffsetMode")
	gpSetInterp        = gdiplus.NewProc("GdipSetInterpolationMode")
	gpCreatePath       = gdiplus.NewProc("GdipCreatePath")
	gpDeletePath       = gdiplus.NewProc("GdipDeletePath")
	gpAddPathArcI      = gdiplus.NewProc("GdipAddPathArcI")
	gpAddPathEllipseI  = gdiplus.NewProc("GdipAddPathEllipseI")
	gpClosePathFigure  = gdiplus.NewProc("GdipClosePathFigure")
	gpFillPath         = gdiplus.NewProc("GdipFillPath")
	gpDrawPath         = gdiplus.NewProc("GdipDrawPath")
	gpCreateSolidFill  = gdiplus.NewProc("GdipCreateSolidFill")
	gpCreateLineBrushI = gdiplus.NewProc("GdipCreateLineBrushI")
	gpSetLinePreset    = gdiplus.NewProc("GdipSetLinePresetBlend")
	gpDeleteBrush      = gdiplus.NewProc("GdipDeleteBrush")
	gpCreatePen1       = gdiplus.NewProc("GdipCreatePen1")
	gpDeletePen        = gdiplus.NewProc("GdipDeletePen")
	gpDrawLineI        = gdiplus.NewProc("GdipDrawLineI")
	gpFillRectI        = gdiplus.NewProc("GdipFillRectangleI")
	gpBitmapFromScan0  = gdiplus.NewProc("GdipCreateBitmapFromScan0")
	gpImageGraphics    = gdiplus.NewProc("GdipGetImageGraphicsContext")
	gpDisposeImage     = gdiplus.NewProc("GdipDisposeImage")
	gpDrawImageRectI   = gdiplus.NewProc("GdipDrawImageRectI")
	gpSetClipPath      = gdiplus.NewProc("GdipSetClipPath")
	gpResetClip        = gdiplus.NewProc("GdipResetClip")
	gpPathGradFromPath = gdiplus.NewProc("GdipCreatePathGradientFromPath")
	gpPathGradCenter   = gdiplus.NewProc("GdipSetPathGradientCenterColor")
	gpPathGradSurround = gdiplus.NewProc("GdipSetPathGradientSurroundColorsWithCount")
	gpTranslate        = gdiplus.NewProc("GdipTranslateWorldTransform")
	gpResetTransform   = gdiplus.NewProc("GdipResetWorldTransform")
	gpAddPathLine2I    = gdiplus.NewProc("GdipAddPathLine2I")
	gpFillEllipseI     = gdiplus.NewProc("GdipFillEllipseI")

	pCreateCompatibleDC  = gdi32.NewProc("CreateCompatibleDC")
	pCreateCompatibleBmp = gdi32.NewProc("CreateCompatibleBitmap")
	pDeleteDC            = gdi32.NewProc("DeleteDC")
	pDeleteObject        = gdi32.NewProc("DeleteObject")
	pBitBlt              = gdi32.NewProc("BitBlt")
	pSetCharExtra        = gdi32.NewProc("SetTextCharacterExtra")
	pGetTextExtent       = gdi32.NewProc("GetTextExtentPoint32W")

	pInitCommonCtrls = comctl32.NewProc("InitCommonControlsEx")
	pSetWindowSubcls = comctl32.NewProc("SetWindowSubclass")
	pDefSubclassProc = comctl32.NewProc("DefSubclassProc")
	pTrackMouseEvent = user32.NewProc("TrackMouseEvent")
)

func f32(v float32) uintptr { return uintptr(math.Float32bits(v)) }
func i32(v int32) uintptr   { return uintptr(v) }

// ARGB colour for GDI+
type argb uint32

func A(a, r, g, b uint8) argb     { return argb(uint32(a)<<24 | uint32(r)<<16 | uint32(g)<<8 | uint32(b)) }
func (c argb) withA(a uint8) argb { return argb(uint32(c)&0x00FFFFFF | uint32(a)<<24) }
func (c argb) ref() uintptr { // → GDI COLORREF
	return rgb(uint8(c>>16), uint8(c>>8), uint8(c))
}
func (c argb) mul(pct int) argb { // brighten/darken RGB by pct (100 = same)
	ch := func(v uint32) uint32 {
		n := int(v) * pct / 100
		if n > 255 {
			n = 255
		}
		return uint32(n)
	}
	u := uint32(c)
	return argb(u&0xFF000000 | ch(u>>16&0xFF)<<16 | ch(u>>8&0xFF)<<8 | ch(u&0xFF))
}

// ───────────────────────────── Theme ─────────────────────────────

type palette struct {
	bg1, bg2, glow, line, head1, head2, card1, card2, border, field, fieldB argb
	text, muted, icon                                                       argb
}

var (
	gold1 = A(255, 246, 214, 125)
	gold2 = A(255, 196, 146, 46)
	navy  = A(255, 20, 28, 56)

	darkPal = palette{
		bg1: A(255, 16, 28, 62), bg2: A(255, 6, 10, 26), glow: A(80, 52, 92, 210), line: A(9, 255, 255, 255),
		head1: A(255, 10, 18, 44), head2: A(255, 18, 32, 72), card1: A(255, 30, 44, 88), card2: A(255, 19, 30, 66),
		border: A(150, 214, 172, 82), field: A(255, 9, 15, 36), fieldB: A(255, 70, 86, 130),
		text: A(255, 243, 233, 204), muted: A(255, 150, 166, 204), icon: A(255, 232, 214, 165),
	}
	lightPal = palette{
		bg1: A(255, 250, 245, 232), bg2: A(255, 234, 223, 196), glow: A(90, 255, 214, 120), line: A(12, 120, 90, 30),
		head1: A(255, 14, 24, 56), head2: A(255, 26, 42, 88), card1: A(255, 255, 253, 246), card2: A(255, 246, 238, 218),
		border: A(170, 184, 134, 43), field: A(255, 255, 255, 255), fieldB: A(255, 205, 190, 150),
		text: A(255, 26, 38, 72), muted: A(255, 107, 115, 147), icon: A(255, 120, 92, 40),
	}
)

func pal() *palette {
	if darkMode {
		return &darkPal
	}
	return &lightPal
}

type box struct{ x, y, w, h int }

func sc(v int) int32  { return int32(v * dpi / 96) }
func (b box) r() rect { return rect{sc(b.x), sc(b.y), sc(b.x + b.w), sc(b.y + b.h)} }
func hit(a, b rect) bool {
	return a.L < b.R && b.L < a.R && a.T < b.B && b.T < a.B
}

// ───────────────────────────── GDI+ painter ─────────────────────────────

type textOp struct {
	s     string
	r     rect
	font  uintptr
	col   argb
	flags uintptr
	extra int32
}

type painter struct {
	g     uintptr
	texts []textOp
}

func newPainter(hdc uintptr) *painter {
	var g uintptr
	gpCreateFromHDC.Call(hdc, uintptr(unsafe.Pointer(&g)))
	gpSetSmoothing.Call(g, 4)   // AntiAlias
	gpSetPixelOffset.Call(g, 4) // HighQuality
	gpSetInterp.Call(g, 7)      // HighQualityBicubic
	return &painter{g: g}
}

// finish releases GDI+ and then draws the queued text with ClearType GDI.
func (p *painter) finish(hdc uintptr) {
	gpDeleteGraphics.Call(p.g)
	pSetBkMode.Call(hdc, 1)
	for _, t := range p.texts {
		old, _, _ := pSelectObject.Call(hdc, t.font)
		pSetTextColor.Call(hdc, t.col.ref())
		pSetCharExtra.Call(hdc, uintptr(t.extra))
		r := t.r
		pDrawTextW.Call(hdc, up(t.s), ^uintptr(0), uintptr(unsafe.Pointer(&r)), t.flags|0x800) // NOPREFIX
		pSetCharExtra.Call(hdc, 0)
		pSelectObject.Call(hdc, old)
	}
}

func (p *painter) text(s string, r rect, font uintptr, col argb, flags uintptr) {
	p.texts = append(p.texts, textOp{s: s, r: r, font: font, col: col, flags: flags})
}

func roundPath(r rect, rad int32) uintptr {
	var path uintptr
	gpCreatePath.Call(0, uintptr(unsafe.Pointer(&path)))
	w, h := r.R-r.L, r.B-r.T
	d := rad * 2
	if d > w {
		d = w
	}
	if d > h {
		d = h
	}
	if d < 1 {
		d = 1
	}
	gpAddPathArcI.Call(path, i32(r.L), i32(r.T), i32(d), i32(d), f32(180), f32(90))
	gpAddPathArcI.Call(path, i32(r.R-d), i32(r.T), i32(d), i32(d), f32(270), f32(90))
	gpAddPathArcI.Call(path, i32(r.R-d), i32(r.B-d), i32(d), i32(d), f32(0), f32(90))
	gpAddPathArcI.Call(path, i32(r.L), i32(r.B-d), i32(d), i32(d), f32(90), f32(90))
	gpClosePathFigure.Call(path)
	return path
}

func (p *painter) fillRound(r rect, rad int32, c1, c2 argb) {
	path := roundPath(r, rad)
	defer gpDeletePath.Call(path)
	var b uintptr
	if c1 == c2 {
		gpCreateSolidFill.Call(uintptr(c1), uintptr(unsafe.Pointer(&b)))
	} else {
		p1, p2 := point{0, r.T}, point{0, r.B + 1}
		gpCreateLineBrushI.Call(uintptr(unsafe.Pointer(&p1)), uintptr(unsafe.Pointer(&p2)), uintptr(c1), uintptr(c2), 3, uintptr(unsafe.Pointer(&b)))
	}
	gpFillPath.Call(p.g, b, path)
	gpDeleteBrush.Call(b)
}

func (p *painter) strokeRound(r rect, rad int32, c argb, width float32) {
	path := roundPath(r, rad)
	defer gpDeletePath.Call(path)
	var pen uintptr
	gpCreatePen1.Call(uintptr(c), f32(width), 2, uintptr(unsafe.Pointer(&pen)))
	gpDrawPath.Call(p.g, pen, path)
	gpDeletePen.Call(pen)
}

// glow draws a soft halo around r by stacking faint, growing outlines.
func (p *painter) glow(r rect, rad int32, c argb, layers int) {
	for i := layers; i >= 1; i-- {
		e := int32(i)
		a := uint8(int(c>>24) * (layers - i + 1) / (layers + 1) / 2)
		p.fillRound(rect{r.L - e, r.T - e, r.R + e, r.B + e}, rad+e, c.withA(a), c.withA(a))
	}
}

func (p *painter) card(r rect) {
	pl := pal()
	p.fillRound(r, sc(12), pl.card1, pl.card2)
	p.strokeRound(r, sc(12), pl.border, float32(sc(3))/2)
}

func (p *painter) icon(name string, r rect, c argb) {
	if bmp := iconBitmap(name, c); bmp != 0 {
		gpDrawImageRectI.Call(p.g, bmp, i32(r.L), i32(r.T), i32(r.R-r.L), i32(r.B-r.T))
	}
}

func iconRect(cx, cy, size int32) rect {
	return rect{cx - size/2, cy - size/2, cx - size/2 + size, cy - size/2 + size}
}

// ───────────────────────────── Images ─────────────────────────────

var (
	iconMasks = map[string]*image.NRGBA{}
	iconCache = map[string]uintptr{}
	iconPix   [][]byte // keep pixel buffers alive for GDI+
	logoBmp   uintptr
)

func loadIcons() {
	ents, _ := iconFS.ReadDir("icons")
	for _, e := range ents {
		b, err := iconFS.ReadFile("icons/" + e.Name())
		if err != nil {
			continue
		}
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			continue
		}
		n := image.NewNRGBA(img.Bounds())
		draw.Draw(n, n.Bounds(), img, img.Bounds().Min, draw.Src)
		iconMasks[strings.TrimSuffix(e.Name(), ".png")] = n
	}
}

func iconBitmap(name string, c argb) uintptr {
	key := name + "|" + strconv.FormatUint(uint64(c), 16)
	if b, ok := iconCache[key]; ok {
		return b
	}
	m := iconMasks[name]
	if m == nil {
		return 0
	}
	w, h := m.Bounds().Dx(), m.Bounds().Dy()
	pix := make([]byte, w*h*4)
	cr, cg, cb, ca := uint32(c>>16&0xFF), uint32(c>>8&0xFF), uint32(c&0xFF), uint32(c>>24)
	for i := 0; i < w*h; i++ {
		a := uint32(m.Pix[i*4+3]) * ca / 255 // premultiplied BGRA
		pix[i*4+0] = byte(cb * a / 255)
		pix[i*4+1] = byte(cg * a / 255)
		pix[i*4+2] = byte(cr * a / 255)
		pix[i*4+3] = byte(a)
	}
	iconPix = append(iconPix, pix)
	var bmp uintptr
	gpBitmapFromScan0.Call(uintptr(w), uintptr(h), uintptr(w*4), 0xE200B, uintptr(unsafe.Pointer(&pix[0])), uintptr(unsafe.Pointer(&bmp)))
	iconCache[key] = bmp
	return bmp
}

func loadLogo() {
	img, err := jpeg.Decode(bytes.NewReader(logoJPG))
	if err != nil {
		return
	}
	b := img.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
	pix := make([]byte, len(rgba.Pix))
	for i := 0; i < len(pix); i += 4 {
		pix[i], pix[i+1], pix[i+2], pix[i+3] = rgba.Pix[i+2], rgba.Pix[i+1], rgba.Pix[i], 255
	}
	var src uintptr
	gpBitmapFromScan0.Call(uintptr(b.Dx()), uintptr(b.Dy()), uintptr(b.Dx()*4), 0xE200B, uintptr(unsafe.Pointer(&pix[0])), uintptr(unsafe.Pointer(&src)))
	// pre-scale once to its on-screen size with high-quality filtering
	size := sc(logoInner)
	gpBitmapFromScan0.Call(uintptr(size), uintptr(size), 0, 0xE200B, 0, uintptr(unsafe.Pointer(&logoBmp)))
	var g uintptr
	gpImageGraphics.Call(logoBmp, uintptr(unsafe.Pointer(&g)))
	gpSetInterp.Call(g, 7)
	gpSetPixelOffset.Call(g, 4)
	gpDrawImageRectI.Call(g, src, 0, 0, i32(size), i32(size))
	gpDeleteGraphics.Call(g)
	gpDisposeImage.Call(src)
}
