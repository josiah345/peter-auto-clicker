// Peter Auto Clicker — records the exact path, pace and clicks of your mouse
// and replays them once, N times, or forever.
//
// Pure Win32 (no dependencies). Build:
//
//	GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui -s -w"
package main

import (
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

//go:embed logo.jpg
var logoJPG []byte

func init() { runtime.LockOSThread() } // GUI + hook live on the main OS thread

// ───────────────────────────── Win32 bindings ─────────────────────────────

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	winmm    = syscall.NewLazyDLL("winmm.dll")
	comdlg32 = syscall.NewLazyDLL("comdlg32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	uxtheme  = syscall.NewLazyDLL("uxtheme.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")

	pSetWindowTheme      = uxtheme.NewProc("SetWindowTheme")
	pGetAsyncKeyState    = user32.NewProc("GetAsyncKeyState")
	pFindWindowW         = user32.NewProc("FindWindowW")
	pSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	pCreateWaitableTimer = kernel32.NewProc("CreateWaitableTimerExW")
	pSetWaitableTimer    = kernel32.NewProc("SetWaitableTimer")
	pWaitForSingleObj    = kernel32.NewProc("WaitForSingleObject")
	pCloseHandle         = kernel32.NewProc("CloseHandle")
	pDwmSetAttr          = dwmapi.NewProc("DwmSetWindowAttribute")
	pRedrawWindow        = user32.NewProc("RedrawWindow")
	pGetClientRect       = user32.NewProc("GetClientRect")
	pSetBkColor          = gdi32.NewProc("SetBkColor")

	pRegisterClassExW    = user32.NewProc("RegisterClassExW")
	pCreateWindowExW     = user32.NewProc("CreateWindowExW")
	pDefWindowProcW      = user32.NewProc("DefWindowProcW")
	pGetMessageW         = user32.NewProc("GetMessageW")
	pTranslateMessage    = user32.NewProc("TranslateMessage")
	pDispatchMessageW    = user32.NewProc("DispatchMessageW")
	pIsDialogMessageW    = user32.NewProc("IsDialogMessageW")
	pPostQuitMessage     = user32.NewProc("PostQuitMessage")
	pPostMessageW        = user32.NewProc("PostMessageW")
	pSendMessageW        = user32.NewProc("SendMessageW")
	pShowWindow          = user32.NewProc("ShowWindow")
	pUpdateWindow        = user32.NewProc("UpdateWindow")
	pDestroyWindow       = user32.NewProc("DestroyWindow")
	pLoadCursorW         = user32.NewProc("LoadCursorW")
	pSetWindowTextW      = user32.NewProc("SetWindowTextW")
	pGetWindowTextW      = user32.NewProc("GetWindowTextW")
	pEnableWindow        = user32.NewProc("EnableWindow")
	pSetWindowsHookExW   = user32.NewProc("SetWindowsHookExW")
	pUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	pCallNextHookEx      = user32.NewProc("CallNextHookEx")
	pRegisterHotKey      = user32.NewProc("RegisterHotKey")
	pUnregisterHotKey    = user32.NewProc("UnregisterHotKey")
	pSendInput           = user32.NewProc("SendInput")
	pSetCursorPos        = user32.NewProc("SetCursorPos")
	pGetCursorPos        = user32.NewProc("GetCursorPos")
	pGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	pBeginPaint          = user32.NewProc("BeginPaint")
	pEndPaint            = user32.NewProc("EndPaint")
	pFillRect            = user32.NewProc("FillRect")
	pDrawTextW           = user32.NewProc("DrawTextW")
	pInvalidateRect      = user32.NewProc("InvalidateRect")
	pGetWindowRect       = user32.NewProc("GetWindowRect")
	pAdjustWindowRectEx  = user32.NewProc("AdjustWindowRectEx")
	pSetWindowPos        = user32.NewProc("SetWindowPos")
	pMessageBoxW         = user32.NewProc("MessageBoxW")
	pGetDpiForSystem     = user32.NewProc("GetDpiForSystem")
	pSetProcessDPIAware  = user32.NewProc("SetProcessDPIAware")
	pSetDpiAwarenessCtx  = user32.NewProc("SetProcessDpiAwarenessContext")
	pIsIconic            = user32.NewProc("IsIconic")

	pGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	pQPC              = kernel32.NewProc("QueryPerformanceCounter")
	pQPF              = kernel32.NewProc("QueryPerformanceFrequency")
	pSleep            = kernel32.NewProc("Sleep")
	pSetThreadPrio    = kernel32.NewProc("SetThreadPriority")
	pGetCurrentThread = kernel32.NewProc("GetCurrentThread")

	pCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	pCreateFontW      = gdi32.NewProc("CreateFontW")
	pSelectObject     = gdi32.NewProc("SelectObject")
	pSetTextColor     = gdi32.NewProc("SetTextColor")
	pSetBkMode        = gdi32.NewProc("SetBkMode")
	pStretchDIBits    = gdi32.NewProc("StretchDIBits")
	pSetStretchBlt    = gdi32.NewProc("SetStretchBltMode")
	pSetBrushOrgEx    = gdi32.NewProc("SetBrushOrgEx")

	pTimeBeginPeriod = winmm.NewProc("timeBeginPeriod")
	pTimeEndPeriod   = winmm.NewProc("timeEndPeriod")

	pGetOpenFileNameW = comdlg32.NewProc("GetOpenFileNameW")
	pGetSaveFileNameW = comdlg32.NewProc("GetSaveFileNameW")

	pExtractIconExW = shell32.NewProc("ExtractIconExW")
)

const (
	wsOverlapped  = 0x00000000
	wsCaption     = 0x00C00000
	wsSysMenu     = 0x00080000
	wsMinimizeBox = 0x00020000
	wsVisible     = 0x10000000
	wsChild       = 0x40000000
	wsTabStop     = 0x00010000
	wsGroup       = 0x00020000
	wsExClient    = 0x00000200

	bsPushButton = 0
	bsAutoCheck  = 3
	bsRadio      = 4
	esNumber     = 0x2000
	esAutoH      = 0x0080
	esCenter     = 0x0001
	ssNoPrefix   = 0x80

	wmDestroy        = 0x0002
	wmPaint          = 0x000F
	wmClose          = 0x0010
	wmSetFont        = 0x0030
	wmSetIcon        = 0x0080
	wmCommand        = 0x0111
	wmHotkey         = 0x0312
	wmCtlColorEdit   = 0x0133
	wmCtlColorBtn    = 0x0135
	wmCtlColorStatic = 0x0138
	wmApp            = 0x8000
	wmAppLoop        = wmApp + 1
	wmAppDone        = wmApp + 2
	wmAppStopPlay    = wmApp + 3

	bmGetCheck = 0x00F0
	bmSetCheck = 0x00F1

	whMouseLL      = 14
	llmhfInjected  = 0x01
	llmhfLowInject = 0x02

	wmMouseMove   = 0x0200
	wmLButtonDown = 0x0201
	wmLButtonUp   = 0x0202
	wmRButtonDown = 0x0204
	wmRButtonUp   = 0x0205
	wmMButtonDown = 0x0207
	wmMButtonUp   = 0x0208
	wmMouseWheel  = 0x020A
	wmXButtonDown = 0x020B
	wmXButtonUp   = 0x020C
	wmMouseHWheel = 0x020E

	mefMove       = 0x0001
	mefLeftDown   = 0x0002
	mefLeftUp     = 0x0004
	mefRightDown  = 0x0008
	mefRightUp    = 0x0010
	mefMiddleDown = 0x0020
	mefMiddleUp   = 0x0040
	mefXDown      = 0x0080
	mefXUp        = 0x0100
	mefWheel      = 0x0800
	mefHWheel     = 0x1000
	mefVirtDesk   = 0x4000
	mefAbsolute   = 0x8000

	vkF6 = 0x75
	vkF7 = 0x76
	vkF8 = 0x77
)

type point struct{ X, Y int32 }
type rect struct{ L, T, R, B int32 }

type msgT struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
	Private uint32
}

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type paintStruct struct {
	Hdc      uintptr
	Erase    int32
	RcPaint  rect
	Restore  int32
	IncUpd   int32
	Reserved [32]byte
}

