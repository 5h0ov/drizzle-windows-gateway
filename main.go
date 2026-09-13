package main

import (
	"encoding/json"
	"fmt"
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
)

var (
	origWndProc uintptr
	reallyQuit  bool
)

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

func initSubclass(hwnd uintptr, w webview.WebView, storeDir string, pNid *NOTIFYICONDATAW, exePath string, isMasterTray bool) {
	callback := syscall.NewCallback(func(h uintptr, msg uint32, wParam uintptr, lParam uintptr) uintptr {
		switch msg {
		case WM_SYSCOMMAND:
			if (wParam & 0xFFF0) == SC_MINIMIZE {
				saveWindowState(h, filepath.Base(filepath.Dir(storeDir)))
			}
		case WM_CLOSE:
			saveWindowState(h, filepath.Base(filepath.Dir(storeDir)))
			if isMasterTray && !reallyQuit {
				// Master window hides so the single tray launcher stays alive
				procShowWindow.Call(h, SW_HIDE)
				return 0
			}
		case WM_TRAYICON:
			switch lParam {
			case WM_LBUTTONUP, WM_LBUTTONDBLCLK:
				procShowWindow.Call(h, SW_SHOW)
				procShowWindow.Call(h, SW_RESTORE)
				procSetForegroundWindow.Call(h)
				return 0
			case WM_RBUTTONUP:
				showTrayContextMenu(h, w, storeDir, pNid, exePath)
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

	var targetConnection string
	for i := 1; i < len(os.Args); i++ {
		if (os.Args[i] == "--connection" || os.Args[i] == "-c") && i+1 < len(os.Args) {
			targetConnection = os.Args[i+1]
			break
		}
	}

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

	titleWithConn := appTitle
	if targetConnection != "" {
		titleWithConn = fmt.Sprintf("%s - %s", targetConnection, appTitle)
	}
	w.SetTitle(titleWithConn)
	w.SetSize(windowWidth, windowHeight, webview.HintNone)

	hwnd := uintptr(w.Window())
	hIcon := getAppIcon()
	setWindowIcon(hwnd, hIcon)
	applyImmersiveDarkModeAndMica(hwnd, isDark)

	if !restoreWindowState(hwnd, dirName) {
		centerWindow(hwnd, windowWidth, windowHeight)
	}

	exePath, _ := os.Executable()

	// Single Persistent Master Tray Owner
	mutexName, _ := syscall.UTF16PtrFromString("Global\\DrizzleGatewayMasterTray")
	hMutex, _, errMutex := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(mutexName)))
	isMasterTray := hMutex != 0 && (errMutex == nil || errMutex.(syscall.Errno) != ERROR_ALREADY_EXISTS)

	var pNid *NOTIFYICONDATAW
	if isMasterTray {
		nid := setupTrayIcon(hwnd, appTitle, hIcon)
		pNid = &nid
	}

	// Init Window subclass (No white menu bar!)
	initSubclass(hwnd, w, storeDir, pNid, exePath, isMasterTray)

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

		if pNid != nil {
			procShell_NotifyIconW.Call(NIM_DELETE, uintptr(unsafe.Pointer(pNid)))
		}
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

			if targetConnection != "" {
				targetJson, _ := json.Marshal(targetConnection)
				js += fmt.Sprintf(`
					(function() {
						const target = %s;
						let attempts = 0;
						const timer = setInterval(() => {
							attempts++;
							const all = Array.from(document.querySelectorAll('button, div, span, a'));
							const match = all.find(el => el.textContent && el.textContent.trim() === target);
							if (match) {
								match.click();
								clearInterval(timer);
							} else if (attempts > 35) {
								clearInterval(timer);
							}
						}, 150);
					})();
				`, string(targetJson))
			}
			w.Eval(js)
		})
	}()

	w.Run()
	cleanupOnce()
}
