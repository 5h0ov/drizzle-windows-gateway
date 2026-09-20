
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"
	"time"

	"rsc.io/qr"
	webview "github.com/webview/webview_go"
)

type MobileState struct {
	Running     bool   `json:"running"`
	Port        int    `json:"port"`
	Token       string `json:"token"`
	WifiIP      string `json:"wifi_ip"`
	TailscaleIP string `json:"tailscale_ip"`
}

var (
	globalMobileListener net.Listener
	globalMobileState    MobileState
	mobileMutex          sync.Mutex
)

func getLocalIPs() (wifiIP string, tailscaleIP string) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", ""
	}

	for _, iface := range ifaces {
		if (iface.Flags & net.FlagUp) == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP.IsLoopback() {
				continue
			}
			ip4 := ipNet.IP.To4()
			if ip4 == nil {
				continue
			}
			ipStr := ip4.String()

			if strings.EqualFold(iface.Name, "Tailscale") || strings.HasPrefix(ipStr, "100.") {
				tailscaleIP = ipStr
			} else if strings.HasPrefix(ipStr, "192.168.") || strings.HasPrefix(ipStr, "10.") || strings.HasPrefix(ipStr, "172.") {
				if wifiIP == "" && !strings.Contains(strings.ToLower(iface.Name), "virtual") && !strings.Contains(strings.ToLower(iface.Name), "vethernet") {
					wifiIP = ipStr
				}
			}
		}
	}
	return wifiIP, tailscaleIP
}

func getOrInitMobileToken(storeDir string) string {
	tokenFile := filepath.Join(storeDir, "mobile_token.txt")
	if b, err := os.ReadFile(tokenFile); err == nil {
		t := strings.TrimSpace(string(b))
		if len(t) == 32 {
			return t
		}
	}
	newToken := generateSecureToken()
	_ = os.WriteFile(tokenFile, []byte(newToken), 0o600)
	return newToken
}

func generateSecureToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func getMobileStatePath(storeDir string) string {
	return filepath.Join(storeDir, "mobile_state.json")
}

func saveMobileState(storeDir string, s MobileState) {
	b, _ := json.MarshalIndent(s, "", "  ")
	_ = os.WriteFile(getMobileStatePath(storeDir), b, 0o600)
}

func getMobileState(storeDir string) MobileState {
	b, err := os.ReadFile(getMobileStatePath(storeDir))
	if err == nil {
		var s MobileState
		if json.Unmarshal(b, &s) == nil {
			return s
		}
	}
	wifi, tail := getLocalIPs()
	token := getOrInitMobileToken(storeDir)
	return MobileState{
		Running:     false,
		Port:        4984,
		Token:       token,
		WifiIP:      wifi,
		TailscaleIP: tail,
	}
}

