package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"unsafe"
)

type WindowState struct {
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	Maximized bool `json:"maximized"`
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
	if hwnd == 0 {
		return
	}
	var wp WINDOWPLACEMENT
	wp.Length = uint32(unsafe.Sizeof(wp))
	ret, _, _ := procGetWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(&wp)))
	if ret == 0 {
		return
	}

	isZoom, _, _ := procIsZoomed.Call(hwnd)
	isMaximized := isZoom != 0 || wp.ShowCmd == SW_SHOWMAXIMIZED

	width := int(wp.RcNormalPosition.Right - wp.RcNormalPosition.Left)
	height := int(wp.RcNormalPosition.Bottom - wp.RcNormalPosition.Top)
	x := int(wp.RcNormalPosition.Left)
	y := int(wp.RcNormalPosition.Top)

	// Fallback to reasonable defaults if normal position isn't valid yet
	if width < 400 || height < 300 {
		width = windowWidth
		height = windowHeight
	}

	ws := WindowState{
		X:         x,
		Y:         y,
		Width:     width,
		Height:    height,
		Maximized: isMaximized,
	}

	data, err := json.MarshalIndent(ws, "", "  ")
	if err == nil {
		_ = os.WriteFile(getWindowStatePath(dirName), data, 0o644)
	}
}

func restoreWindowState(hwnd uintptr, dirName string) (bool, bool) {
	data, err := os.ReadFile(getWindowStatePath(dirName))
	if err != nil {
		return false, false
	}
	var ws WindowState
	if err := json.Unmarshal(data, &ws); err != nil {
		return false, false
	}
	if ws.Width < 400 || ws.Height < 300 {
		return false, false
	}

	r := RECT{
		Left:   int32(ws.X),
		Top:    int32(ws.Y),
		Right:  int32(ws.X + ws.Width),
		Bottom: int32(ws.Y + ws.Height),
	}

	// 2 = MONITOR_DEFAULTTONEAREST ensures monitor detection even if off slightly
	hMon, _, _ := procMonitorFromRect.Call(uintptr(unsafe.Pointer(&r)), 2)
	if hMon == 0 {
		return false, false
	}

	var wp WINDOWPLACEMENT
	wp.Length = uint32(unsafe.Sizeof(wp))
	wp.RcNormalPosition = r
	if ws.Maximized {
		wp.ShowCmd = SW_SHOWMAXIMIZED
	} else {
		wp.ShowCmd = SW_SHOWNORMAL
	}
	procSetWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(&wp)))

	if ws.Maximized {
		procShowWindow.Call(hwnd, SW_SHOWMAXIMIZED)
	}

	return true, ws.Maximized
}
