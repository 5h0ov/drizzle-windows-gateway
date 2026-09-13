package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	webview "github.com/webview/webview_go"
)

const (
	defaultPort   = 4983
	windowWidth   = 1180
	windowHeight  = 760
	healthTimeout = 20 * time.Second

	GWLP_WNDPROC     = -4
	WM_CLOSE         = 0x0010
	WM_SYSCOMMAND    = 0x0112
	SC_MINIMIZE      = 0xF020
	WM_USER          = 0x0400
	WM_TRAYICON      = WM_USER + 100
	WM_LBUTTONUP     = 0x0202
	WM_LBUTTONDBLCLK = 0x0203
	WM_RBUTTONUP     = 0x0205

	NIM_ADD    = 0x00000000
	NIM_MODIFY = 0x00000001
	NIM_DELETE = 0x00000002

	NIF_MESSAGE = 0x00000001
	NIF_ICON    = 0x00000002
	NIF_TIP     = 0x00000004

	SW_HIDE          = 0
	SW_SHOWNORMAL    = 1
	SW_SHOWMAXIMIZED = 3
	SW_SHOW          = 5
	SW_RESTORE       = 9

	ID_TRAY_SHOW = 1001
	ID_TRAY_NEW  = 1002
	ID_TRAY_QUIT = 1003

	TPM_RETURNCMD   = 0x0100
	TPM_RIGHTBUTTON = 0x0002
	MF_SEPARATOR    = 0x0800
	MF_STRING       = 0x0000

	MONITOR_DEFAULTTONULL = 0
)

var (
	modDwmapi                 = syscall.NewLazyDLL("dwmapi.dll")
	procDwmSetWindowAttribute = modDwmapi.NewProc("DwmSetWindowAttribute")

	modAdvapi32          = syscall.NewLazyDLL("advapi32.dll")
	procRegOpenKeyExW    = modAdvapi32.NewProc("RegOpenKeyExW")
	procRegQueryValueExW = modAdvapi32.NewProc("RegQueryValueExW")
	procRegCloseKey      = modAdvapi32.NewProc("RegCloseKey")

	modKernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procGetModuleHandleW         = modKernel32.NewProc("GetModuleHandleW")
	procCreateToolhelp32Snapshot = modKernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW          = modKernel32.NewProc("Process32FirstW")
	procProcess32NextW           = modKernel32.NewProc("Process32NextW")
	procCloseHandle              = modKernel32.NewProc("CloseHandle")

	modUser32               = syscall.NewLazyDLL("user32.dll")
	procGetSystemMetrics    = modUser32.NewProc("GetSystemMetrics")
	procSetWindowPos        = modUser32.NewProc("SetWindowPos")
	procLoadIconW           = modUser32.NewProc("LoadIconW")
	procSendMessageW        = modUser32.NewProc("SendMessageW")
	procGetWindowRect       = modUser32.NewProc("GetWindowRect")
	procIsZoomed            = modUser32.NewProc("IsZoomed")
	procShowWindow          = modUser32.NewProc("ShowWindow")
	procSetForegroundWindow = modUser32.NewProc("SetForegroundWindow")
	procMonitorFromRect     = modUser32.NewProc("MonitorFromRect")
	procSetWindowLongPtrW   = modUser32.NewProc("SetWindowLongPtrW")
	procCallWindowProcW     = modUser32.NewProc("CallWindowProcW")
	procCreatePopupMenu     = modUser32.NewProc("CreatePopupMenu")
	procAppendMenuW         = modUser32.NewProc("AppendMenuW")
	procTrackPopupMenu      = modUser32.NewProc("TrackPopupMenu")
	procDestroyMenu         = modUser32.NewProc("DestroyMenu")
	procGetCursorPos        = modUser32.NewProc("GetCursorPos")

	modShell32                                  = syscall.NewLazyDLL("shell32.dll")
	procShell_NotifyIconW                       = modShell32.NewProc("Shell_NotifyIconW")
	procSetCurrentProcessExplicitAppUserModelID = modShell32.NewProc("SetCurrentProcessExplicitAppUserModelID")

	origWndProc uintptr
	reallyQuit  bool
)

type PROCESSENTRY32W struct {
	DwSize              uint32
	CntUsage            uint32
	Th32ProcessID       uint32
	Th32DefaultHeapID   uintptr
	Th32ModuleID        uint32
	CntThreads          uint32
	Th32ParentProcessID uint32
	PcPriClassBase      int32
	DwFlags             uint32
	SzExeFile           [260]uint16
}