type msllHook struct {
	Pt        point
	MouseData uint32
	Flags     uint32
	Time      uint32
	Extra     uintptr
}

type mouseInput struct {
	Type      uint32
	_         uint32
	Dx, Dy    int32
	MouseData uint32
	Flags     uint32
	Time      uint32
	Extra     uintptr
}

type bmiHeader struct {
	Size          uint32
	Width, Height int32
	Planes, Bits  uint16
	Compression   uint32
	SizeImage     uint32
	XPels, YPels  int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type openFileName struct {
	StructSize    uint32
	Owner         uintptr
	Instance      uintptr
	Filter        *uint16
	CustomFilter  *uint16
	MaxCustFilter uint32
	FilterIndex   uint32
	File          *uint16
	MaxFile       uint32
	FileTitle     *uint16
	MaxFileTitle  uint32
	InitialDir    *uint16
	Title         *uint16
	Flags         uint32
	FileOffset    uint16
	FileExtension uint16
	DefExt        *uint16
	CustData      uintptr
	Hook          uintptr
	TemplateName  *uint16
	Reserved      uintptr
	Reserved2     uint32
	FlagsEx       uint32
}

func u16(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }
func up(s string) uintptr  { return uintptr(unsafe.Pointer(u16(s))) }
func rgb(r, g, b byte) uintptr {
	return uintptr(r) | uintptr(g)<<8 | uintptr(b)<<16
}
func loword(v uintptr) uint16 { return uint16(v & 0xFFFF) }
func hiword(v uintptr) uint16 { return uint16((v >> 16) & 0xFFFF) }

// ───────────────────────────── Timing ─────────────────────────────

var qpcFreq int64

func qpc() int64 { var v int64; pQPC.Call(uintptr(unsafe.Pointer(&v))); return v }

// microseconds elapsed since base (overflow-safe)
func usSince(base int64) int64 {
	d := qpc() - base
	return d/qpcFreq*1_000_000 + (d%qpcFreq)*1_000_000/qpcFreq
}

// ───────────────────────────── Recording model ─────────────────────────────

const (
	kMove uint8 = iota
	kLDown
	kLUp
	kRDown
	kRUp
	kMDown
	kMUp
	kWheel
	kHWheel
	kXDown
	kXUp
)

type Event struct {
	T    int64 // µs since recording start
	K    uint8
	X, Y int32
	D    int32 // wheel delta or X-button number
}

type Recording struct {
	StartX, StartY int32
	Dur            int64 // µs, total length incl. trailing idle
	Ev             []Event
}

func (r *Recording) summary() string {
	if r == nil || len(r.Ev) == 0 {
		return "No recording yet"
	}
	clicks := 0
	for _, e := range r.Ev {
		switch e.K {
		case kLDown, kRDown, kMDown, kXDown:
			clicks++
		}
	}
	return fmt.Sprintf("⏱ %s s      🖱 %d clicks",
		strconv.FormatFloat(float64(r.Dur)/1e6, 'f', 1, 64), clicks)
}

func (r *Recording) save(path string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "PAC1\nstart %d %d\ndur %d\n", r.StartX, r.StartY, r.Dur)
	for _, e := range r.Ev {
		fmt.Fprintf(&b, "%d %d %d %d %d\n", e.T, e.K, e.X, e.Y, e.D)
	}
	return os.WriteFile(path, []byte(b.String()), 0644)
}

func loadRecording(path string) (*Recording, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<16), 1<<20)
	r := &Recording{}
	line := 0
	for sc.Scan() {
		line++
		t := strings.Fields(sc.Text())
		if line == 1 {
			if len(t) != 1 || t[0] != "PAC1" {
				return nil, fmt.Errorf("not a Peter Auto Clicker recording")
			}
			continue
		}
		if len(t) == 0 {
			continue
		}
		switch t[0] {
		case "start":
			if len(t) == 3 {
				x, _ := strconv.Atoi(t[1])
				y, _ := strconv.Atoi(t[2])
				r.StartX, r.StartY = int32(x), int32(y)
			}
		case "dur":
			if len(t) == 2 {
				r.Dur, _ = strconv.ParseInt(t[1], 10, 64)
			}
		default:
			if len(t) != 5 {
				continue
			}
			var v [5]int64
			for i := range v {
				v[i], _ = strconv.ParseInt(t[i], 10, 64)
			}
			r.Ev = append(r.Ev, Event{T: v[0], K: uint8(v[1]), X: int32(v[2]), Y: int32(v[3]), D: int32(v[4])})
		}
	}
	if len(r.Ev) == 0 {
		return nil, fmt.Errorf("the file has no mouse events")
	}
	if last := r.Ev[len(r.Ev)-1].T; r.Dur < last {
		r.Dur = last
	}
	return r, sc.Err()
}

// ───────────────────────────── App state ─────────────────────────────

const (
	stIdle = iota
	stRecording
	stPlaying
)

var (
	hInst    uintptr
	hwndMain uintptr
	dpi      = 96
	state    = stIdle

	hookHandle uintptr
	hookCB     = syscall.NewCallback(hookProc)

	cur      *Recording // current recording (UI thread only)
	recBuf   []Event
	recBase  int64
	recStart point

	stopFlag   atomic.Bool
	expectPos  atomic.Uint64 // last position playback put the cursor at
	userMoved  atomic.Int64  // accumulated real-hand movement during playback
	stopOnMove bool
	playTotal  int // loops (0 = infinite)

	smartMode = false
	logoPix   []byte
	logoW     int32
	logoH     int32
	headerCol uintptr

	bgBrush, headBrush uintptr
	darkBg, darkEdit   uintptr
	editBrush          uintptr
	fontUI, fontIcon   uintptr
	fontBtnBig, fontSm uintptr
	fontTitle, fontSt  uintptr

	statusKind = 0
	darkMode   = false
	hotkeysOK  = true
)

// control handles
var (
	hStatus, hInfo               uintptr
	hRecord, hPlay, hStop        uintptr
	hEditCount, hInf             uintptr
	hEditGap, hEditSpeed         uintptr
	hPrecise, hSmart             uintptr
	hStopMove, hMin, hTop, hDark uintptr
	hSave, hLoad, hClear         uintptr
	allInputs, editLike          []uintptr
	toggled                      = map[uintptr]bool{}
	accent                       = map[uintptr]int{} // big-button colour role
	hTip                         uintptr
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
	idStopMove
	idMin
	idTop
	idDark
	idSave
	idLoad
	idClear
)

