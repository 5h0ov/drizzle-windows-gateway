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