func startMobileProxy(localPort, preferredPort int, storeDir string) (MobileState, error) {
	mobileMutex.Lock()
	defer mobileMutex.Unlock()

	if globalMobileListener != nil {
		return globalMobileState, nil
	}

	token := getOrInitMobileToken(storeDir)
	wifiIP, tailscaleIP := getLocalIPs()

	var ln net.Listener
	var err error
	boundPort := preferredPort

	for p := preferredPort; p < preferredPort+20; p++ {
		ln, err = net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", p))
		if err == nil {
			boundPort = p
			break
		}
	}

	if ln == nil {
		return MobileState{}, fmt.Errorf("could not find available port for mobile access: %v", err)
	}

	globalMobileListener = ln
	globalMobileState = MobileState{
		Running:     true,
		Port:        boundPort,
		Token:       token,
		WifiIP:      wifiIP,
		TailscaleIP: tailscaleIP,
	}
	saveMobileState(storeDir, globalMobileState)

	// Auto-activate Tailscale HTTPS in background if available
	go func() {
		_, _ = getTailscaleAccess(tailscaleIP, boundPort, token)
	}()

	targetURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", localPort))
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Custom response modifier for PWA meta tags injection
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = targetURL.Host
	}

	proxy.ModifyResponse = func(resp *http.Response) error {
		ct := resp.Header.Get("Content-Type")
		if strings.Contains(ct, "text/html") {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			resp.Body.Close()

			headInject := `
  <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no, viewport-fit=cover">
  <meta name="apple-mobile-web-app-capable" content="yes">
  <meta name="mobile-web-app-capable" content="yes">
  <meta name="apple-mobile-web-app-status-bar-style" content="black-translucent">
  <meta name="theme-color" content="#09090b">
  <link rel="manifest" href="/manifest.webmanifest">
  <script>
    if ('serviceWorker' in navigator) {
      navigator.serviceWorker.register('/sw.js').catch(function(){});
    }
  </script>
  <style>
    /* Mobile responsive optimizations */
    body { -webkit-tap-highlight-color: transparent; }
    ::-webkit-scrollbar { width: 4px; height: 4px; }
  </style>
</head>`
			newHtml := bytes.Replace(body, []byte("</head>"), []byte(headInject), 1)
			resp.Body = io.NopCloser(bytes.NewReader(newHtml))
			resp.ContentLength = int64(len(newHtml))
			resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(newHtml)))
		}
		return nil
	}

	mux := http.NewServeMux()

	// 1. Service Worker & PWA Manifest
	mux.HandleFunc("/sw.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Write([]byte("self.addEventListener('install', (e) => self.skipWaiting());\nself.addEventListener('activate', (e) => e.waitUntil(clients.claim()));\nself.addEventListener('fetch', (e) => e.respondWith(fetch(e.request)));"))
	})

	mux.HandleFunc("/icon-192.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Write(generateIconPNG(192))
	})

	mux.HandleFunc("/icon-512.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Write(generateIconPNG(512))
	})

	mux.HandleFunc("/manifest.webmanifest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/manifest+json")
		manifest := `{
  "name": "Drizzle Studio",
  "short_name": "Drizzle",
  "start_url": "/",
  "scope": "/",
  "display": "standalone",
  "background_color": "#09090b",
  "theme_color": "#09090b",
  "icons": [
    {
      "src": "/icon-192.png",
      "sizes": "192x192",
      "type": "image/png",
      "purpose": "any maskable"
    },
    {
      "src": "/icon-512.png",
      "sizes": "512x512",
      "type": "image/png",
      "purpose": "any maskable"
    }
  ]
}`
		w.Write([]byte(manifest))
	})

	// 2. Control endpoints
	mux.HandleFunc("/__mobile/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(globalMobileState)
		w.Write(b)
	})

	mux.HandleFunc("/__mobile/regenerate", func(w http.ResponseWriter, r *http.Request) {
		newToken := generateSecureToken()
		tokenFile := filepath.Join(storeDir, "mobile_token.txt")
		_ = os.WriteFile(tokenFile, []byte(newToken), 0o600)
		globalMobileState.Token = newToken
		saveMobileState(storeDir, globalMobileState)
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(globalMobileState)
		w.Write(b)
	})

	// 3. Proxy with authentication
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Check auth query param: ?auth=<token>
		qAuth := r.URL.Query().Get("auth")
		if qAuth != "" {
			if qAuth == globalMobileState.Token {
				// Set persistent auth cookie and redirect cleanly to strip token from URL
				http.SetCookie(w, &http.Cookie{
					Name:     "drizzle_auth",
					Value:    globalMobileState.Token,
					Path:     "/",
					MaxAge:   31536000,
					SameSite: http.SameSiteLaxMode,
				})
				// Strip auth query param
				cleanURL := r.URL
				q := cleanURL.Query()
				q.Del("auth")
				cleanURL.RawQuery = q.Encode()
				targetPath := cleanURL.RequestURI()
				if targetPath == "" {
					targetPath = "/"
				}
				http.Redirect(w, r, targetPath, http.StatusFound)
				return
			}
		}

		// Check cookie auth
		cookie, err := r.Cookie("drizzle_auth")
		if err == nil && cookie.Value == globalMobileState.Token {
			proxy.ServeHTTP(w, r)
			return
		}

		// Allow favicon and static manifest through without auth
		if r.URL.Path == "/favicon.svg" || r.URL.Path == "/manifest.webmanifest" {
			proxy.ServeHTTP(w, r)
			return
		}

		// Unauthenticated response
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Drizzle Gateway - Authentication Required</title>
  <style>
    body {
      margin: 0;
      background: #09090b;
      color: #f4f4f5;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 100vh;
      text-align: center;
      padding: 20px;
      box-sizing: border-box;
    }
    .card {
      background: #18181b;
      border: 1px solid #27272a;
      border-radius: 12px;
      padding: 30px 24px;
      max-width: 380px;
    }
    h2 { font-size: 18px; margin-bottom: 8px; color: #fafafa; }
    p { font-size: 13px; color: #a1a1aa; line-height: 1.5; }
    .badge {
      display: inline-block;
      background: rgba(197, 247, 79, 0.15);
      color: #c5f74f;
      padding: 4px 10px;
      border-radius: 6px;
      font-size: 12px;
      margin-top: 14px;
      font-weight: 500;
    }
  </style>
</head>
<body>
  <div class="card">
    <h2>Authentication Required</h2>
    <p>Please scan the pairing QR code from the Drizzle Gateway desktop system tray on your computer to access this session.</p>
    <div class="badge">Drizzle Gateway Mobile</div>
  </div>
</body>
</html>`))
	})

	server := &http.Server{Handler: mux}

	go func() {
		_ = server.Serve(ln)
	}()

	return globalMobileState, nil
}

var (
	tailscaleServeActive bool
	tailscaleServeMu     sync.Mutex
)

func getTailscaleAccess(tailscaleIP string, port int, token string) (string, bool) {
	if tailscaleIP == "" {
		return "", false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "tailscale", "status", "--json")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return fmt.Sprintf("http://%s:%d/?auth=%s", tailscaleIP, port, token), false
	}

	var status struct {
		BackendState string `json:"BackendState"`
		Self         struct {
			DNSName string `json:"DNSName"`
		} `json:"Self"`
	}
	if err := json.Unmarshal(out, &status); err != nil || status.BackendState != "Running" {
		return fmt.Sprintf("http://%s:%d/?auth=%s", tailscaleIP, port, token), false
	}

	dnsHost := strings.TrimSuffix(status.Self.DNSName, ".")
	if dnsHost == "" {
		return fmt.Sprintf("http://%s:%d/?auth=%s", tailscaleIP, port, token), false
	}

	// Try checking if tailscale serve is active or can be enabled
	serveCtx, serveCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer serveCancel()

	serveCmd := exec.CommandContext(serveCtx, "tailscale", "serve", "--bg", fmt.Sprintf("%d", port))
	serveCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := serveCmd.Run(); err == nil {
		tailscaleServeMu.Lock()
		tailscaleServeActive = true
		tailscaleServeMu.Unlock()
		return fmt.Sprintf("https://%s/?auth=%s", dnsHost, token), true
	}

	return fmt.Sprintf("http://%s:%d/?auth=%s", tailscaleIP, port, token), false
}

func stopTailscaleServe() {
	tailscaleServeMu.Lock()
	defer tailscaleServeMu.Unlock()
	if tailscaleServeActive {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "tailscale", "serve", "--https=443", "off")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_ = cmd.Run()
		tailscaleServeActive = false
	}
}

func cleanupMobileProxy() {
	mobileMutex.Lock()
	defer mobileMutex.Unlock()

	stopTailscaleServe()

	if globalMobileListener != nil {
		_ = globalMobileListener.Close()
		globalMobileListener = nil
	}
}

func stopMobileProxy(storeDir string) {
	mobileMutex.Lock()
	defer mobileMutex.Unlock()

	stopTailscaleServe()

	if globalMobileListener != nil {
		_ = globalMobileListener.Close()
		globalMobileListener = nil
	}

	globalMobileState.Running = false
	saveMobileState(storeDir, globalMobileState)
}

func generateQRSVG(content string) string {
	if content == "" {
		return `<div style="color:#71717a;font-size:12px;text-align:center;padding:40px 10px;">Address not detected</div>`
	}
	code, err := qr.Encode(content, qr.M)
	if err != nil {
		return `<div style="color:#ef4444;font-size:12px;text-align:center;padding:40px 10px;">QR generation error</div>`
	}

	size := code.Size
	quietZone := 2
	totalSize := size + quietZone*2

	var path strings.Builder
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if code.Black(x, y) {
				fmt.Fprintf(&path, "M%d %dh1v1h-1z ", x+quietZone, y+quietZone)
			}
		}
	}

	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges" style="width:100%%;height:100%%;display:block;"><rect width="100%%" height="100%%" fill="#ffffff"/><path d="%s" fill="#09090b"/></svg>`, totalSize, totalSize, path.String())
}


