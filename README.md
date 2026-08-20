> [!CAUTION]
> This code is pure neuroslop.

# pugio-go

A Windows system tray utility that reports the battery status of the ASUS ROG Pugio II wireless mouse. It queries the mouse over HID every few seconds and updates a tray icon with the current charge level and charging state. Only the Pugio II is recognized; all other HID devices are ignored.

The program is single-instance and exits if another copy is already running. It targets only the ASUS vendor (VID 0x0B05) and the two Pugio II product IDs (PID 0x1906 charging, PID 0x1908 wireless), and only reads the MI_00 control interface plus the 0x12 0x07 battery report opcode.

## Original idea

A Go neuroslop port of [Asus ROG Pugio II Battery Status](https://github.com/dkajan19/Asus-ROG-Pugio-II-Battery-Status) by dkajan19.

## Requirements

Go 1.21 or newer. Windows only; the code uses syscall bindings to user32.dll, hid.dll, and setupapi.dll.

## Building

There are no external Go dependencies; the standard library is enough.

### Console build (with debug output)

```go
go build
```

Produces a console application. A console window will appear and display all diagnostic output (`[ENUM]`, `[BATT]`, `[HID ]` prefixed logs). Useful for debugging HID communication or verifying device detection.

### GUI build (no console window)

```go
go build -ldflags -H=windowsgui
```

Produces a GUI application with no console window. All diagnostic output is discarded. Use this for daily use — the tray icon is the only interface.

### Custom output name

```go
go build -ldflags -H=windowsgui -o pugio-go.exe
```