type RECT struct {
	Left, Top, Right, Bottom int32
}

type POINT struct {
	X, Y int32
}

type WindowState struct {
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	Maximized bool `json:"maximized"`
}

type NOTIFYICONDATAW struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	TimeoutOrVersion uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         [16]byte
	HBalloonIcon     uintptr
}

func getAppIdentity() (title string, dirName string, port int, isEnhanced bool, exeBaseName string) {
	exe, err := os.Executable()
	exeBase := "DrizzleGateway.exe"
	if err == nil {
		exeBase = filepath.Base(exe)
	}
	if strings.Contains(strings.ToLower(exeBase), "enhanced") {
		return "Drizzle Gateway (Enhanced)", "DrizzleGateway-Enhanced", 4984, true, exeBase
	}
	return "Drizzle Gateway", "DrizzleGateway", defaultPort, false, exeBase
}

func countGatewayWindows(exeBaseName string) int {
	snap, _, _ := procCreateToolhelp32Snapshot.Call(0x00000002, 0) // TH32CS_SNAPPROCESS
	if snap == uintptr(syscall.InvalidHandle) {
		return 1
	}
	defer procCloseHandle.Call(snap)

	var entry PROCESSENTRY32W
	entry.DwSize = uint32(unsafe.Sizeof(entry))

	ret, _, _ := procProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&entry)))
	count := 0
	for ret != 0 {
		name := syscall.UTF16ToString(entry.SzExeFile[:])
		if strings.EqualFold(name, exeBaseName) {
			count++
		}
		ret, _, _ = procProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&entry)))
	}
	return count
}

func getAppIcon() uintptr {
	hInst, _, _ := procGetModuleHandleW.Call(0)
	hIcon, _, _ := procLoadIconW.Call(hInst, uintptr(1))
	return hIcon
}

func setWindowIcon(hwnd uintptr, hIcon uintptr) {
	if hIcon != 0 {
		const WM_SETICON = 0x0080
		procSendMessageW.Call(hwnd, WM_SETICON, 0, hIcon) // ICON_SMALL
		procSendMessageW.Call(hwnd, WM_SETICON, 1, hIcon) // ICON_BIG
	}
}

func getWindowStatePath(dirName string) string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, _ := os.UserHomeDir()
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	return filepath.Join(appData, dirName, "window.json")
}

func saveWindowState(hwnd uintptr, dirName string) {
	var r RECT
	ret, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	if ret == 0 {
		return
	}
	isZoom, _, _ := procIsZoomed.Call(hwnd)
	ws := WindowState{
		X:         int(r.Left),
		Y:         int(r.Top),
		Width:     int(r.Right - r.Left),
		Height:    int(r.Bottom - r.Top),
		Maximized: isZoom != 0,
	}
	data, err := json.MarshalIndent(ws, "", "  ")
	if err == nil {
		_ = os.WriteFile(getWindowStatePath(dirName), data, 0o644)
	}
}

func restoreWindowState(hwnd uintptr, dirName string) bool {
	data, err := os.ReadFile(getWindowStatePath(dirName))
	if err != nil {
		return false
	}
	var ws WindowState
	if err := json.Unmarshal(data, &ws); err != nil {
		return false
	}
	if ws.Width < 400 || ws.Height < 300 {
		return false
	}
	r := RECT{
		Left:   int32(ws.X),
		Top:    int32(ws.Y),
		Right:  int32(ws.X + ws.Width),
		Bottom: int32(ws.Y + ws.Height),
	}
	hMon, _, _ := procMonitorFromRect.Call(uintptr(unsafe.Pointer(&r)), MONITOR_DEFAULTTONULL)
	if hMon == 0 {
		return false
	}
	procSetWindowPos.Call(hwnd, 0, uintptr(ws.X), uintptr(ws.Y), uintptr(ws.Width), uintptr(ws.Height), 0x0040)
	if ws.Maximized {
		procShowWindow.Call(hwnd, SW_SHOWMAXIMIZED)
	}
	return true
}