func sc(v int) int32 { return int32(v * dpi / 96) }

// dataDir is %APPDATA%\PeterAutoClicker — always writable, even if the .exe
// sits in Program Files, a read-only folder or inside a zip.
func dataDir() string {
	if d, err := os.UserConfigDir(); err == nil {
		dir := filepath.Join(d, "PeterAutoClicker")
		if os.MkdirAll(dir, 0755) == nil {
			return dir
		}
	}
	return os.TempDir()
}

// oldPath is where v1 kept its files (next to the .exe), for one-time migration.
func oldPath(name string) string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), name)
}

func lastRecPath() string { return filepath.Join(dataDir(), "last_recording.pacr") }

// ───────────────────────────── Low-level mouse hook ─────────────────────────────

func hookProc(nCode, wParam, lParam uintptr) uintptr {
	if int32(nCode) >= 0 {
		m := (*msllHook)(unsafe.Pointer(lParam))
		injected := m.Flags&(llmhfInjected|llmhfLowInject) != 0
		switch state {
		case stRecording:
			if !injected {
				recordEvent(uint32(wParam), m)
			}
		case stPlaying:
			// All playback input is sent with SendInput, so it is flagged as
			// injected. Anything NOT injected is your real hand on the mouse.
			if stopOnMove && !injected && uint32(wParam) == wmMouseMove {
				if userMoved.Add(1) > moveStopEvents && !stopFlag.Load() {
					stopFlag.Store(true)
					pPostMessageW.Call(hwndMain, wmAppStopPlay, 0, 0)
				}
			}
		}
	}
	r, _, _ := pCallNextHookEx.Call(0, nCode, wParam, lParam)
	return r
}

const moveStopEvents = 12 // real mouse-move events needed to count as "you moved it"

// Buttons that were already held when recording started: their release is
// dropped so playback never sends an "up" without a matching "down".
var preHeld = map[uint8]bool{}

func recordEvent(msg uint32, m *msllHook) {
	e := Event{T: usSince(recBase), X: m.Pt.X, Y: m.Pt.Y}
	switch msg {
	case wmMouseMove:
		e.K = kMove
	case wmLButtonDown:
		e.K = kLDown
	case wmLButtonUp:
		e.K = kLUp
	case wmRButtonDown:
		e.K = kRDown
	case wmRButtonUp:
		e.K = kRUp
	case wmMButtonDown:
		e.K = kMDown
	case wmMButtonUp:
		e.K = kMUp
	case wmMouseWheel:
		e.K, e.D = kWheel, int32(int16(m.MouseData>>16))
	case wmMouseHWheel:
		e.K, e.D = kHWheel, int32(int16(m.MouseData>>16))
	case wmXButtonDown:
		e.K, e.D = kXDown, int32(m.MouseData>>16)
	case wmXButtonUp:
		e.K, e.D = kXUp, int32(m.MouseData>>16)
	default:
		return
	}
	upKey := e.K
	if e.K == kXUp {
		upKey = uint8(100 + e.D)
	}
	if preHeld[upKey] {
		delete(preHeld, upKey)
		return
	}
	recBuf = append(recBuf, e)
}

func installHook() {
	if hookHandle == 0 {
		mod, _, _ := pGetModuleHandleW.Call(0)
		hookHandle, _, _ = pSetWindowsHookExW.Call(whMouseLL, hookCB, mod, 0)
	}
}

func removeHook() {
	if hookHandle != 0 {
		pUnhookWindowsHookEx.Call(hookHandle)
		hookHandle = 0
	}
}

// ───────────────────────────── Playback ─────────────────────────────

type player struct {
	smart          bool
	vx, vy, vw, vh int64
	held           map[uint8]Event // buttons currently held down
}

func (p *player) norm(x, y int32) (int32, int32) {
	nx := (int64(x)-p.vx)*65536/p.vw + 1
	ny := (int64(y)-p.vy)*65536/p.vh + 1
	if (int64(x)-p.vx)*65536%p.vw == 0 {
		nx--
	}
	if (int64(y)-p.vy)*65536%p.vh == 0 {
		ny--
	}
	clamp := func(v int64) int32 {
		if v < 0 {
			return 0
		}
		if v > 65535 {
			return 65535
		}
		return int32(v)
	}
	return clamp(nx), clamp(ny)
}

func sendMouse(flags uint32, dx, dy int32, data uint32) {
	in := mouseInput{Type: 0, Dx: dx, Dy: dy, MouseData: data, Flags: flags}
	pSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
}

func (p *player) moveTo(x, y int32) {
	nx, ny := p.norm(x, y)
	sendMouse(mefMove|mefAbsolute|mefVirtDesk, nx, ny, 0)
}

func btnFlags(k uint8) uint32 {
	switch k {
	case kLDown:
		return mefLeftDown
	case kLUp:
		return mefLeftUp
	case kRDown:
		return mefRightDown
	case kRUp:
		return mefRightUp
	case kMDown:
		return mefMiddleDown
	case kMUp:
		return mefMiddleUp
	case kXDown:
		return mefXDown
	case kXUp:
		return mefXUp
	case kWheel:
		return mefWheel
	case kHWheel:
		return mefHWheel
	}
	return 0
}

func (p *player) do(e Event) {
	if e.K == kMove {
		p.moveTo(e.X, e.Y)
		return
	}
	// Clicks / wheel always land on the exact recorded pixel.
	var data uint32
	switch e.K {
	case kWheel, kHWheel, kXDown, kXUp:
		data = uint32(e.D)
	}
	nx, ny := p.norm(e.X, e.Y)
	sendMouse(mefMove|mefAbsolute|mefVirtDesk|btnFlags(e.K), nx, ny, data)
	switch e.K {
	case kLDown, kRDown, kMDown:
		p.held[e.K] = e
	case kXDown:
		p.held[uint8(100+e.D)] = e
	case kLUp:
		delete(p.held, kLDown)
	case kRUp:
		delete(p.held, kRDown)
	case kMUp:
		delete(p.held, kMDown)
	case kXUp:
		delete(p.held, uint8(100+e.D))
	}
}

func (p *player) releaseAll() {
	for k, e := range p.held {
		switch {
		case k == kLDown:
			sendMouse(mefLeftUp, 0, 0, 0)
		case k == kRDown:
			sendMouse(mefRightUp, 0, 0, 0)
		case k == kMDown:
			sendMouse(mefMiddleUp, 0, 0, 0)
		case k >= 100:
			sendMouse(mefXUp, 0, 0, uint32(e.D))
		}
	}
	p.held = map[uint8]Event{}
}

// waitUntil blocks until `target` µs after base. Precise mode sleeps coarsely
// then spins for sub-millisecond accuracy; smart mode only sleeps (low CPU).
// Returns false if stopped.
var hiResTimer uintptr // high-resolution waitable timer (Windows 10 1803+), 0 if unavailable