var (
	modUser32Clipboard    = syscall.NewLazyDLL("user32.dll")
	procOpenClipboard     = modUser32Clipboard.NewProc("OpenClipboard")
	procCloseClipboard    = modUser32Clipboard.NewProc("CloseClipboard")
	procEmptyClipboard    = modUser32Clipboard.NewProc("EmptyClipboard")
	procSetClipboardData  = modUser32Clipboard.NewProc("SetClipboardData")
	procGlobalAlloc       = modKernel32.NewProc("GlobalAlloc")
	procGlobalLock        = modKernel32.NewProc("GlobalLock")
	procGlobalUnlock      = modKernel32.NewProc("GlobalUnlock")
)

func setClipboardText(text string) error {
	utf16Chars, err := syscall.UTF16FromString(text)
	if err != nil {
		return err
	}
	r1, _, _ := procOpenClipboard.Call(0)
	if r1 == 0 {
		return fmt.Errorf("OpenClipboard failed")
	}
	defer procCloseClipboard.Call()

	procEmptyClipboard.Call()

	bytesCount := uintptr(len(utf16Chars) * 2)
	const GMEM_MOVEABLE = 0x0002
	hMem, _, _ := procGlobalAlloc.Call(GMEM_MOVEABLE, bytesCount)
	if hMem == 0 {
		return fmt.Errorf("GlobalAlloc failed")
	}

	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		return fmt.Errorf("GlobalLock failed")
	}

	dst := unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(utf16Chars))
	copy(dst, utf16Chars)
	procGlobalUnlock.Call(hMem)

	const CF_UNICODETEXT = 13
	procSetClipboardData.Call(CF_UNICODETEXT, hMem)
	return nil
}

