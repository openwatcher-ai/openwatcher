//go:build windows

package main

import (
	"context"
	"openwatcher/desktop-app/widget/internal/widgetwindow"
	"syscall"
	"unsafe"
)

var user32 = syscall.NewLazyDLL("user32.dll")
var enumDisplayMonitors = user32.NewProc("EnumDisplayMonitors")
var getMonitorInfo = user32.NewProc("GetMonitorInfoW")
var enumWindows = user32.NewProc("EnumWindows")
var getWindowTextW = user32.NewProc("GetWindowTextW")
var showWindow = user32.NewProc("ShowWindow")
var setForegroundWindow = user32.NewProc("SetForegroundWindow")
var getActiveWindow = user32.NewProc("GetActiveWindow")
var findWindowW = user32.NewProc("FindWindowW")
var getWindowLongPtr = user32.NewProc("GetWindowLongPtrW")
var setWindowLongPtr = user32.NewProc("SetWindowLongPtrW")
var getWindowRect = user32.NewProc("GetWindowRect")
var setWindowPos = user32.NewProc("SetWindowPos")

type rect struct{ Left, Top, Right, Bottom int32 }
type monitorInfoEx struct {
	CbSize            uint32
	RcMonitor, RcWork rect
	DwFlags           uint32
	SzDevice          [32]uint16
}

func setWidgetPlatform(context.Context) { // Wails has no public tool-window option; apply WS_EX_TOOLWINDOW to its HWND.
	hwnd := widgetWindowHandle()
	if hwnd == 0 {
		hwnd, _, _ = getActiveWindow.Call()
	}
	if hwnd == 0 {
		return
	}
	const gwlExStyle uintptr = ^uintptr(19) // -20 as the uintptr ABI argument
	const wsExToolWindow = 0x00000080
	const wsExAppWindow = 0x00040000
	style, _, _ := getWindowLongPtr.Call(hwnd, gwlExStyle)
	style = (style | wsExToolWindow) &^ wsExAppWindow
	setWindowLongPtr.Call(hwnd, gwlExStyle, style)
	const swpNoSize = 0x0001
	const swpNoMove = 0x0002
	const swpNoActivate = 0x0010
	const swpFrameChanged = 0x0020
	setWindowPos.Call(hwnd, 0, 0, 0, 0, 0, swpNoSize|swpNoMove|swpNoActivate|swpFrameChanged)
}
func nativeWorkAreas() []widgetwindow.WorkArea {
	out := []widgetwindow.WorkArea{}
	cb := syscall.NewCallback(func(h, _, _ uintptr, _ uintptr) uintptr {
		var mi monitorInfoEx
		mi.CbSize = uint32(unsafe.Sizeof(mi))
		if r, _, _ := getMonitorInfo.Call(h, uintptr(unsafe.Pointer(&mi))); r != 0 {
			device := syscall.UTF16ToString(mi.SzDevice[:])
			out = append(out, widgetwindow.WorkArea{X: float64(mi.RcWork.Left), Y: float64(mi.RcWork.Top), Width: float64(mi.RcWork.Right - mi.RcWork.Left), Height: float64(mi.RcWork.Bottom - mi.RcWork.Top), MonitorID: "display-" + device})
		}
		return 1
	})
	enumDisplayMonitors.Call(0, 0, cb, 0)
	if len(out) == 0 {
		out = append(out, widgetwindow.WorkArea{Width: 1920, Height: 1080, MonitorID: "primary"})
	}
	return out
}
func nativeWindowGeometry(context.Context) widgetwindow.Geometry {
	hwnd := widgetWindowHandle()
	var r rect
	if hwnd == 0 {
		return widgetwindow.DefaultGeometry()
	}
	if ok, _, _ := getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r))); ok == 0 {
		return widgetwindow.DefaultGeometry()
	}
	return widgetwindow.Geometry{X: float64(r.Left), Y: float64(r.Top), Width: float64(r.Right - r.Left), Height: float64(r.Bottom - r.Top)}
}
func nativeApplyGeometry(_ context.Context, g widgetwindow.Geometry) {
	hwnd := widgetWindowHandle()
	if hwnd == 0 {
		return
	}
	const hwndTopMost = ^uintptr(0)
	const swpNoActivate = 0x0010
	const swpShowWindow = 0x0040
	setWindowPos.Call(hwnd, hwndTopMost, uintptr(int32(g.X)), uintptr(int32(g.Y)), uintptr(int32(g.Width)), uintptr(int32(g.Height)), swpNoActivate|swpShowWindow)
}
func widgetWindowHandle() uintptr {
	title, err := syscall.UTF16PtrFromString("OpenWatcher Widget")
	if err != nil {
		return 0
	}
	hwnd, _, _ := findWindowW.Call(0, uintptr(unsafe.Pointer(title)))
	return hwnd
}
func nativeOpenMainApp() {
	cb := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		buf := make([]uint16, 256)
		getWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), 256)
		if syscall.UTF16ToString(buf) == "OpenWatcher" {
			showWindow.Call(hwnd, 9)
			setForegroundWindow.Call(hwnd)
			return 0
		}
		return 1
	})
	enumWindows.Call(cb, 0)
}