func sleepUS(us int64) {
	if hiResTimer != 0 {
		due := -us * 10 // relative, in 100 ns units
		pSetWaitableTimer.Call(hiResTimer, uintptr(unsafe.Pointer(&due)), 0, 0, 0, 0)
		pWaitForSingleObj.Call(hiResTimer, 0xFFFFFFFF)
		return
	}
	ms := us / 1000
	pSleep.Call(uintptr(ms))
}

func waitUntil(base, target int64, spin bool) bool {
	for {
		if stopFlag.Load() {
			return false
		}
		rem := target - usSince(base)
		if rem <= 0 {
			return true
		}
		if spin {
			if hiResTimer != 0 && rem > 600 {
				// sleep until ~0.4 ms before the target, then spin the rest
				d := rem - 400
				if d > 10_000 {
					d = 10_000
				}
				sleepUS(d)
			} else if hiResTimer == 0 && rem > 2500 {
				ms := (rem - 2000) / 1000
				if ms > 10 {
					ms = 10
				}
				pSleep.Call(uintptr(ms))
			} else if rem > 300 {
				pSleep.Call(0)
			} // else: busy-spin the final few hundred µs
		} else {
			ms := rem / 1000
			if ms < 1 {
				ms = 1
			}
			if ms > 10 {
				ms = 10
			}
			pSleep.Call(uintptr(ms))
		}
	}
}

var playDone chan struct{}

func playLoop(r *Recording, loops int, gapMs int, speed float64, smart bool, done chan struct{}) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	th, _, _ := pGetCurrentThread.Call()
	pSetThreadPrio.Call(th, 2) // THREAD_PRIORITY_HIGHEST
	pTimeBeginPeriod.Call(1)   // fine timer only while playing (saves battery when idle)
	defer pTimeEndPeriod.Call(1)
	if pCreateWaitableTimer.Find() == nil {
		hiResTimer, _, _ = pCreateWaitableTimer.Call(0, 0, 0x2, 0x1F0003) // HIGH_RESOLUTION
		if hiResTimer != 0 {
			defer func() { pCloseHandle.Call(hiResTimer); hiResTimer = 0 }()
		}
	}

	p := &player{smart: smart, held: map[uint8]Event{}}
	gx := func(i uintptr) int64 { v, _, _ := pGetSystemMetrics.Call(i); return int64(int32(v)) }
	p.vx, p.vy, p.vw, p.vh = gx(76), gx(77), gx(78), gx(79)
	if p.vw <= 0 || p.vh <= 0 {
		p.vw, p.vh = gx(0), gx(1)
	}
	defer func() {
		p.releaseAll()
		pPostMessageW.Call(hwndMain, wmAppDone, 0, 0)
		close(done)
	}()

	scale := func(t int64) int64 { return int64(float64(t) / speed) }
	n := len(r.Ev)
	for loop := 1; loops == 0 || loop <= loops; loop++ {
		if stopFlag.Load() {
			return
		}
		pPostMessageW.Call(hwndMain, wmAppLoop, uintptr(loop), 0)
		p.moveTo(r.StartX, r.StartY)
		base := qpc()
		var shift int64 // smart mode: timeline re-sync offset
		for i := 0; i < n; i++ {
			e := r.Ev[i]
			target := scale(e.T) + shift
			if smart {
				now := usSince(base)
				if e.K == kMove {
					// Running late? Skip stale movement points — jump to the newest
					// one that is already due. Clicks are never skipped.
					if i+1 < n && r.Ev[i+1].K == kMove && scale(r.Ev[i+1].T)+shift <= now {
						continue
					}
				} else if lag := now - target; lag > 120_000 {
					// Fell far behind (PC hiccup): re-sync the timeline so the
					// rhythm between clicks stays the same instead of a burst.
					shift += lag
					target += lag
				}
			}
			if !waitUntil(base, target, !smart) {
				return
			}
			p.do(e)
		}
		if !waitUntil(base, scale(r.Dur)+shift, !smart) {
			return
		}
		p.releaseAll() // never carry a held button into the next loop
		if (loops == 0 || loop < loops) && gapMs > 0 {
			if !waitUntil(qpc(), int64(gapMs)*1000, !smart) {
				return
			}
		}
	}
}

// ───────────────────────────── UI helpers ─────────────────────────────

var (
	comctl32         = syscall.NewLazyDLL("comctl32.dll")
	pInitCommonCtrls = comctl32.NewProc("InitCommonControlsEx")
	pRoundRect       = gdi32.NewProc("RoundRect")
	pCreatePen       = gdi32.NewProc("CreatePen")
	pDeleteObject    = gdi32.NewProc("DeleteObject")
	tipTexts         [][]uint16
)

type drawItem struct {
	CtlType, CtlID, ItemID, ItemAction, ItemState uint32
	Hwnd                                          uintptr
	Hdc                                           uintptr
	Rc                                            rect
	ItemData                                      uintptr
}

type toolInfo struct {
	Size     uint32
	Flags    uint32
	Hwnd     uintptr
	ID       uintptr
	Rc       rect
	Inst     uintptr
	Text     *uint16
	LParam   uintptr
	Reserved uintptr
}

const (
	roleNormal = iota
	roleRec
	rolePlay
	roleStop
)

func setText(h uintptr, s string) { pSetWindowTextW.Call(h, up(s)); pInvalidateRect.Call(h, 0, 1) }
func getText(h uintptr) string {
	buf := make([]uint16, 256)
	pGetWindowTextW.Call(h, uintptr(unsafe.Pointer(&buf[0])), 256)
	return syscall.UTF16ToString(buf)
}
func isChecked(h uintptr) bool { return toggled[h] }
func setCheck(h uintptr, on bool) {
	toggled[h] = on
	pInvalidateRect.Call(h, 0, 1)
}
func enable(h uintptr, on bool) {
	v := uintptr(0)
	if on {
		v = 1
	}
	pEnableWindow.Call(h, v)
}

func mkCtl(class, text string, style uint32, ex uint32, x, y, w, h int, id int, font uintptr) uintptr {
	hw, _, _ := pCreateWindowExW.Call(uintptr(ex), up(class), up(text),
		uintptr(wsChild|wsVisible|style),
		uintptr(sc(x)), uintptr(sc(y)), uintptr(sc(w)), uintptr(sc(h)),
		hwndMain, uintptr(id), hInst, 0)
	pSendMessageW.Call(hw, wmSetFont, font, 1)
	return hw
}

// Owner-drawn button (we paint it ourselves so it looks the same in light and dark).
var btnFont = map[uintptr]uintptr{}

func mkBtn(text string, x, y, w, h, id int, font uintptr, role int, tip string) uintptr {
	hw := mkCtl("BUTTON", text, wsTabStop|0xB, 0, x, y, w, h, id, font) // BS_OWNERDRAW
	btnFont[hw] = font
	accent[hw] = role
	addTip(hw, tip)
	return hw
}

func mkIcon(text string, x, y, w, h int, tip string) uintptr {
	hw := mkCtl("STATIC", text, ssNoPrefix|0x1|0x100|0x200, 0, x, y, w, h, 0, fontIcon) // CENTER|NOTIFY|CENTERIMAGE
	addTip(hw, tip)
	inCard[hw] = true
	return hw
}

var cards []rect
var inCard = map[uintptr]bool{}

func addCard(x, y, w, h int) { cards = append(cards, rect{sc(x), sc(y), sc(x + w), sc(y + h)}) }

