package main

import (
	"syscall"
	"unsafe"
)

const errorAlreadyExists = 183

var (
	procCreateMutexW = kernel32DLL.NewProc("CreateMutexW")
	procGetLastError = kernel32DLL.NewProc("GetLastError")
	procCloseHandle  = kernel32DLL.NewProc("CloseHandle")
)

var hMutex uintptr

func acquireLock() bool {
	name, _ := syscall.UTF16PtrFromString("Global\\ROGPugioIIBatteryStatus")
	ret, _, _ := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if ret == 0 {
		return false
	}
	hMutex = ret
	errCode, _, _ := procGetLastError.Call()
	if errCode == errorAlreadyExists {
		procCloseHandle.Call(hMutex)
		hMutex = 0
		return false
	}
	return true
}

func releaseLock() {
	if hMutex != 0 {
		procCloseHandle.Call(hMutex)
		hMutex = 0
	}
}