func centerWindow(hwnd uintptr, width, height int) {
	screenWidth, _, _ := procGetSystemMetrics.Call(0)  // SM_CXSCREEN
	screenHeight, _, _ := procGetSystemMetrics.Call(1) // SM_CYSCREEN
	if screenWidth > 0 && screenHeight > 0 {
		x := (int(screenWidth) - width) / 2
		y := (int(screenHeight) - height) / 2
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		procSetWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), uintptr(width), uintptr(height), 0x0040)
	}
}

func isWindowsDarkMode() bool {
	var hKey uintptr
	subKey, _ := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`)
	ret, _, _ := procRegOpenKeyExW.Call(
		0x80000001, // HKEY_CURRENT_USER
		uintptr(unsafe.Pointer(subKey)),
		0,
		0x20019, // KEY_READ
		uintptr(unsafe.Pointer(&hKey)),
	)
	if ret != 0 {
		return false
	}
	defer procRegCloseKey.Call(hKey)

	valName, _ := syscall.UTF16PtrFromString("AppsUseLightTheme")
	var valType uint32
	var data uint32
	dataSize := uint32(unsafe.Sizeof(data))

	ret, _, _ = procRegQueryValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(valName)),
		0,
		uintptr(unsafe.Pointer(&valType)),
		uintptr(unsafe.Pointer(&data)),
		uintptr(unsafe.Pointer(&dataSize)),
	)
	if ret == 0 {
		return data == 0
	}
	return false
}

func applyImmersiveDarkModeAndMica(hwnd uintptr, isDark bool) {
	if procDwmSetWindowAttribute.Find() != nil {
		return
	}
	var val int32 = 0
	if isDark {
		val = 1
	}
	// DWMWA_USE_IMMERSIVE_DARK_MODE (Windows 10/11)
	procDwmSetWindowAttribute.Call(hwnd, 20, uintptr(unsafe.Pointer(&val)), 4)
	procDwmSetWindowAttribute.Call(hwnd, 19, uintptr(unsafe.Pointer(&val)), 4)

	// DWMWA_SYSTEMBACKDROP_TYPE = 38 (Windows 11 22H2+)
	// 2 = DWMSBT_MAINWINDOW (Mica)
	var backdrop int32 = 2
	procDwmSetWindowAttribute.Call(hwnd, 38, uintptr(unsafe.Pointer(&backdrop)), 4)
}

func appDataDir(dirName string) string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, _ := os.UserHomeDir()
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	dir := filepath.Join(appData, dirName, "data")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func serverBinaryPath(isEnhanced bool) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(exe)

	candidateName := "DrizzleGatewayServer.exe"
	if isEnhanced {
		candidateName = "DrizzleGatewayServer-Enhanced.exe"
	}

	candidate := filepath.Join(dir, candidateName)
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	standardCandidate := filepath.Join(dir, "DrizzleGatewayServer.exe")
	if _, err := os.Stat(standardCandidate); err == nil {
		return standardCandidate, nil
	}
	if _, err := os.Stat(candidateName); err == nil {
		abs, _ := filepath.Abs(candidateName)
		return abs, nil
	}
	return "", fmt.Errorf("DrizzleGatewayServer.exe not found alongside executable")
}

func isHealthy(port int) bool {
	url := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(url)
	if err == nil {
		resp.Body.Close()
		return resp.StatusCode == 200
	}
	return false
}

func waitHealthy(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if isHealthy(port) {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

func startServer(serverBin string, port int, storeDir string) (*exec.Cmd, error) {
	cmd := exec.Command(serverBin)
	cmd.Dir = filepath.Dir(serverBin)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PORT=%d", port),
		"STORE_PATH="+storeDir,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func stopServers(serverBinName string) {
	killCmd := exec.Command("taskkill", "/F", "/T", "/IM", serverBinName)
	killCmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	_ = killCmd.Run()
}

func setupTrayIcon(hwnd uintptr, appTitle string, hIcon uintptr) NOTIFYICONDATAW {
	var nid NOTIFYICONDATAW
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = hwnd
	nid.UID = 1
	nid.UFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP
	nid.UCallbackMessage = WM_TRAYICON
	nid.HIcon = hIcon

	tipChars, _ := syscall.UTF16FromString(appTitle)
	for i := 0; i < len(tipChars) && i < len(nid.SzTip); i++ {
		nid.SzTip[i] = tipChars[i]
	}

	procShell_NotifyIconW.Call(NIM_ADD, uintptr(unsafe.Pointer(&nid)))
	return nid
}

func showTrayContextMenu(hwnd uintptr, w webview.WebView, dirName string, pNid *NOTIFYICONDATAW, exePath string) {
	hMenu, _, _ := procCreatePopupMenu.Call()
	if hMenu == 0 {
		return
	}
	defer procDestroyMenu.Call(hMenu)

	titleStr, _ := syscall.UTF16PtrFromString("Show Window")
	newWinStr, _ := syscall.UTF16PtrFromString("Open New Window")
	quitStr, _ := syscall.UTF16PtrFromString("Quit Drizzle Gateway")

	procAppendMenuW.Call(hMenu, MF_STRING, ID_TRAY_SHOW, uintptr(unsafe.Pointer(titleStr)))
	procAppendMenuW.Call(hMenu, MF_STRING, ID_TRAY_NEW, uintptr(unsafe.Pointer(newWinStr)))
	procAppendMenuW.Call(hMenu, MF_SEPARATOR, 0, 0)
	procAppendMenuW.Call(hMenu, MF_STRING, ID_TRAY_QUIT, uintptr(unsafe.Pointer(quitStr)))

	var pt POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForegroundWindow.Call(hwnd)

	cmd, _, _ := procTrackPopupMenu.Call(
		hMenu,
		TPM_RETURNCMD|TPM_RIGHTBUTTON,
		uintptr(pt.X),
		uintptr(pt.Y),
		0,
		hwnd,
		0,
	)

	switch cmd {
	case ID_TRAY_SHOW:
		procShowWindow.Call(hwnd, SW_SHOW)
		procShowWindow.Call(hwnd, SW_RESTORE)
		procSetForegroundWindow.Call(hwnd)
	case ID_TRAY_NEW:
		_ = exec.Command(exePath).Start()
	case ID_TRAY_QUIT:
		procShell_NotifyIconW.Call(NIM_DELETE, uintptr(unsafe.Pointer(pNid)))
		saveWindowState(hwnd, dirName)
		reallyQuit = true
		w.Terminate()
	}
}

func initSubclass(hwnd uintptr, w webview.WebView, dirName string, pNid *NOTIFYICONDATAW, exePath string) {
	callback := syscall.NewCallback(func(h uintptr, msg uint32, wParam uintptr, lParam uintptr) uintptr {
		switch msg {
		case WM_SYSCOMMAND:
			if (wParam & 0xFFF0) == SC_MINIMIZE {
				saveWindowState(h, dirName)
				procShowWindow.Call(h, SW_HIDE)
				return 0 // Minimize to tray!
			}
		case WM_CLOSE:
			saveWindowState(h, dirName)
			procShell_NotifyIconW.Call(NIM_DELETE, uintptr(unsafe.Pointer(pNid)))
			reallyQuit = true
		case WM_TRAYICON:
			switch lParam {
			case WM_LBUTTONUP, WM_LBUTTONDBLCLK:
				procShowWindow.Call(h, SW_SHOW)
				procShowWindow.Call(h, SW_RESTORE)
				procSetForegroundWindow.Call(h)
				return 0
			case WM_RBUTTONUP:
				showTrayContextMenu(h, w, dirName, pNid, exePath)
				return 0
			}
		}
		ret, _, _ := procCallWindowProcW.Call(origWndProc, h, uintptr(msg), wParam, lParam)
		return ret
	})

	orig, _, _ := procSetWindowLongPtrW.Call(hwnd, ^uintptr(3), callback)
	origWndProc = orig
}

func getLoadingHTML(isDark bool, message string) string {
	themeClass := ""
	if isDark {
		themeClass = "dark"
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" class="%s">
<head>
  <meta charset="UTF-8">
  <title>Drizzle Gateway</title>
  <style>
    body {
      margin: 0;
      display: flex;
      align-items: center;
      justify-content: center;
      height: 100vh;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      background: #f8fafc;
      color: #0f172a;
    }
    .dark body {
      background: #030711;
      color: #f8fafc;
    }
    .container {
      text-align: center;
    }
    .spinner {
      width: 40px;
      height: 40px;
      border: 3px solid rgba(56, 189, 248, 0.2);
      border-top-color: #38bdf8;
      border-radius: 50%%%%;
      animation: spin 0.8s linear infinite;
      margin: 0 auto 20px;
    }
    @keyframes spin {
      to { transform: rotate(360deg); }
    }
    h2 { font-size: 18px; font-weight: 600; margin-bottom: 8px; }
    p { font-size: 14px; opacity: 0.7; margin: 0; }
  </style>
</head>
<body>
  <div class="container">
    <div class="spinner"></div>
    <h2>Drizzle Gateway</h2>
    <p>%s</p>
  </div>
</body>
</html>`, themeClass, message)
}