func mkUnit(text string, x, y, w, h int) uintptr {
	hw := mkCtl("STATIC", text, ssNoPrefix|0x200, 0, x, y, w, h, 0, fontSm)
	inCard[hw] = true
	return hw
}

func cardFill() uintptr {
	if darkMode {
		return rgb(42, 48, 64)
	}
	return rgb(255, 255, 255)
}

func mkEdit(text string, x, y, w, h, id int, extra uint32, tip string) uintptr {
	hw := mkCtl("EDIT", text, wsTabStop|esAutoH|esCenter|extra, 0, x, y, w, h, id, fontUI)
	addTip(hw, tip)
	inCard[hw] = true
	editLike = append(editLike, hw)
	return hw
}

func addTip(ctl uintptr, text string) {
	if hTip == 0 || text == "" {
		return
	}
	t := syscall.StringToUTF16(text)
	tipTexts = append(tipTexts, t)
	ti := toolInfo{Flags: 0x1 | 0x10, Hwnd: hwndMain, ID: ctl, Text: &t[0]} // IDISHWND|SUBCLASS
	ti.Size = uint32(unsafe.Sizeof(ti))
	pSendMessageW.Call(hTip, 0x432, 0, uintptr(unsafe.Pointer(&ti))) // TTM_ADDTOOLW
}

func mkFont(pt int, weight int, face string) uintptr {
	h := -(pt * dpi / 72)
	f, _, _ := pCreateFontW.Call(uintptr(h), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, up(face))
	return f
}

func msgBox(text string, flags uintptr) {
	pMessageBoxW.Call(hwndMain, up(text), up("Peter Auto Clicker"), flags)
}

const (
	cNeutral = iota
	cRed
	cGreen
	cBlue
	cOrange
)

func textCol() uintptr {
	if darkMode {
		return rgb(232, 236, 244)
	}
	return rgb(20, 33, 61)
}

func statusCol() uintptr {
	if darkMode {
		return [...]uintptr{rgb(232, 236, 244), rgb(255, 110, 110), rgb(110, 220, 140), rgb(120, 180, 255), rgb(255, 190, 90)}[statusKind]
	}
	return [...]uintptr{rgb(20, 33, 61), rgb(200, 30, 30), rgb(20, 130, 60), rgb(20, 90, 180), rgb(200, 110, 0)}[statusKind]
}

func curBg() uintptr {
	if darkMode {
		return darkBg
	}
	return bgBrush
}

func shade(c uintptr, pct uintptr) uintptr {
	r, g, b := c&0xFF, (c>>8)&0xFF, (c>>16)&0xFF
	return r*pct/100 | (g*pct/100)<<8 | (b*pct/100)<<16
}

func drawButton(d *drawItem) {
	h, hdc, r := d.Hwnd, d.Hdc, d.Rc
	pressed := d.ItemState&0x1 != 0
	disabled := d.ItemState&0x4 != 0

	fill, text, border := rgb(255, 255, 255), rgb(20, 33, 61), rgb(214, 203, 170)
	if darkMode {
		fill, text, border = rgb(42, 48, 64), rgb(235, 238, 245), rgb(72, 80, 102)
	}
	switch accent[h] {
	case roleRec:
		fill, text, border = rgb(214, 58, 58), rgb(255, 255, 255), rgb(214, 58, 58)
	case rolePlay:
		fill, text, border = rgb(34, 150, 86), rgb(255, 255, 255), rgb(34, 150, 86)
	case roleStop:
		fill, text, border = rgb(70, 82, 108), rgb(255, 255, 255), rgb(70, 82, 108)
	}
	if toggled[h] {
		fill, text, border = headerCol, rgb(20, 33, 61), shade(headerCol, 75)
	}
	if pressed {
		fill = shade(fill, 82)
	}
	if disabled {
		if darkMode {
			fill, text, border = rgb(34, 38, 50), rgb(96, 102, 118), rgb(44, 50, 64)
		} else {
			fill, text, border = rgb(236, 232, 222), rgb(168, 164, 156), rgb(226, 220, 206)
		}
	}

	pFillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), curBg())
	br, _, _ := pCreateSolidBrush.Call(fill)
	pen, _, _ := pCreatePen.Call(0, uintptr(sc(1)), border)
	ob, _, _ := pSelectObject.Call(hdc, br)
	op, _, _ := pSelectObject.Call(hdc, pen)
	pRoundRect.Call(hdc, uintptr(r.L), uintptr(r.T), uintptr(r.R), uintptr(r.B), uintptr(sc(14)), uintptr(sc(14)))
	pSelectObject.Call(hdc, ob)
	pSelectObject.Call(hdc, op)
	pDeleteObject.Call(br)
	pDeleteObject.Call(pen)

	pSetBkMode.Call(hdc, 1)
	pSetTextColor.Call(hdc, text)
	label := getText(h)
	of, _, _ := pSelectObject.Call(hdc, btnFont[h])
	if parts := strings.SplitN(label, "\n", 2); len(parts) == 2 {
		top := rect{r.L, r.T, r.R, r.T + (r.B-r.T)*60/100}
		pDrawTextW.Call(hdc, up(parts[0]), ^uintptr(0), uintptr(unsafe.Pointer(&top)), 0x1|0x8|0x20) // CENTER|BOTTOM|SINGLELINE
		pSelectObject.Call(hdc, fontSm)
		bot := rect{r.L, top.B + sc(3), r.R, r.B}
		pDrawTextW.Call(hdc, up(parts[1]), ^uintptr(0), uintptr(unsafe.Pointer(&bot)), 0x1|0x20)
	} else {
		rr := r
		pDrawTextW.Call(hdc, up(label), ^uintptr(0), uintptr(unsafe.Pointer(&rr)), 0x1|0x4|0x20) // CENTER|VCENTER|SINGLELINE
	}
	pSelectObject.Call(hdc, of)
}

func settingsPath() string { return filepath.Join(dataDir(), "settings.ini") }

func loadSettings() {
	// move files from the old location (next to the .exe) the first time
	for _, n := range []string{"settings.ini", "last_recording.pacr"} {
		newP := filepath.Join(dataDir(), n)
		if _, err := os.Stat(newP); err != nil {
			if b, err := os.ReadFile(oldPath(n)); err == nil {
				_ = os.WriteFile(newP, b, 0644)
			}
		}
	}
	b, err := os.ReadFile(settingsPath())
	if err == nil && strings.Contains(string(b), "dark=1") {
		darkMode = true
	}
}

func saveSettings() {
	v := "0"
	if darkMode {
		v = "1"
	}
	_ = os.WriteFile(settingsPath(), []byte("dark="+v+"\n"), 0644)
}

func applyTheme() {
	on := int32(0)
	if darkMode {
		on = 1
	}
	pDwmSetAttr.Call(hwndMain, 20, uintptr(unsafe.Pointer(&on)), 4)
	pDwmSetAttr.Call(hwndMain, 19, uintptr(unsafe.Pointer(&on)), 4)
	for _, h := range editLike {
		if darkMode {
			pSetWindowTheme.Call(h, up("DarkMode_CFD"), 0)
		} else {
			pSetWindowTheme.Call(h, 0, 0)
		}
	}
	pRedrawWindow.Call(hwndMain, 0, 0, 0x1|0x4|0x80|0x400|0x100)
}

