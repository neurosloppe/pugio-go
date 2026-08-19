package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	wmUser         = 0x0400
	wmTrayIcon     = wmUser + 1
	wmAppUpdate    = 0x8001
	wmCommand      = 0x0111
	wmDestroy      = 0x0002
	wmNull         = 0x0000
	idShowStatus   = 1001
	idQuit         = 1003
	nifMessage     = 0x00000001
	nifIcon        = 0x00000002
	nifTip         = 0x00000004
	nimAdd         = 0x00000000
	nimModify      = 0x00000001
	nimDelete      = 0x00000002
	wsPopup        = 0x80000000
	tpmRightButton = 0x0008
	mfString       = 0x00000000
	mfSeparator    = 0x00000800
	wmLButtonUp    = 0x0202
	wmLButtonDblClk = 0x0203
	wmRButtonUp    = 0x0204
)

var (
	shell32DLL = syscall.NewLazyDLL("shell32.dll")

	procShell_NotifyIconW = shell32DLL.NewProc("Shell_NotifyIconW")
	procCreatePopupMenu  = user32DLL.NewProc("CreatePopupMenu")
	procAppendMenuW      = user32DLL.NewProc("AppendMenuW")
	procTrackPopupMenu   = user32DLL.NewProc("TrackPopupMenu")
	procGetCursorPos     = user32DLL.NewProc("GetCursorPos")
	procSetForegroundWindow = user32DLL.NewProc("SetForegroundWindow")
	procRegisterClassExW = user32DLL.NewProc("RegisterClassExW")
	procCreateWindowExW  = user32DLL.NewProc("CreateWindowExW")
	procDefWindowProcW   = user32DLL.NewProc("DefWindowProcW")
	procDestroyMenu      = user32DLL.NewProc("DestroyMenu")
	procPostMessageW     = user32DLL.NewProc("PostMessageW")
	procGetModuleHandleW = kernel32DLL.NewProc("GetModuleHandleW")

	procCreateCompatibleDC     = gdi32DLL.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32DLL.NewProc("CreateCompatibleBitmap")
	procSelectObject           = gdi32DLL.NewProc("SelectObject")
	procDeleteObject           = gdi32DLL.NewProc("DeleteObject")
	procDeleteDC               = gdi32DLL.NewProc("DeleteDC")
	procCreateSolidBrush       = gdi32DLL.NewProc("CreateSolidBrush")
	procEllipse                = gdi32DLL.NewProc("Ellipse")
	procCreateIconIndirect     = user32DLL.NewProc("CreateIconIndirect")
	procGetDC                  = user32DLL.NewProc("GetDC")
	procReleaseDC             = user32DLL.NewProc("ReleaseDC")
)

type notifyIconDataW struct {
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
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         [16]byte
	HBalloonIcon     uintptr
}

type pt struct {
	X int32
	Y int32
}

type iconInfo struct {
	FIcon    int32
	XHotspot int32
	YHotspot int32
	HbmMask  uintptr
	HbmColor uintptr
}

type wndClassExW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

var (
	trayWndClass    = syscall.StringToUTF16Ptr("PugioTrayWindow")
	trayHWND        uintptr
	trayIconData    notifyIconDataW
	currentBattery  = -1
	currentCharging = false
)

func getModuleHandle() uintptr {
	h, _, _ := procGetModuleHandleW.Call(0)
	return h
}

func initTray() {
	hInstance := getModuleHandle()

	wndProc := syscall.NewCallback(trayWndProc)

	var wc wndClassExW
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	wc.LpfnWndProc = wndProc
	wc.HInstance = hInstance
	wc.LpszClassName = trayWndClass
	wc.HbrBackground = 1

	ret, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if ret == 0 {
		fmt.Printf("RegisterClassExW failed: %v\n", err)
		return
	}

	trayHWND, _, err = procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(trayWndClass)),
		0, wsPopup,
		0, 0, 0, 0,
		0, 0, hInstance, 0,
	)
	if trayHWND == 0 {
		fmt.Printf("CreateWindowExW failed: %v\n", err)
		return
	}

	addTrayIcon()
	updateTrayIcon()
}