func main() {
	appTitle, dirName, port, isEnhanced, exeBase := getAppIdentity()

	if procSetCurrentProcessExplicitAppUserModelID.Find() == nil {
		appIDStr := "Drizzle.Gateway.Windows"
		if isEnhanced {
			appIDStr = "Drizzle.Gateway.Windows.Enhanced"
		}
		appID, _ := syscall.UTF16PtrFromString(appIDStr)
		procSetCurrentProcessExplicitAppUserModelID.Call(uintptr(unsafe.Pointer(appID)))
	}

	isDark := isWindowsDarkMode()
	storeDir := appDataDir(dirName)
	wvDir := filepath.Join(filepath.Dir(storeDir), "webview2")
	_ = os.MkdirAll(wvDir, 0o755)
	_ = os.Setenv("WEBVIEW2_USER_DATA_FOLDER", wvDir)

	w := webview.New(false)
	defer w.Destroy()

	w.SetTitle(appTitle)
	w.SetSize(windowWidth, windowHeight, webview.HintNone)

	hwnd := uintptr(w.Window())
	hIcon := getAppIcon()
	setWindowIcon(hwnd, hIcon)
	applyImmersiveDarkModeAndMica(hwnd, isDark)

	// Restore last saved window geometry, or center if first run
	if !restoreWindowState(hwnd, dirName) {
		centerWindow(hwnd, windowWidth, windowHeight)
	}

	exePath, _ := os.Executable()
	nid := setupTrayIcon(hwnd, appTitle, hIcon)
	initSubclass(hwnd, w, dirName, &nid, exePath)

	serverBin, err := serverBinaryPath(isEnhanced)
	serverBinName := "DrizzleGatewayServer.exe"
	if err == nil {
		serverBinName = filepath.Base(serverBin)
	}

	var (
		serverCmd *exec.Cmd
		cmdMu     sync.Mutex
		cleaned   bool
		cleanMu   sync.Mutex
	)

	cleanupOnce := func() {
		cleanMu.Lock()
		defer cleanMu.Unlock()
		if cleaned {
			return
		}
		cleaned = true

		procShell_NotifyIconW.Call(NIM_DELETE, uintptr(unsafe.Pointer(&nid)))
		saveWindowState(hwnd, dirName)

		if countGatewayWindows(exeBase) <= 1 {
			cmdMu.Lock()
			cmd := serverCmd
			cmdMu.Unlock()
			if cmd != nil && cmd.Process != nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
			stopServers(serverBinName)
		}
	}
	defer cleanupOnce()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		cleanupOnce()
		os.Exit(0)
	}()

	if err != nil && !isHealthy(port) {
		errMsg, _ := json.Marshal(err.Error())
		w.SetHtml(getLoadingHTML(isDark, fmt.Sprintf("Error: %s", string(errMsg))))
		w.Run()
		return
	}

	w.SetHtml(getLoadingHTML(isDark, fmt.Sprintf("Starting %s...", appTitle)))

	go func() {
		if !isHealthy(port) {
			cmd, err := startServer(serverBin, port, storeDir)
			if err != nil {
				w.Dispatch(func() {
					w.SetHtml(getLoadingHTML(isDark, fmt.Sprintf("Failed to launch daemon: %v", err)))
				})
				return
			}
			cmdMu.Lock()
			serverCmd = cmd
			cmdMu.Unlock()

			if !waitHealthy(port, healthTimeout) {
				cleanupOnce()
				w.Dispatch(func() {
					w.SetHtml(getLoadingHTML(isDark, fmt.Sprintf("%s timed out while starting.", appTitle)))
				})
				return
			}
		}

		targetURL := fmt.Sprintf("http://127.0.0.1:%d", port)
		w.Dispatch(func() {
			w.Navigate(targetURL)
			themePref := "light"
			if isDark {
				themePref = "dark"
			}
			js := fmt.Sprintf(`
				if (!localStorage.getItem('theme')) {
					localStorage.setItem('theme', '%s');
				}
			`, themePref)
			w.Eval(js)
		})
	}()

	w.Run()
	cleanupOnce()
}

