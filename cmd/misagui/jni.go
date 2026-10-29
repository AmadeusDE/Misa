//go:build android && cgo

package main

/*
#cgo LDFLAGS: -landroid -llog

#include <stdlib.h>
*/
import "C"
import (
	"unsafe"

	"github.com/AmadeusDE/Misa/internal/gui/android"
)

func main() {}

//export misaJNIClass
func misaJNIClass() *C.char {
	return C.CString(android.JNIClassName())
}

//export misaOnCreate
func misaOnCreate(exe *C.char, vars **C.char, n C.int) *C.char {
	if err := android.Default.OnCreate(cStrings(vars, n), C.GoString(exe)); err != nil {
		return C.CString(err.Error())
	}
	return nil
}

//export misaOnSurface
func misaOnSurface(win unsafe.Pointer) {
	android.Default.OnSurfaceCreated(win)
}

//export misaOnSurfaceChanged
func misaOnSurfaceChanged(w, h C.int, density C.float, left, top, right, bottom C.int) {
	android.Default.OnSurfaceChanged(int(w), int(h), float64(density), int(left), int(top), int(right), int(bottom))
}

//export misaOnSurfaceDestroyed
func misaOnSurfaceDestroyed() { android.Default.OnSurfaceDestroyed() }

//export misaOnText
func misaOnText(s *C.char, meta C.int) {
	android.Default.OnText(C.GoString(s), int(meta))
}

//export misaMuxAction
func misaMuxAction(s *C.char) { android.Default.MuxAction(C.GoString(s)) }

//export misaOnKey
func misaOnKey(code, meta, down C.int) {
	android.Default.OnKey(int(code), int(meta), down != 0)
}

//export misaOnMotion
func misaOnMotion(action C.int, x, y C.float, buttons C.int) {
	android.Default.OnMotion(int(action), float64(x), float64(y), int(buttons))
}

//export misaOnPinch
func misaOnPinch(scale C.float) { android.Default.OnPinch(float64(scale)) }

//export misaOnLowMemory
func misaOnLowMemory() { android.Default.OnLowMemory() }

//export misaNewSession
func misaNewSession() C.int { return C.int(android.Default.NewSession()) }

//export misaCloseSession
func misaCloseSession(id C.int) { android.Default.CloseSession(int(id)) }

//export misaSwitchSession
func misaSwitchSession(id C.int) { android.Default.SwitchSession(int(id)) }

//export misaActiveSession
func misaActiveSession() C.int { return C.int(android.Default.ActiveSession()) }

//export misaSessionCount
func misaSessionCount() C.int { return C.int(android.Default.SessionCount()) }

func cStrings(vars **C.char, n C.int) []string {
	if vars == nil || n <= 0 {
		return nil
	}
	out := make([]string, int(n))
	base := unsafe.Pointer(vars)
	step := unsafe.Sizeof(*vars)
	for i := range out {
		p := *(**C.char)(unsafe.Add(base, uintptr(i)*step))
		out[i] = C.GoString(p)
	}
	return out
}
