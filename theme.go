package main

import (
	"syscall"
	"unsafe"
)

var (
	advapi32DLL          = syscall.NewLazyDLL("advapi32.dll")
	procRegOpenKeyExW    = advapi32DLL.NewProc("RegOpenKeyExW")
	procRegQueryValueExW = advapi32DLL.NewProc("RegQueryValueExW")
	procRegCloseKey      = advapi32DLL.NewProc("RegCloseKey")
)

const (
	hkeyCurrentUser = ^uintptr(0x7FFFFFFE)
	keyRead         = 0x20019
)

func isWindowsLightTheme() bool {
	subkey, _ := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`)
	var hKey uintptr
	ret, _, _ := procRegOpenKeyExW.Call(
		hkeyCurrentUser,
		uintptr(unsafe.Pointer(subkey)),
		0,
		keyRead,
		uintptr(unsafe.Pointer(&hKey)),
	)
	if ret != 0 {
		return true
	}
	defer procRegCloseKey.Call(hKey)

	valueName, _ := syscall.UTF16PtrFromString("SystemUsesLightTheme")
	var buf [4]byte
	var bufLen uint32 = 4
	ret, _, _ = procRegQueryValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(valueName)),
		0,
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&bufLen)),
	)
	if ret != 0 {
		return true
	}
	return buf[0] == 1
}
