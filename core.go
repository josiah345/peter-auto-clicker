// Peter Auto Clicker — records the exact path, pace and clicks of your mouse
// and replays them once, N times, or forever.
//
// Pure Win32 (no dependencies). Build:
//
//	GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui -s -w"
package main

import (
	"bufio"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
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
	wmAppCount       = wmApp + 4

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
	userMoved  atomic.Int64 // real-hand mouse moves seen during playback
	stopOnMove bool
	playTotal  int // loops (0 = infinite)

	smartMode = false
	darkMode  = true
	hotkeysOK = true
)

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
			if stopOnMove && !injected && uint32(wParam) == wmMouseMove && !inCountdown.Load() {
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

// inCountdown is true during the 3-2-1 before playback: moving the mouse to
// click into your game must not trigger Auto-stop.
var inCountdown atomic.Bool

func playLoop(r *Recording, loops int, gapMs int, speed float64, smart bool, countdown int, done chan struct{}) {
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

	if countdown > 0 {
		inCountdown.Store(true)
		for n := countdown; n > 0; n-- {
			pPostMessageW.Call(hwndMain, wmAppCount, uintptr(n), 0)
			if !waitUntil(qpc(), 1_000_000, false) {
				inCountdown.Store(false)
				return
			}
		}
		userMoved.Store(0)
		inCountdown.Store(false)
		pPostMessageW.Call(hwndMain, wmAppCount, 0, 0) // "go"
	}

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
	setStatus("Recording…  press "+hkName(0)+" to finish", cRed)
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
		if cur != nil && len(cur.Ev) > 0 {
			setStatus("Nothing new recorded. Last recording kept", cOrange)
		} else {
			setStatus("Nothing was recorded. Try again", cRed)
		}
		return
	}
	cur = &Recording{StartX: recStart.X, StartY: recStart.Y, Dur: dur, Ev: ev}
	_ = cur.save(lastRecPath())
	refreshInfo()
	setBusy(false)
	setStatus("Recorded!  Press Play", cGreen)
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
	stopOnMove = isChecked(hAutoStop)
	playTotal = loops
	stopFlag.Store(false)
	userMoved.Store(0)
	state = stPlaying
	if stopOnMove {
		installHook()
	}
	setBusy(true)
	setStatus("Playing…  press "+hkName(2)+" to stop", cBlue)
	minimizeIfWanted()
	rec := *cur
	playDone = make(chan struct{})
	cd := 0
	if countdownOn {
		cd = 3
	}
	go playLoop(&rec, loops, gap, speed, smartMode, cd, playDone)
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
