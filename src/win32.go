package main

import (
	"strings"
	"syscall"
	"unsafe"
)

const (
	GWLP_WNDPROC     = -4
	WM_CLOSE         = 0x0010
	WM_COMMAND       = 0x0111
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

	TPM_RETURNCMD   = 0x0100
	TPM_RIGHTBUTTON = 0x0002
	MF_STRING       = 0x0000
	MF_SEPARATOR    = 0x0800

	MONITOR_DEFAULTTONULL = 0
	ERROR_ALREADY_EXISTS  = 183
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
	procCreateMutexW             = modKernel32.NewProc("CreateMutexW")

	modUser32               = syscall.NewLazyDLL("user32.dll")
	procGetSystemMetrics    = modUser32.NewProc("GetSystemMetrics")
	procGetWindowPlacement  = modUser32.NewProc("GetWindowPlacement")
	procSetWindowPlacement  = modUser32.NewProc("SetWindowPlacement")
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

	modPsapi            = syscall.NewLazyDLL("psapi.dll")
	procEmptyWorkingSet = modPsapi.NewProc("EmptyWorkingSet")
	procOpenProcess     = modKernel32.NewProc("OpenProcess")
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
type WINDOWPLACEMENT struct {
	Length           uint32
	Flags            uint32
	ShowCmd          uint32
	PtMinPosition    POINT
	PtMaxPosition    POINT
	RcNormalPosition RECT
}


func countGatewayWindows(exeBaseName string) int {
	snap, _, _ := procCreateToolhelp32Snapshot.Call(0x00000002, 0)
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
		procSendMessageW.Call(hwnd, WM_SETICON, 0, hIcon)
		procSendMessageW.Call(hwnd, WM_SETICON, 1, hIcon)
	}
}

func centerWindow(hwnd uintptr, width, height int) {
	screenWidth, _, _ := procGetSystemMetrics.Call(0)
	screenHeight, _, _ := procGetSystemMetrics.Call(1)
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
		0x80000001,
		uintptr(unsafe.Pointer(subKey)),
		0,
		0x20019,
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
	// Dark titlebar
	procDwmSetWindowAttribute.Call(hwnd, 20, uintptr(unsafe.Pointer(&val)), 4)
	procDwmSetWindowAttribute.Call(hwnd, 19, uintptr(unsafe.Pointer(&val)), 4)

	// Mica material (Windows 11)
	var backdrop int32 = 2
	procDwmSetWindowAttribute.Call(hwnd, 38, uintptr(unsafe.Pointer(&backdrop)), 4)
}

const (
	PROCESS_SET_QUOTA         = 0x0100
	PROCESS_QUERY_INFORMATION = 0x0400
)

func trimProcessMemory(pid uint32) {
	hProc, _, _ := procOpenProcess.Call(PROCESS_SET_QUOTA|PROCESS_QUERY_INFORMATION, 0, uintptr(pid))
	if hProc != 0 {
		procEmptyWorkingSet.Call(hProc)
		procCloseHandle.Call(hProc)
	}
}

func trimAllGatewayMemory(exeBaseName, serverBaseName string) {
	currentPid := uint32(syscall.Getpid())
	trimProcessMemory(currentPid)

	snap, _, _ := procCreateToolhelp32Snapshot.Call(0x00000002, 0)
	if snap == uintptr(syscall.InvalidHandle) {
		return
	}
	defer procCloseHandle.Call(snap)

	var entry PROCESSENTRY32W
	entry.DwSize = uint32(unsafe.Sizeof(entry))

	relatedPids := map[uint32]bool{currentPid: true}
	var allEntries []PROCESSENTRY32W

	ret, _, _ := procProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&entry)))
	for ret != 0 {
		allEntries = append(allEntries, entry)
		ret, _, _ = procProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&entry)))
	}

	// Discover child and grandchild WebView2 processes of this gateway
	for pass := 0; pass < 3; pass++ {
		for _, e := range allEntries {
			if relatedPids[e.Th32ParentProcessID] {
				relatedPids[e.Th32ProcessID] = true
			}
		}
	}

	for _, e := range allEntries {
		name := syscall.UTF16ToString(e.SzExeFile[:])
		if strings.EqualFold(name, serverBaseName) || strings.EqualFold(name, exeBaseName) || strings.EqualFold(name, "msedgewebview2.exe") || relatedPids[e.Th32ProcessID] {
			trimProcessMemory(e.Th32ProcessID)
		}
	}
}