func setStatus(s string, col int) {
	statusKind = col
	setText(hStatus, s)
}

func refreshInfo() { setText(hInfo, cur.summary()) }

func setBusy(busy bool) {
	for _, h := range allInputs {
		enable(h, !busy)
	}
	enable(hStop, busy)
	enable(hPlay, !busy && cur != nil && len(cur.Ev) > 0)
	enable(hRecord, state != stPlaying)
	if state == stRecording {
		setText(hRecord, "■  DONE\nF6")
	} else {
		setText(hRecord, "●  REC\nF6")
	}
	if !busy {
		enable(hEditCount, !isChecked(hInf))
	}
}

func idleStatus() {
	msg := "Press ●  to record"
	if cur != nil && len(cur.Ev) > 0 {
		msg = "Ready  —  press ▶"
	}
	if !hotkeysOK {
		msg += "   (⚠ F-keys busy)"
	}
	setStatus(msg, cNeutral)
}

func minimizeIfWanted() {
	if isChecked(hMin) {
		pShowWindow.Call(hwndMain, 6)
	}
}
func restoreIfMin() {
	if r, _, _ := pIsIconic.Call(hwndMain); r != 0 {
		pShowWindow.Call(hwndMain, 9)
	}
}

// ───────────────────────────── Actions ─────────────────────────────

func startRecording() {
	if state != stIdle {
		return
	}
	var pt point
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	recStart = pt
	preHeld = map[uint8]bool{}
	for vk, k := range map[uintptr]uint8{0x01: kLUp, 0x02: kRUp, 0x04: kMUp, 0x05: 101, 0x06: 102} {
		if st, _, _ := pGetAsyncKeyState.Call(vk); st&0x8000 != 0 {
			preHeld[k] = true
		}
	}
	recBuf = make([]Event, 0, 1<<16)
	recBase = qpc()
	state = stRecording
	installHook()
	setBusy(true)
	setStatus("●  Recording…   F6 to finish", cRed)
	minimizeIfWanted()
}

func stopRecording(viaButton bool) {
	if state != stRecording {
		return
	}
	dur := usSince(recBase)
	state = stIdle
	removeHook()
	ev := recBuf
	if viaButton {
		// Drop the click on our own DONE button (and the approach movement inside our window).
		var wr rect
		pGetWindowRect.Call(hwndMain, uintptr(unsafe.Pointer(&wr)))
		in := func(e Event) bool { return e.X >= wr.L && e.X < wr.R && e.Y >= wr.T && e.Y < wr.B }
		for i := len(ev) - 1; i >= 0; i-- {
			if ev[i].K == kLDown {
				if in(ev[i]) {
					ev = ev[:i]
					for len(ev) > 0 && ev[len(ev)-1].K == kMove && in(ev[len(ev)-1]) {
						ev = ev[:len(ev)-1]
					}
				}
				break
			}
		}
		if len(ev) > 0 {
			dur = ev[len(ev)-1].T + 50_000
		}
	}
	restoreIfMin()
	if len(ev) == 0 {
		setBusy(false)
		setStatus("⚠  Nothing recorded — try again", cRed)
		return
	}
	cur = &Recording{StartX: recStart.X, StartY: recStart.Y, Dur: dur, Ev: ev}
	_ = cur.save(lastRecPath())
	refreshInfo()
	setBusy(false)
	setStatus("✔  Recorded!  Press ▶", cGreen)
}

func startPlaying() {
	if state != stIdle || cur == nil || len(cur.Ev) == 0 {
		return
	}
	loops := 0
	if !isChecked(hInf) {
		loops, _ = strconv.Atoi(strings.TrimSpace(getText(hEditCount)))
		if loops < 1 {
			loops = 1
			setText(hEditCount, "1")
		}
	}
	gap, _ := strconv.Atoi(strings.TrimSpace(getText(hEditGap)))
	if gap < 0 {
		gap = 0
	}
	speed, err := strconv.ParseFloat(strings.TrimSpace(strings.ReplaceAll(getText(hEditSpeed), ",", ".")), 64)
	if err != nil || speed <= 0 {
		speed = 1
	}
	if speed < 0.1 {
		speed = 0.1
	}
	if speed > 10 {
		speed = 10
	}
	setText(hEditSpeed, strconv.FormatFloat(speed, 'f', -1, 64))

	smartMode = isChecked(hSmart)
	stopOnMove = isChecked(hStopMove)
	playTotal = loops
	stopFlag.Store(false)
	userMoved.Store(0)
	state = stPlaying
	if stopOnMove {
		installHook()
	}
	setBusy(true)
	setStatus("▶  Playing…   F8 to stop", cBlue)
	minimizeIfWanted()
	rec := *cur
	playDone = make(chan struct{})
	go playLoop(&rec, loops, gap, speed, smartMode, playDone)
}

func stopAll(viaButton bool) {
	switch state {
	case stRecording:
		stopRecording(viaButton)
	case stPlaying:
		stopFlag.Store(true) // playLoop posts wmAppDone when it exits
	}
}