func createCircleIcon(r, g, b byte) uintptr {
	size := 16

	hdcScreen, _, _ := procGetDC.Call(0)
	defer procReleaseDC.Call(0, hdcScreen)

	hdcMem, _, _ := procCreateCompatibleDC.Call(hdcScreen)
	hBitmap, _, _ := procCreateCompatibleBitmap.Call(hdcScreen, uintptr(size), uintptr(size))
	oldBmp, _, _ := procSelectObject.Call(hdcMem, hBitmap)

	bgColor := uintptr(0x00FFFFFF)
	bgBrush, _, _ := procCreateSolidBrush.Call(bgColor)
	oldBrush, _, _ := procSelectObject.Call(hdcMem, bgBrush)
	procEllipse.Call(hdcMem, 0, 0, uintptr(size), uintptr(size))
	procSelectObject.Call(hdcMem, oldBrush)
	procDeleteObject.Call(bgBrush)

	color := uintptr(r) | (uintptr(g) << 8) | (uintptr(b) << 16)
	fillBrush, _, _ := procCreateSolidBrush.Call(color)
	oldBrush2, _, _ := procSelectObject.Call(hdcMem, fillBrush)
	procEllipse.Call(hdcMem, 1, 1, uintptr(size-1), uintptr(size-1))
	procSelectObject.Call(hdcMem, oldBrush2)
	procDeleteObject.Call(fillBrush)

	procSelectObject.Call(hdcMem, oldBmp)

	var ii iconInfo
	ii.FIcon = 1
	ii.HbmMask = hBitmap
	ii.HbmColor = hBitmap
	hIcon, _, _ := procCreateIconIndirect.Call(uintptr(unsafe.Pointer(&ii)))

	procDeleteObject.Call(hBitmap)
	procDeleteDC.Call(hdcMem)
	procReleaseDC.Call(0, hdcScreen)

	return hIcon
}

func getIconForBattery(battery int, charging bool) uintptr {
	if battery == -1 {
		return createCircleIcon(128, 128, 128)
	}
	if charging {
		return createCircleIcon(255, 215, 0)
	}
	if battery >= 75 {
		return createCircleIcon(0, 180, 0)
	}
	if battery >= 25 {
		return createCircleIcon(255, 165, 0)
	}
	return createCircleIcon(255, 0, 0)
}

func addTrayIcon() {
	trayIconData.CbSize = uint32(unsafe.Sizeof(trayIconData))
	trayIconData.HWnd = trayHWND
	trayIconData.UID = 1
	trayIconData.UFlags = nifMessage | nifIcon | nifTip
	trayIconData.UCallbackMessage = wmTrayIcon

	copy(trayIconData.SzTip[:], syscall.StringToUTF16("ROG Pugio II"))
	trayIconData.HIcon = getIconForBattery(-1, false)

	ret, _, err := procShell_NotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&trayIconData)))
	if ret == 0 {
		fmt.Printf("Shell_NotifyIconW (NIM_ADD) failed: %v\n", err)
	} else {
		fmt.Println("Tray icon added successfully.")
	}
}

func updateTrayIcon() {
	battery, charging := getBatteryInfo()
	currentBattery = battery
	currentCharging = charging

	var tip string
	if battery == -1 {
		tip = "Not connected"
	} else if charging {
		tip = fmt.Sprintf("Battery: %d%% (charging)", battery)
	} else {
		tip = fmt.Sprintf("Battery: %d%%", battery)
	}

	copy(trayIconData.SzTip[:], syscall.StringToUTF16(tip))

	oldIcon := trayIconData.HIcon
	trayIconData.HIcon = getIconForBattery(battery, charging)
	trayIconData.UFlags = nifIcon | nifTip
	procShell_NotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&trayIconData)))

	if oldIcon != 0 {
		procDeleteObject.Call(oldIcon)
	}
}

func removeTrayIcon() {
	procShell_NotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&trayIconData)))
	if trayIconData.HIcon != 0 {
		procDeleteObject.Call(trayIconData.HIcon)
		trayIconData.HIcon = 0
	}
}

func showContextMenu() {
	hMenu, _, _ := procCreatePopupMenu.Call()

	text1, _ := syscall.UTF16PtrFromString("Show Battery")
	procAppendMenuW.Call(hMenu, mfString, idShowStatus, uintptr(unsafe.Pointer(text1)))

	procAppendMenuW.Call(hMenu, mfSeparator, 0, 0)

	text3, _ := syscall.UTF16PtrFromString("Exit")
	procAppendMenuW.Call(hMenu, mfString, idQuit, uintptr(unsafe.Pointer(text3)))

	var p pt
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))

	procSetForegroundWindow.Call(trayHWND)

	procTrackPopupMenu.Call(
		hMenu,
		tpmRightButton,
		uintptr(p.X),
		uintptr(p.Y),
		0, trayHWND, 0,
	)

	procPostMessageW.Call(trayHWND, wmNull, 0, 0)
	procDestroyMenu.Call(hMenu)
}

func trayWndProc(hwnd uintptr, msg uint32, wParam uintptr, lParam uintptr) uintptr {
	switch msg {
	case wmTrayIcon:
		switch lParam {
		case wmRButtonUp:
			showContextMenu()
			return 0
		case wmLButtonUp, wmLButtonDblClk:
			showStatusWindow(currentBattery, currentCharging)
			return 0
		}
	case wmAppUpdate:
		updateTrayIcon()
		return 0
	case wmCommand:
		switch wParam & 0xFFFF {
		case idShowStatus:
			showStatusWindow(currentBattery, currentCharging)
			return 0
		case idQuit:
			removeTrayIcon()
			procPostQuitMessage.Call(0)
			return 0
		}
	case wmDestroy:
		removeTrayIcon()
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}
