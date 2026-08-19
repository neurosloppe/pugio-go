package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"time"
	"unsafe"
)

var (
	procGetMessageW      = user32DLL.NewProc("GetMessageW")
	procTranslateMessage = user32DLL.NewProc("TranslateMessage")
	procDispatchMessageW = user32DLL.NewProc("DispatchMessageW")
	procPostQuitMessage  = user32DLL.NewProc("PostQuitMessage")
)

type msg struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      pt
}

func main() {
	runtime.LockOSThread()

	if !acquireLock() {
		fmt.Println("Another instance is already running.")
		return
	}
	defer releaseLock()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		<-sig
		fmt.Println("Ctrl+C received, exiting...")
		if trayHWND != 0 {
			procPostMessageW.Call(trayHWND, wmCommand, idQuit, 0)
		}
	}()

	initTray()

	go func() {
		for {
			time.Sleep(5 * time.Second)
			if trayHWND != 0 {
				procPostMessageW.Call(trayHWND, wmAppUpdate, 0, 0)
			}
		}
	}()

	var m msg
	for {
		ret, _, _ := procGetMessageW.Call(
			uintptr(unsafe.Pointer(&m)),
			0, 0, 0,
		)
		if ret == 0 {
			break
		}
		if ret == ^uintptr(0) {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}

	fmt.Println("Exiting.")
}