func fileDialog(save bool) string {
	buf := make([]uint16, 1024)
	filter := utf16z("Peter Auto Clicker recording (*.pacr)", "*.pacr", "All files (*.*)", "*.*")
	ofn := openFileName{
		Owner:   hwndMain,
		Filter:  &filter[0],
		File:    &buf[0],
		MaxFile: uint32(len(buf)),
		DefExt:  u16("pacr"),
	}
	ofn.StructSize = uint32(unsafe.Sizeof(ofn))
	var r uintptr
	if save {
		copy(buf, syscall.StringToUTF16("my_recording.pacr"))
		ofn.Flags = 0x2 | 0x800
		r, _, _ = pGetSaveFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	} else {
		ofn.Flags = 0x1000 | 0x800
		r, _, _ = pGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	}
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

func utf16z(parts ...string) []uint16 {
	var out []uint16
	for _, p := range parts {
		out = append(out, syscall.StringToUTF16(p)...)
	}
	return append(out, 0)
}

// ───────────────────────────── Window procedure ─────────────────────────────

func wndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case 0x002B: // WM_DRAWITEM
		drawButton((*drawItem)(unsafe.Pointer(lParam)))
		return 1

	case wmCommand:
		id := int(loword(wParam))
		code := hiword(wParam)
		if code != 0 && code != 5 { // BN_CLICKED / BN_DOUBLECLICKED
			break
		}
		switch id {
		case idRecord:
			if state == stRecording {
				stopRecording(true)
			} else {
				startRecording()
			}
		case idPlay:
			startPlaying()
		case idStop:
			stopAll(true)
		case idInf:
			setCheck(hInf, !isChecked(hInf))
			enable(hEditCount, !isChecked(hInf))
		case idPrecise, idSmart:
			setCheck(hPrecise, id == idPrecise)
			setCheck(hSmart, id == idSmart)
		case idStopMove:
			setCheck(hStopMove, !isChecked(hStopMove))
		case idMin:
			setCheck(hMin, !isChecked(hMin))
		case idDark:
			darkMode = !darkMode
			setCheck(hDark, darkMode)
			saveSettings()
			applyTheme()
		case idTop:
			setCheck(hTop, !isChecked(hTop))
			after := ^uintptr(1) // HWND_NOTOPMOST
			if isChecked(hTop) {
				after = ^uintptr(0) // HWND_TOPMOST
			}
			pSetWindowPos.Call(hwndMain, after, 0, 0, 0, 0, 0x1|0x2|0x10)
		case idSave:
			if cur == nil || len(cur.Ev) == 0 {
				setStatus("⚠  Record something first", cOrange)
				break
			}
			if path := fileDialog(true); path != "" {
				if err := cur.save(path); err != nil {
					msgBox("Could not save: "+err.Error(), 0x10)
				} else {
					setStatus("💾  Saved", cGreen)
				}
			}
		case idLoad:
			if path := fileDialog(false); path != "" {
				r, err := loadRecording(path)
				if err != nil {
					msgBox("Could not open that file: "+err.Error(), 0x10)
					break
				}
				cur = r
				refreshInfo()
				setBusy(false)
				setStatus("📂  Loaded  —  press ▶", cGreen)
			}
		case idClear:
			if cur == nil || len(cur.Ev) == 0 {
				break
			}
			if r, _, _ := pMessageBoxW.Call(hwndMain, up("Delete this recording?"), up("Peter Auto Clicker"), 0x4|0x20|0x100); r != 6 {
				break // not IDYES
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
		case 1: // F6
			if state == stRecording {
				stopRecording(false)
			} else if state == stIdle {
				startRecording()
			}
		case 2: // F7
			if state == stPlaying {
				stopAll(false)
			} else if state == stIdle {
				startPlaying()
			}
		case 3: // F8
			stopAll(false)
		}
		return 0

	case wmAppLoop:
		if state == stPlaying {
			total := "∞"
			if playTotal > 0 {
				total = strconv.Itoa(playTotal)
			}
			icon := "🎯"
			if smartMode {
				icon = "⚡"
			}
			setStatus(fmt.Sprintf("▶  🔁 %d / %s   %s   ·   F8 ■", wParam, total, icon), cBlue)
		}
		return 0

	case wmAppStopPlay:
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
			setStatus("✋  Stopped (you moved the mouse)", cOrange)
		case wasStopped:
			setStatus("■  Stopped", cOrange)
		default:
			setStatus("✔  Done!", cGreen)
		}
		return 0

	case wmCtlColorStatic, wmCtlColorBtn:
		hdc := wParam
		pSetBkMode.Call(hdc, 1)
		if lParam == hStatus {
			pSetTextColor.Call(hdc, statusCol())
		} else {
			pSetTextColor.Call(hdc, textCol())
		}
		if inCard[lParam] {
			pSetBkColor.Call(hdc, cardFill())
			if darkMode {
				return darkEdit
			}
			return editBrush
		}
		if darkMode {
			pSetBkColor.Call(hdc, rgb(24, 28, 40))
		} else {
			pSetBkColor.Call(hdc, rgb(250, 247, 238))
		}
		return curBg()

	case wmCtlColorEdit:
		hdc := wParam
		if darkMode {
			pSetTextColor.Call(hdc, rgb(240, 242, 248))
			pSetBkColor.Call(hdc, rgb(42, 48, 64))
			return darkEdit
		}
		pSetTextColor.Call(hdc, rgb(20, 33, 61))
		pSetBkColor.Call(hdc, rgb(255, 255, 255))
		return editBrush

	case 0x0014: // WM_ERASEBKGND
		var cr rect
		pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&cr)))
		pFillRect.Call(wParam, uintptr(unsafe.Pointer(&cr)), curBg())
		return 1

	case wmPaint:
		var ps paintStruct
		hdc, _, _ := pBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		hr := rect{0, 0, sc(winW), sc(104)}
		pFillRect.Call(hdc, uintptr(unsafe.Pointer(&hr)), headBrush)
		if logoPix != nil {
			bi := bmiHeader{Width: logoW, Height: -logoH, Planes: 1, Bits: 32}
			bi.Size = uint32(unsafe.Sizeof(bi))
			pSetStretchBlt.Call(hdc, 4)
			pSetBrushOrgEx.Call(hdc, 0, 0, 0)
			pStretchDIBits.Call(hdc, uintptr(sc(12)), uintptr(sc(7)), uintptr(sc(90)), uintptr(sc(90)),
				0, 0, uintptr(logoW), uintptr(logoH),
				uintptr(unsafe.Pointer(&logoPix[0])), uintptr(unsafe.Pointer(&bi)), 0, 0xCC0020)
		}
		border := rgb(214, 203, 170)
		if darkMode {
			border = rgb(72, 80, 102)
		}
		cb, _, _ := pCreateSolidBrush.Call(cardFill())
		cp, _, _ := pCreatePen.Call(0, uintptr(sc(1)), border)
		ob, _, _ := pSelectObject.Call(hdc, cb)
		op, _, _ := pSelectObject.Call(hdc, cp)
		for _, c := range cards {
			pRoundRect.Call(hdc, uintptr(c.L), uintptr(c.T), uintptr(c.R), uintptr(c.B), uintptr(sc(14)), uintptr(sc(14)))
		}
		pSelectObject.Call(hdc, ob)
		pSelectObject.Call(hdc, op)
		pDeleteObject.Call(cb)
		pDeleteObject.Call(cp)

		pSetBkMode.Call(hdc, 1)
		pSetTextColor.Call(hdc, rgb(20, 33, 61))
		old, _, _ := pSelectObject.Call(hdc, fontTitle)
		tr := rect{sc(116), 0, sc(winW - 8), sc(104)}
		pDrawTextW.Call(hdc, up("Peter Auto Clicker"), ^uintptr(0), uintptr(unsafe.Pointer(&tr)), 0x4|0x20|0x100)
		pSelectObject.Call(hdc, old)
		pEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
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
		removeHook()
		for i := 1; i <= 3; i++ {
			pUnregisterHotKey.Call(hwnd, uintptr(i))
		}
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return r
}

// ───────────────────────────── Build the window ─────────────────────────────

const (
	winW = 420
	winH = 432
)

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
	logoPix, logoW, logoH = pix, int32(b.Dx()), int32(b.Dy())
	c := rgba.RGBAAt(6, 6)
	headerCol = rgb(c.R, c.G, c.B)
}

