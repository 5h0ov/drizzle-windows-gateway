package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"

	webview "github.com/webview/webview_go"
)

const (
	ID_TRAY_NEW  = 1001
	ID_TRAY_QUIT = 1002
	ID_CONN_BASE = 2000
)

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

func showTrayContextMenu(hwnd uintptr, w webview.WebView, storeDir string, pNid *NOTIFYICONDATAW, exePath, serverBinName string) {
	hMenu, _, _ := procCreatePopupMenu.Call()
	if hMenu == 0 {
		return
	}
	defer procDestroyMenu.Call(hMenu)

	// List stored database connections cleanly without bullet symbols
	conns := getStoredConnections(storeDir)
	if len(conns) > 0 {
		for i, c := range conns {
			label := fmt.Sprintf("%s [%s]", c.Name, formatDialect(c.Dialect))
			strPtr, _ := syscall.UTF16PtrFromString(label)
			procAppendMenuW.Call(hMenu, MF_STRING, uintptr(ID_CONN_BASE+i), uintptr(unsafe.Pointer(strPtr)))
		}
		procAppendMenuW.Call(hMenu, MF_SEPARATOR, 0, 0)
	}

	newWinStr, _ := syscall.UTF16PtrFromString("New Empty Window")
	quitStr, _ := syscall.UTF16PtrFromString("Quit Drizzle Gateway")

	procAppendMenuW.Call(hMenu, MF_STRING, ID_TRAY_NEW, uintptr(unsafe.Pointer(newWinStr)))
	procAppendMenuW.Call(hMenu, MF_SEPARATOR, 0, 0)
	procAppendMenuW.Call(hMenu, MF_STRING, ID_TRAY_QUIT, uintptr(unsafe.Pointer(quitStr)))

	// Create a temporary hidden dummy window so TrackPopupMenu captures focus and dismisses
	// properly without raising or activating the main Drizzle window when it is in the background.
	staticClass, _ := syscall.UTF16PtrFromString("STATIC")
	hDummy, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(staticClass)),
		0,
		0,
		0, 0, 0, 0,
		0, 0, 0, 0,
	)
	menuOwner := hwnd
	if hDummy != 0 {
		defer procDestroyWindow.Call(hDummy)
		menuOwner = hDummy
	}

	var pt POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForegroundWindow.Call(menuOwner)

	cmd, _, _ := procTrackPopupMenu.Call(
		hMenu,
		TPM_RETURNCMD|TPM_RIGHTBUTTON,
		uintptr(pt.X),
		uintptr(pt.Y),
		0,
		menuOwner,
		0,
	)
	procPostMessageW.Call(menuOwner, 0, 0, 0)

	if cmd == ID_TRAY_NEW {
		_ = exec.Command(exePath, "--empty").Start()
	} else if cmd == ID_TRAY_QUIT {
		procShell_NotifyIconW.Call(NIM_DELETE, uintptr(unsafe.Pointer(pNid)))
		saveWindowState(hwnd, filepath.Base(filepath.Dir(storeDir)))
		reallyQuit = true
		closeAllGatewayInstances(filepath.Base(exePath), serverBinName)
		w.Terminate()
	} else if cmd >= ID_CONN_BASE && int(cmd-ID_CONN_BASE) < len(conns) {
		selectedConn := conns[cmd-ID_CONN_BASE]
		_ = exec.Command(exePath, "--connection", selectedConn.Name).Start()
	}
}
