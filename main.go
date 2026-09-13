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
	appName       = "Drizzle Gateway"
	defaultPort   = 4983
	windowWidth   = 1180
	windowHeight  = 760
	healthTimeout = 20 * time.Second
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

	modUser32            = syscall.NewLazyDLL("user32.dll")
	procGetSystemMetrics = modUser32.NewProc("GetSystemMetrics")
	procSetWindowPos     = modUser32.NewProc("SetWindowPos")
	procLoadIconW        = modUser32.NewProc("LoadIconW")
	procSendMessageW     = modUser32.NewProc("SendMessageW")

	modShell32                                  = syscall.NewLazyDLL("shell32.dll")
	procSetCurrentProcessExplicitAppUserModelID = modShell32.NewProc("SetCurrentProcessExplicitAppUserModelID")
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

func countGatewayWindows() int {
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
		exeName := syscall.UTF16ToString(entry.SzExeFile[:])
		if strings.EqualFold(exeName, "DrizzleGateway.exe") {
			count++
		}
		ret, _, _ = procProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&entry)))
	}
	return count
}

func setWindowIcon(hwnd uintptr) {
	hInst, _, _ := procGetModuleHandleW.Call(0)
	hIcon, _, _ := procLoadIconW.Call(hInst, uintptr(1))
	if hIcon != 0 {
		const WM_SETICON = 0x0080
		const ICON_SMALL = 0
		const ICON_BIG = 1
		procSendMessageW.Call(hwnd, WM_SETICON, ICON_SMALL, hIcon)
		procSendMessageW.Call(hwnd, WM_SETICON, ICON_BIG, hIcon)
	}
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
		return data == 0 // 0 means Dark mode, 1 means Light mode
	}
	return false
}

func applyImmersiveDarkMode(hwnd uintptr, isDark bool) {
	if procDwmSetWindowAttribute.Find() != nil {
		return
	}
	var val int32 = 0
	if isDark {
		val = 1
	}
	procDwmSetWindowAttribute.Call(hwnd, 20, uintptr(unsafe.Pointer(&val)), 4)
	procDwmSetWindowAttribute.Call(hwnd, 19, uintptr(unsafe.Pointer(&val)), 4)
}

func appDataDir() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, _ := os.UserHomeDir()
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	dir := filepath.Join(appData, "DrizzleGateway", "data")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func serverBinaryPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(exe)
	candidate := filepath.Join(dir, "DrizzleGatewayServer.exe")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	if _, err := os.Stat("DrizzleGatewayServer.exe"); err == nil {
		abs, _ := filepath.Abs("DrizzleGatewayServer.exe")
		return abs, nil
	}
	return "", fmt.Errorf("DrizzleGatewayServer.exe not found alongside DrizzleGateway.exe")
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

func stopAllServers() {
	killCmd := exec.Command("taskkill", "/F", "/T", "/IM", "DrizzleGatewayServer.exe")
	killCmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	_ = killCmd.Run()
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
	if procSetCurrentProcessExplicitAppUserModelID.Find() == nil {
		appID, _ := syscall.UTF16PtrFromString("Drizzle.Gateway.Windows")
		procSetCurrentProcessExplicitAppUserModelID.Call(uintptr(unsafe.Pointer(appID)))
	}

	isDark := isWindowsDarkMode()
	wvDir := filepath.Join(filepath.Dir(appDataDir()), "webview2")
	_ = os.MkdirAll(wvDir, 0o755)
	_ = os.Setenv("WEBVIEW2_USER_DATA_FOLDER", wvDir)

	w := webview.New(false)
	defer w.Destroy()

	w.SetTitle(appName)
	w.SetSize(windowWidth, windowHeight, webview.HintNone)

	hwnd := uintptr(w.Window())
	setWindowIcon(hwnd)
	applyImmersiveDarkMode(hwnd, isDark)
	centerWindow(hwnd, windowWidth, windowHeight)

	port := defaultPort
	storeDir := appDataDir()

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

		if countGatewayWindows() <= 1 {
			cmdMu.Lock()
			cmd := serverCmd
			cmdMu.Unlock()
			if cmd != nil && cmd.Process != nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
			stopAllServers()
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

	serverBin, err := serverBinaryPath()
	if err != nil && !isHealthy(port) {
		errMsg, _ := json.Marshal(err.Error())
		w.SetHtml(getLoadingHTML(isDark, fmt.Sprintf("Error: %s", string(errMsg))))
		w.Run()
		return
	}

	w.SetHtml(getLoadingHTML(isDark, "Starting Drizzle Gateway..."))

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
					w.SetHtml(getLoadingHTML(isDark, "Drizzle Gateway timed out while starting."))
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