func buildControls() {
	// Tooltips: hover any icon to see what it does.
	icc := [2]uint32{8, 0x8} // ICC_TAB_CLASSES (tooltips)
	pInitCommonCtrls.Call(uintptr(unsafe.Pointer(&icc)))
	hTip, _, _ = pCreateWindowExW.Call(0x8, up("tooltips_class32"), 0, 0x80000000|0x1|0x2,
		0x80000000, 0x80000000, 0x80000000, 0x80000000, hwndMain, 0, hInst, 0)
	pSendMessageW.Call(hTip, 0x418, 0, uintptr(sc(260))) // TTM_SETMAXTIPWIDTH

	hStatus = mkCtl("STATIC", "", ssNoPrefix|0x1|0x200, 0, 14, 112, 392, 28, 0, fontSt)
	hInfo = mkCtl("STATIC", "", ssNoPrefix|0x1|0x200, 0, 14, 140, 392, 22, 0, fontSm)

	// Big three
	hRecord = mkBtn("●  REC\nF6", 14, 170, 124, 72, idRecord, fontBtnBig, roleRec, "Record your mouse  (F6)\nPress again to finish")
	hPlay = mkBtn("▶  PLAY\nF7", 148, 170, 124, 72, idPlay, fontBtnBig, rolePlay, "Play the recording  (F7)")
	hStop = mkBtn("■  STOP\nF8", 282, 170, 124, 72, idStop, fontBtnBig, roleStop, "Stop  (F8)")

	// Row 1  —  [🔁 10]  [∞]  [⏳ 0 ms]      (same 3 columns as the big buttons)
	y := 252
	addCard(14, y, 124, 40)
	mkIcon("🔁", 16, y, 34, 40, "How many times to repeat")
	hEditCount = mkEdit("10", 52, y+8, 78, 24, idEditCount, esNumber, "How many times to repeat")
	hInf = mkBtn("∞", 148, y, 124, 40, idInf, fontIcon, roleNormal, "Loop forever (until F8)")
	addCard(282, y, 124, 40)
	mkIcon("⏳", 284, y, 34, 40, "Pause between loops (milliseconds)")
	hEditGap = mkEdit("0", 320, y+8, 52, 24, idEditGap, esNumber, "Pause between loops (milliseconds)")
	mkUnit("ms", 372, y, 30, 40)

	// Row 2  —  [⏩ 1 ×]  [🎯 Precise]  [⚡ Smart]
	y = 302
	addCard(14, y, 124, 40)
	mkIcon("⏩", 16, y, 34, 40, "Speed:  1 = real pace,  2 = 2× faster,  0.5 = slower")
	hEditSpeed = mkEdit("1", 52, y+8, 52, 24, idEditSpeed, 0, "Speed:  1 = real pace,  2 = 2× faster,  0.5 = slower")
	mkUnit("×", 104, y, 30, 40)
	hPrecise = mkBtn("🎯  Precise", 148, y, 124, 40, idPrecise, fontUI, roleNormal, "Precise mode: replays every movement at the exact pace")
	hSmart = mkBtn("⚡  Smart", 282, y, 124, 40, idSmart, fontUI, roleNormal, "Smart mode: use if it lags.\nKeeps the rhythm, never misses a click")

	// Row 3  —  7 equal icon buttons across the full width
	y = 352
	x := func(i int) int { return 14 + i*57 } // 7 × 50px buttons, 7px gaps = 392px
	w := 50
	hStopMove = mkBtn("✋", x(0), y, w, 44, idStopMove, fontIcon, roleNormal, "Stop when I move the mouse")
	hMin = mkBtn("➖", x(1), y, w, 44, idMin, fontIcon, roleNormal, "Hide this window while running")
	hTop = mkBtn("📌", x(2), y, w, 44, idTop, fontIcon, roleNormal, "Keep on top")
	hDark = mkBtn("🌙", x(3), y, w, 44, idDark, fontIcon, roleNormal, "Dark mode")
	hSave = mkBtn("💾", x(4), y, w, 44, idSave, fontIcon, roleNormal, "Save recording")
	hLoad = mkBtn("📂", x(5), y, w, 44, idLoad, fontIcon, roleNormal, "Open recording")
	hClear = mkBtn("🗑", x(6), y, w, 44, idClear, fontIcon, roleNormal, "Delete recording")

	mkCtl("STATIC", "F6 ●     F7 ▶     F8 ■", ssNoPrefix|0x1|0x200, 0, 14, 404, 392, 22, 0, fontSm)

	setCheck(hPrecise, true)
	setCheck(hStopMove, true)
	setCheck(hDark, darkMode)

	allInputs = []uintptr{hPlay, hEditCount, hInf, hEditGap, hEditSpeed,
		hPrecise, hSmart, hStopMove, hMin, hTop, hDark, hSave, hLoad, hClear}
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

	// FIX F: only one copy at a time (two copies would fight over F6/F7/F8)
	if w, _, _ := pFindWindowW.Call(up("PeterAutoClickerWnd"), 0); w != 0 {
		pShowWindow.Call(w, 9)
		pSetForegroundWindow.Call(w)
		return
	}
	loadLogo()
	if headerCol == 0 {
		headerCol = rgb(222, 184, 73)
	}
	loadSettings()
	bgBrush, _, _ = pCreateSolidBrush.Call(rgb(250, 247, 238))
	headBrush, _, _ = pCreateSolidBrush.Call(headerCol)
	darkBg, _, _ = pCreateSolidBrush.Call(rgb(24, 28, 40))
	darkEdit, _, _ = pCreateSolidBrush.Call(rgb(42, 48, 64))
	editBrush, _, _ = pCreateSolidBrush.Call(rgb(255, 255, 255))

	sym := "Segoe UI Symbol"
	fontUI = mkFont(12, 600, sym)
	fontSm = mkFont(9, 600, sym)
	fontSt = mkFont(12, 700, sym)
	fontIcon = mkFont(16, 400, sym)
	fontBtnBig = mkFont(15, 800, sym)
	fontTitle = mkFont(19, 800, "Segoe UI")

	var bigIcon, smallIcon uintptr
	if exe, err := os.Executable(); err == nil {
		pExtractIconExW.Call(up(exe), 0, uintptr(unsafe.Pointer(&bigIcon)), uintptr(unsafe.Pointer(&smallIcon)), 1)
	}
	cursor, _, _ := pLoadCursorW.Call(0, 32512)
	wc := wndClassEx{
		WndProc:    syscall.NewCallback(wndProc),
		Instance:   hInst,
		Icon:       bigIcon,
		IconSm:     smallIcon,
		Cursor:     cursor,
		Background: bgBrush,
		ClassName:  u16("PeterAutoClickerWnd"),
	}
	wc.Size = uint32(unsafe.Sizeof(wc))
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	style := uint32(wsOverlapped | wsCaption | wsSysMenu | wsMinimizeBox)
	cr := rect{0, 0, sc(winW), sc(winH)}
	pAdjustWindowRectEx.Call(uintptr(unsafe.Pointer(&cr)), uintptr(style), 0, 0)
	hwndMain, _, _ = pCreateWindowExW.Call(0, up("PeterAutoClickerWnd"), up("Peter Auto Clicker"),
		uintptr(style), 0x80000000, 0x80000000,
		uintptr(cr.R-cr.L), uintptr(cr.B-cr.T), 0, 0, hInst, 0)
	if hwndMain == 0 {
		return
	}
	buildControls()

	ok1, _, _ := pRegisterHotKey.Call(hwndMain, 1, 0x4000, vkF6)
	ok2, _, _ := pRegisterHotKey.Call(hwndMain, 2, 0x4000, vkF7)
	ok3, _, _ := pRegisterHotKey.Call(hwndMain, 3, 0x4000, vkF8)
	hotkeysOK = ok1 != 0 && ok2 != 0 && ok3 != 0

	if r, err := loadRecording(lastRecPath()); err == nil {
		cur = r
	}
	refreshInfo()
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
		if d, _, _ := pIsDialogMessageW.Call(hwndMain, uintptr(unsafe.Pointer(&m))); d != 0 {
			continue
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	stopFlag.Store(true)
}