func runMobilePairingWindow(appTitle, dirName string, localPort int, isEnhanced bool) {
	storeDir := appDataDir(dirName)
	state := getMobileState(storeDir)

	// Ensure proxy is running
	if !state.Running {
		newState, err := startMobileProxy(localPort, 4984, storeDir)
		if err == nil {
			state = newState
		}
	}

	tailscaleUrl, isHTTPS := getTailscaleAccess(state.TailscaleIP, state.Port, state.Token)

	wifiUrl := ""
	if state.WifiIP != "" {
		wifiUrl = fmt.Sprintf("http://%s:%d/?auth=%s", state.WifiIP, state.Port, state.Token)
	}

	tailscaleSvg := generateQRSVG(tailscaleUrl)
	wifiSvg := generateQRSVG(wifiUrl)

	html := mobilePairHTMLTemplate
	html = strings.ReplaceAll(html, "{{TAILSCALE_QR}}", tailscaleSvg)
	html = strings.ReplaceAll(html, "{{WIFI_QR}}", wifiSvg)
	html = strings.ReplaceAll(html, "{{TAILSCALE_URL}}", tailscaleUrl)
	html = strings.ReplaceAll(html, "{{WIFI_URL}}", wifiUrl)
	html = strings.ReplaceAll(html, "{{PORT}}", fmt.Sprintf("%d", state.Port))
	isHttpsStr := "false"
	if isHTTPS {
		isHttpsStr = "true"
	}
	html = strings.ReplaceAll(html, "{{IS_HTTPS}}", isHttpsStr)

	w := webview.New(false)
	defer w.Destroy()

	_ = w.Bind("copyToClipboard", func(text string) error {
		return setClipboardText(text)
	})

	_ = w.Bind("stopMobileServer", func() error {
		stopMobileProxy(storeDir)
		return nil
	})

	_ = w.Bind("startMobileServer", func() error {
		_, err := startMobileProxy(localPort, 4984, storeDir)
		return err
	})

	_ = w.Bind("closeMobileWindow", func() error {
		w.Terminate()
		return nil
	})

	w.SetTitle(fmt.Sprintf("Mobile Access - %s", appTitle))
	w.SetSize(460, 680, webview.HintNone)

	hwnd := uintptr(w.Window())
	applyImmersiveDarkModeAndMica(hwnd, true)
	hIcon := getAppIcon()
	if hIcon != 0 {
		setWindowIcon(hwnd, hIcon)
	}
	centerWindow(hwnd, 460, 680)

	w.SetHtml(html)
	w.Run()
}

func generateIconPNG(size int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	bgColor := color.RGBA{R: 0x17, G: 0x16, B: 0x1B, A: 0xFF}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, bgColor)
		}
	}

	accentColor := color.RGBA{R: 0xC5, G: 0xF7, B: 0x4F, A: 0xFF}
	w := size / 10
	h := size / 3
	startX := size / 4
	startY := size / 3
	for i := 0; i < 3; i++ {
		bx := startX + i*(w*2)
		by := startY - i*(w/2)
		for y := by; y < by+h && y < size; y++ {
			for x := bx; x < bx+w && x < size; x++ {
				img.Set(x, y, accentColor)
			}
		}
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
