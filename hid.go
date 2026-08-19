package main

import (
	"fmt"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	vendorID             = 0x0B05
	productID1           = 0x1908
	productID2           = 0x1906
	usbPacketSize        = 65
	digcfPresent         = 0x00000002
	digcfDeviceInterface = 0x00000010
	invalidHandle        = ^uintptr(0)
)

var (
	hidDLL      = syscall.NewLazyDLL("hid.dll")
	setupapiDLL = syscall.NewLazyDLL("setupapi.dll")

	procHidD_GetHidGuid        = hidDLL.NewProc("HidD_GetHidGuid")
	procHidD_GetPreparsedData  = hidDLL.NewProc("HidD_GetPreparsedData")
	procHidD_FreePreparsedData = hidDLL.NewProc("HidD_FreePreparsedData")
	procHidP_GetCaps           = hidDLL.NewProc("HidP_GetCaps")
	procHidD_SetOutputReport   = hidDLL.NewProc("HidD_SetOutputReport")
	procSetupDiGetClassDevsW             = setupapiDLL.NewProc("SetupDiGetClassDevsW")
	procSetupDiEnumDeviceInterfaces      = setupapiDLL.NewProc("SetupDiEnumDeviceInterfaces")
	procSetupDiGetDeviceInterfaceDetailW = setupapiDLL.NewProc("SetupDiGetDeviceInterfaceDetailW")
	procSetupDiDestroyDeviceInfoList     = setupapiDLL.NewProc("SetupDiDestroyDeviceInfoList")
)

type hidCaps struct {
	UsagePage                 uint16
	Usage                     uint16
	InputReportByteLength     uint16
	OutputReportByteLength    uint16
	FeatureReportByteLength   uint16
	Reserved                  [17]uint16
	NumberLinkCollectionNodes uint16
	NumberInputButtonCaps     uint16
	NumberInputValueCaps      uint16
	NumberInputDataIndices    uint16
	NumberOutputButtonCaps    uint16
	NumberOutputValueCaps     uint16
	NumberOutputDataIndices   uint16
	NumberFeatureButtonCaps   uint16
	NumberFeatureValueCaps    uint16
	NumberFeatureDataIndices  uint16
}

type diInterfaceData struct {
	CbSize             uint32
	InterfaceClassGuid [16]byte
	Flags              uint32
	Reserved           uintptr
}

func utf16PtrToString(p *uint16) string {
	if p == nil {
		return ""
	}
	var size int
	for ptr := uintptr(unsafe.Pointer(p)); ; size++ {
		ch := *(*uint16)(unsafe.Pointer(ptr))
		if ch == 0 {
			break
		}
		ptr += 2
	}
	b := make([]uint16, size)
	for i := 0; i < size; i++ {
		b[i] = *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + uintptr(i)*2))
	}
	return syscall.UTF16ToString(b)
}

func getHidGuid() [16]byte {
	var guid [16]byte
	procHidD_GetHidGuid.Call(uintptr(unsafe.Pointer(&guid[0])))
	return guid
}

func enumerateHIDDevices() []string {
	var devices []string
	guid := getHidGuid()

	fmt.Printf("[ENUM] HID GUID: %02X%02X%02X%02X-%02X%02X-%02X%02X-%02X%02X-%02X%02X%02X%02X%02X%02X\n",
		guid[3], guid[2], guid[1], guid[0],
		guid[5], guid[4],
		guid[7], guid[6],
		guid[8], guid[9],
		guid[10], guid[11], guid[12], guid[13], guid[14], guid[15])

	devInfoSet, _, callErr := procSetupDiGetClassDevsW.Call(
		uintptr(unsafe.Pointer(&guid[0])),
		0, 0,
		uintptr(digcfPresent|digcfDeviceInterface),
	)
	fmt.Printf("[ENUM] SetupDiGetClassDevsW returned: 0x%X\n", devInfoSet)
	if devInfoSet == invalidHandle || devInfoSet == 0 {
		fmt.Printf("[ENUM] SetupDiGetClassDevsW failed: %v\n", callErr)
		return devices
	}
	defer procSetupDiDestroyDeviceInfoList.Call(devInfoSet)

	fmt.Printf("[ENUM] Enumerating HID devices (VID_%04X)...\n", vendorID)

	var index uint32
	for {
		var interfaceData diInterfaceData
		interfaceData.CbSize = uint32(unsafe.Sizeof(interfaceData))
		fmt.Printf("[ENUM] interfaceData.CbSize = %d (sizeof=%d)\n", interfaceData.CbSize, unsafe.Sizeof(interfaceData))

		ret, _, lastErr := procSetupDiEnumDeviceInterfaces.Call(
			devInfoSet, 0,
			uintptr(unsafe.Pointer(&guid[0])),
			uintptr(index),
			uintptr(unsafe.Pointer(&interfaceData)),
		)
		fmt.Printf("[ENUM] SetupDiEnumDeviceInterfaces(index=%d) returned %d\n", index, ret)
		if ret == 0 {
			fmt.Printf("[ENUM] GetLastError = %v\n", lastErr)
			break
		}

		var requiredSize uint32
		procSetupDiGetDeviceInterfaceDetailW.Call(
			devInfoSet,
			uintptr(unsafe.Pointer(&interfaceData)),
			0, 0,
			uintptr(unsafe.Pointer(&requiredSize)),
			0,
		)
		fmt.Printf("[ENUM] Required detail size: %d\n", requiredSize)

		buf := make([]byte, requiredSize)
		const detailCbSize = 8
		*(*uint32)(unsafe.Pointer(&buf[0])) = detailCbSize

		ret, _, lastErr = procSetupDiGetDeviceInterfaceDetailW.Call(
			devInfoSet,
			uintptr(unsafe.Pointer(&interfaceData)),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(requiredSize),
			0, 0,
		)
		if ret == 0 {
			fmt.Printf("[ENUM] SetupDiGetDeviceInterfaceDetailW failed: GetLastError = %v\n", lastErr)
			index++
			continue
		}

		pathPtr := (*uint16)(unsafe.Pointer(&buf[4]))
		path := utf16PtrToString(pathPtr)

		fmt.Printf("[ENUM] Device[%d]: %s\n", index, path)
		devices = append(devices, path)
		index++
	}

	fmt.Printf("[ENUM] Total HID devices found: %d\n", len(devices))
	return devices
}

