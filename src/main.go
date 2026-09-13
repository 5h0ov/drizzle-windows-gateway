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

func initSubclass(hwnd uintptr, w webview.WebView, storeDir string, pNid *NOTIFYICONDATAW, exePath, exeBaseName, serverBinName string, isMasterTray bool) {
	callback := syscall.NewCallback(func(h uintptr, msg uint32, wParam uintptr, lParam uintptr) uintptr {
		switch msg {
		case WM_SYSCOMMAND:
			if (wParam & 0xFFF0) == SC_MINIMIZE {
				saveWindowState(h, filepath.Base(filepath.Dir(storeDir)))
				go trimAllGatewayMemory(exeBaseName, serverBinName)
			}
		case WM_CLOSE:
			saveWindowState(h, filepath.Base(filepath.Dir(storeDir)))
			if isMasterTray && !reallyQuit {
				// Master window hides so the single tray launcher stays alive
				procShowWindow.Call(h, SW_HIDE)
				go trimAllGatewayMemory(exeBaseName, serverBinName)
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
				showTrayContextMenu(h, w, storeDir, pNid, exePath, serverBinName)
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
	var isEmptyWindow bool
	for i := 1; i < len(os.Args); i++ {
		if (os.Args[i] == "--connection" || os.Args[i] == "-c") && i+1 < len(os.Args) {
			targetConnection = os.Args[i+1]
			i++
			continue
		}
		if os.Args[i] == "--empty" || os.Args[i] == "--new" {
			isEmptyWindow = true
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
	_ = os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", "--disable-features=Translate,OptimizationHints,MediaRouter --disable-background-networking --disable-component-update --disable-extensions")

	connRawJSON := "null"
	if targetConnection != "" {
		raw := getStoredConnectionRaw(storeDir, targetConnection)
		if raw != "" {
			connRawJSON = raw
		}
	}
	isEmptyStr := "false"
	if isEmptyWindow {
		isEmptyStr = "true"
	}
	themePref := "light"
	if isDark {
		themePref = "dark"
	}

	initScript := fmt.Sprintf(`
(function() {
	// Only execute on actual HTTP pages, ignore data: loading screen
	if (!window.location.href.startsWith('http')) return;

	const initConn = %s;
	const initEmpty = %s;
	const themePref = '%s';

	try {
		if (!localStorage.getItem('theme')) {
			localStorage.setItem('theme', themePref);
		}
	} catch(e) {}

	// If this window was explicitly launched with a target connection or as an empty window:
	if (initConn) {
		try {
			sessionStorage.setItem('drizzle_window_conn', JSON.stringify(initConn));
			sessionStorage.removeItem('drizzle_window_empty');
		} catch(e) {}
	} else if (initEmpty) {
		try {
			sessionStorage.setItem('drizzle_window_empty', 'true');
			sessionStorage.removeItem('drizzle_window_conn');
		} catch(e) {}
	} else if (!sessionStorage.getItem('drizzle_window_conn') && !sessionStorage.getItem('drizzle_window_empty')) {
		// Normal session resume: seed this window's session from localStorage
		try {
			const raw = localStorage.getItem('drizzle-gate');
			if (raw) {
				const parsed = JSON.parse(raw);
				if (parsed && parsed.state) {
					if (parsed.state.currentConnection) {
						sessionStorage.setItem('drizzle_window_conn', JSON.stringify(parsed.state.currentConnection));
					} else {
						sessionStorage.setItem('drizzle_window_empty', 'true');
					}
				}
			}
		} catch(e) {}
	}

	// On every page load or F5 reload, restore THIS window's connection into localStorage
	// before React reads it, preventing cross-window state collision.
	try {
		const winConn = sessionStorage.getItem('drizzle_window_conn');
		const winEmpty = sessionStorage.getItem('drizzle_window_empty');

		if (winEmpty === 'true') {
			const raw = localStorage.getItem('drizzle-gate');
			const parsed = raw ? JSON.parse(raw) : { state: {} };
			if (!parsed.state) parsed.state = {};
			parsed.state.currentConnection = null;
			localStorage.setItem('drizzle-gate', JSON.stringify(parsed));
		} else if (winConn) {
			const connObj = JSON.parse(winConn);
			const raw = localStorage.getItem('drizzle-gate');
			const parsed = raw ? JSON.parse(raw) : { state: {} };
			if (!parsed.state) parsed.state = {};
			parsed.state.currentConnection = connObj;
			localStorage.setItem('drizzle-gate', JSON.stringify(parsed));
		}
	} catch(e) {}

	// Continuously keep this window's sessionStorage in sync with any user switching
	try {
		const origSetItem = localStorage.setItem.bind(localStorage);
		localStorage.setItem = function(key, val) {
			if (key === 'drizzle-gate') {
				try {
					const parsed = JSON.parse(val);
					if (parsed && parsed.state) {
						if (parsed.state.currentConnection) {
							sessionStorage.setItem('drizzle_window_conn', JSON.stringify(parsed.state.currentConnection));
							sessionStorage.removeItem('drizzle_window_empty');
						} else {
							sessionStorage.setItem('drizzle_window_empty', 'true');
							sessionStorage.removeItem('drizzle_window_conn');
						}
					}
				} catch(err) {}
			}
			return origSetItem(key, val);
		};
	} catch(e) {}

	// Component synchronization helper
	const targetObj = initConn || (function() {
		try {
			const s = sessionStorage.getItem('drizzle_window_conn');
			return s ? JSON.parse(s) : null;
		} catch(e) { return null; }
	})();

	const isTargetEmpty = initEmpty || (function() {
		try {
			return sessionStorage.getItem('drizzle_window_empty') === 'true';
		} catch(e) { return false; }
	})();

	if (targetObj) {
		let attempts = 0;
		const timer = setInterval(() => {
			attempts++;
			const ds = document.querySelector('drizzle-studio');
			if (ds && typeof ds.setCurrentConnection === 'function') {
				ds.setCurrentConnection(targetObj);
				clearInterval(timer);
				return;
			}
			const all = Array.from(document.querySelectorAll('button, div, span, a'));
			const match = all.find(el => el.textContent && el.textContent.trim() === targetObj.name);
			if (match) {
				match.click();
				clearInterval(timer);
				return;
			}
			if (attempts > 35) clearInterval(timer);
		}, 150);
	} else if (isTargetEmpty) {
		let attempts = 0;
		const timer = setInterval(() => {
			attempts++;
			const ds = document.querySelector('drizzle-studio');
			if (ds && typeof ds.setCurrentConnection === 'function') {
				ds.setCurrentConnection(null);
				clearInterval(timer);
				return;
			}
			const all = Array.from(document.querySelectorAll('button, div, span, a'));
			const backBtn = all.find(el => el.textContent && el.textContent.trim() === 'Back to connections');
			if (backBtn) {
				backBtn.click();
				clearInterval(timer);
				return;
			}
			if (attempts > 40) clearInterval(timer);
		}, 100);
	}
})();
`, connRawJSON, isEmptyStr, themePref)

	w := webview.New(false)
	defer w.Destroy()

	// Registered on the main UI thread before any navigation
	w.Init(initScript)

	titleWithConn := appTitle
	if targetConnection != "" {
		titleWithConn = fmt.Sprintf("%s - %s", targetConnection, appTitle)
	} else if isEmptyWindow {
		titleWithConn = fmt.Sprintf("Connections - %s", appTitle)
	}
	w.SetTitle(titleWithConn)
	w.SetSize(windowWidth, windowHeight, webview.HintNone)

	hwnd := uintptr(w.Window())
	hIcon := getAppIcon()
	setWindowIcon(hwnd, hIcon)
	applyImmersiveDarkModeAndMica(hwnd, isDark)

	restored, wasMaximized := restoreWindowState(hwnd, dirName)
	if !restored {
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

	serverBin, err := serverBinaryPath(isEnhanced)
	serverBinName := "DrizzleGatewayServer.exe"
	if err == nil {
		serverBinName = filepath.Base(serverBin)
	}

	// Init Window subclass (No white menu bar!)
	initSubclass(hwnd, w, storeDir, pNid, exePath, exeBase, serverBinName, isMasterTray)

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

		if reallyQuit || countGatewayWindows(exeBase) <= 1 {
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

		// Periodic gentle memory trimming (after 4s startup, then every 60s)
		go func() {
			time.Sleep(4 * time.Second)
			trimAllGatewayMemory(exeBase, serverBinName)
			ticker := time.NewTicker(60 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				trimAllGatewayMemory(exeBase, serverBinName)
			}
		}()

		targetURL := fmt.Sprintf("http://127.0.0.1:%d", port)
		w.Dispatch(func() {
			if wasMaximized {
				procShowWindow.Call(hwnd, SW_SHOWMAXIMIZED)
			}
			w.Navigate(targetURL)
		})
	}()

	w.Run()
	cleanupOnce()
}
