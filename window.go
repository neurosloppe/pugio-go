package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	wsPopupWindow  = 0x80880000
	wsExTopmost    = 0x00000008
	wsExToolWindow = 0x00000080
	wmPaint        = 0x000F
	wmTimer        = 0x0113
	wmWinDestroy   = 0x0002
	idtClose       = 1
	dtCenter       = 0x00000001
	dtVCenter      = 0x00000004
	dtSingleLine   = 0x00000020
	smCxScreen     = 0
	smCyScreen     = 1
	swShow         = 5
)

var (
	procShowWindow       = user32DLL.NewProc("ShowWindow")
	procUpdateWindow     = user32DLL.NewProc("UpdateWindow")
	procSetTimer         = user32DLL.NewProc("SetTimer")
	procKillTimer        = user32DLL.NewProc("KillTimer")
	procDestroyWindow    = user32DLL.NewProc("DestroyWindow")
	procBeginPaint       = user32DLL.NewProc("BeginPaint")
	procEndPaint         = user32DLL.NewProc("EndPaint")
	procCreatePen        = gdi32DLL.NewProc("CreatePen")
	procFillRect         = user32DLL.NewProc("FillRect")
	procRectangle        = gdi32DLL.NewProc("Rectangle")
	procDrawTextW        = user32DLL.NewProc("DrawTextW")
	procGetSystemMetrics = user32DLL.NewProc("GetSystemMetrics")
	procCreateFontW      = gdi32DLL.NewProc("CreateFontW")
	procSetBkMode        = gdi32DLL.NewProc("SetBkMode")
)

type paintStruct struct {
	Hdc        uintptr
	FErase     int32
	RcPaint    rect
	FRestore   int32
	FIncUpdate int32
	RgbReserved [32]byte
}

type rect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

var (
	statusWndClass = syscall.StringToUTF16Ptr("PugioStatusWindow")
	statusHWND     uintptr
	statusBattery  = -1
	statusCharging = false
)

func showStatusWindow(battery int, charging bool) {
	statusBattery = battery
	statusCharging = charging

	if statusHWND != 0 {
		procShowWindow.Call(statusHWND, swShow)
		return
	}

	hInstance := getModuleHandle()

	wndProc := syscall.NewCallback(statusWndProc)

	var wc wndClassExW
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	wc.LpfnWndProc = wndProc
	wc.HInstance = hInstance
	wc.LpszClassName = statusWndClass
	wc.HbrBackground = 1

	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	screenW, _, _ := procGetSystemMetrics.Call(smCxScreen)
	screenH, _, _ := procGetSystemMetrics.Call(smCyScreen)

	var winW int32 = 320
	var winH int32 = 120
	x := int32(screenW) - winW - 10
	y := int32(screenH) - winH - 60

	statusHWND, _, _ = procCreateWindowExW.Call(
		wsExTopmost|wsExToolWindow,
		uintptr(unsafe.Pointer(statusWndClass)),
		0, wsPopupWindow,
		uintptr(x), uintptr(y), uintptr(winW), uintptr(winH),
		0, 0, hInstance, 0,
	)
	if statusHWND == 0 {
		return
	}

	procShowWindow.Call(statusHWND, swShow)
	procUpdateWindow.Call(statusHWND)
	procSetTimer.Call(statusHWND, idtClose, 2500, 0)
}

func closeStatusWindow() {
	if statusHWND != 0 {
		procDestroyWindow.Call(statusHWND)
		statusHWND = 0
	}
}

func statusWndProc(hwnd uintptr, msg uint32, wParam uintptr, lParam uintptr) uintptr {
	switch msg {
	case wmPaint:
		var ps paintStruct
		procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		drawBatteryStatus(ps.Hdc)
		procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case wmTimer:
		if wParam == idtClose {
			procKillTimer.Call(hwnd, idtClose)
			closeStatusWindow()
		}
		return 0
	case wmWinDestroy:
		statusHWND = 0
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func drawBatteryStatus(hdc uintptr) {
	var bgColor, fgColor uintptr
	bgColor = 0x00F0F0F0
	fgColor = 0x00000000

	hBgBrush, _, _ := procCreateSolidBrush.Call(bgColor)
	bgRect := rect{0, 0, 320, 120}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&bgRect)), hBgBrush)
	procDeleteObject.Call(hBgBrush)

	procSetBkMode.Call(hdc, 1)

	fontName, _ := syscall.UTF16PtrFromString("Segoe UI")
	hFont, _, _ := procCreateFontW.Call(
		16, 0, 0, 0,
		700, 0, 0, 0,
		1, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(fontName)),
	)
	oldFont, _, _ := procSelectObject.Call(hdc, hFont)

	titlePtr, _ := syscall.UTF16PtrFromString("Asus ROG Pugio II")
	titleRect := rect{0, 2, 320, 24}
	procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(titlePtr)), ^uintptr(0), uintptr(unsafe.Pointer(&titleRect)), dtCenter|dtSingleLine)

	if statusBattery == -1 {
		discPtr, _ := syscall.UTF16PtrFromString("Not connected")
		discRect := rect{0, 30, 320, 90}
		procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(discPtr)), ^uintptr(0), uintptr(unsafe.Pointer(&discRect)), dtCenter|dtSingleLine|dtVCenter)
	} else {
		hPen, _, _ := procCreatePen.Call(0, 2, fgColor)
		oldPen, _, _ := procSelectObject.Call(hdc, hPen)

		procRectangle.Call(hdc, 85, 25, 205, 55)
		procRectangle.Call(hdc, 205, 32, 210, 48)

		var fillColor uintptr
		if statusBattery >= 50 {
			fillColor = 0x0000CC00
		} else if statusBattery >= 20 {
			fillColor = 0x000080FF
		} else {
			fillColor = 0x000000FF
		}

		hFillBrush, _, _ := procCreateSolidBrush.Call(fillColor)
		procSelectObject.Call(hdc, hFillBrush)
		fillWidth := int32(float64(statusBattery) / 100.0 * 116)
		fillRect := rect{87, 27, 87 + fillWidth, 53}
		procFillRect.Call(hdc, uintptr(unsafe.Pointer(&fillRect)), hFillBrush)
		procDeleteObject.Call(hFillBrush)

		procSelectObject.Call(hdc, oldPen)
		procDeleteObject.Call(hPen)

		procSelectObject.Call(hdc, hFont)
		percentText, _ := syscall.UTF16PtrFromString(fmt.Sprintf("%d%%", statusBattery))
		percentRect := rect{212, 25, 300, 55}
		procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(percentText)), ^uintptr(0), uintptr(unsafe.Pointer(&percentRect)), dtVCenter|dtSingleLine)

		if statusCharging {
			chargePtr, _ := syscall.UTF16PtrFromString("Charging")
			chargeRect := rect{85, 60, 210, 80}
			procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(chargePtr)), ^uintptr(0), uintptr(unsafe.Pointer(&chargeRect)), dtCenter|dtSingleLine)
		}
	}

	procSelectObject.Call(hdc, oldFont)
	procDeleteObject.Call(hFont)
}