func getBatteryInfo() (int, bool) {
	devices := enumerateHIDDevices()
	for _, path := range devices {
		lowerPath := strings.ToLower(path)

		if !strings.Contains(lowerPath, fmt.Sprintf("vid_%04x", vendorID)) {
			continue
		}

		if !strings.Contains(lowerPath, "mi_00") {
			continue
		}

		charging := false
		var pid uint16
		if strings.Contains(lowerPath, fmt.Sprintf("pid_%04x", productID2)) {
			charging = true
			pid = productID2
		} else if strings.Contains(lowerPath, fmt.Sprintf("pid_%04x", productID1)) {
			charging = false
			pid = productID1
		} else {
			continue
		}

		fmt.Printf("[BATT] Trying PID_%04X (charging=%v) MI_00 device...\n", pid, charging)
		fmt.Printf("[BATT] Path: %s\n", path)
		bat, ok := readBatteryFromDevice(path)
		if ok {
			fmt.Printf("[BATT] Battery: %d%%, charging: %v\n", bat, charging)
			return bat, charging
		}
		fmt.Printf("[BATT] PID_%04X MI_00 did not respond with battery data.\n", pid)
	}

	fmt.Println("[BATT] No battery info retrieved from any device.")
	return -1, false
}

func readBatteryFromDevice(path string) (int, bool) {
	pathPtr, _ := syscall.UTF16PtrFromString(path)
	handle, err := syscall.CreateFile(
		pathPtr,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil, syscall.OPEN_EXISTING, 0, 0,
	)
	if err != nil {
		fmt.Printf("[HID ] CreateFile failed: %v\n", err)
		return -1, false
	}
	defer syscall.CloseHandle(handle)

	var preparsedData uintptr
	ret, _, _ := procHidD_GetPreparsedData.Call(uintptr(handle), uintptr(unsafe.Pointer(&preparsedData)))
	if ret == 0 {
		fmt.Println("[HID ] HidD_GetPreparsedData failed")
		return -1, false
	}
	defer procHidD_FreePreparsedData.Call(preparsedData)

	var caps hidCaps
	ret, _, _ = procHidP_GetCaps.Call(preparsedData, uintptr(unsafe.Pointer(&caps)))
	if ret == 0 {
		fmt.Println("[HID ] HidP_GetCaps failed")
		return -1, false
	}

	fmt.Printf("[HID ] Caps: InputReportLen=%d OutputReportLen=%d\n",
		caps.InputReportByteLength, caps.OutputReportByteLength)

	var packet [usbPacketSize]byte
	packet[0] = 0x00
	packet[1] = 0x12
	packet[2] = 0x07

	ret, _, _ = procHidD_SetOutputReport.Call(uintptr(handle), uintptr(unsafe.Pointer(&packet[0])), uintptr(len(packet)))
	fmt.Printf("[HID ] HidD_SetOutputReport returned %d\n", ret)
	if ret == 0 {
		fmt.Println("[HID ] HidD_SetOutputReport failed, trying WriteFile...")
		var bytesWritten uint32
		err = syscall.WriteFile(handle, packet[:], &bytesWritten, nil)
		if err != nil {
			fmt.Printf("[HID ] WriteFile failed: %v\n", err)
			return -1, false
		}
		fmt.Printf("[HID ] WriteFile sent %d bytes\n", bytesWritten)
	}

	time.Sleep(50 * time.Millisecond)

	var response [usbPacketSize]byte
	var bytesRead uint32
	err = syscall.ReadFile(handle, response[:], &bytesRead, nil)
	if err != nil {
		fmt.Printf("[HID ] ReadFile failed: %v\n", err)
		return -1, false
	}

	fmt.Printf("[HID ] Read %d bytes: %02X %02X %02X %02X %02X %02X %02X %02X %02X %02X ...\n",
		bytesRead,
		response[0], response[1], response[2], response[3], response[4],
		response[5], response[6], response[7], response[8], response[9])

	if bytesRead >= 6 && response[1] == 0x12 && response[2] == 0x07 {
		battery := int(response[5]) * 25
		fmt.Printf("[HID ] Parsed battery: %d%% (raw=%d)\n", battery, response[5])
		return battery, true
	}

	if bytesRead >= 5 && response[0] == 0x12 && response[1] == 0x07 {
		battery := int(response[4]) * 25
		fmt.Printf("[HID ] Parsed battery (no report ID): %d%% (raw=%d)\n", battery, response[4])
		return battery, true
	}

	fmt.Println("[HID ] Response does not match expected battery report header")
	return -1, false
}
