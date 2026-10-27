package android

import (
	"image"
	"unsafe"
)

// Frame is one present. Src is the terminal image in RGBA (or BGRA when
// Swap). Bounds is the window region to update, already shifted by Origin.
// WindowW and WindowH are the full surface, including inset bars.
type Frame struct {
	Src                   []byte
	SrcStride, SrcW, SrcH int
	OriginX, OriginY      int
	WindowW, WindowH      int
	Bounds                image.Rectangle
	Swap                  bool
}

// Surface receives frames. Release drops the native window; it is safe to
// call while Present is running.
type Surface interface {
	Present(Frame)
	Release()
}

// nativeSurface is set by the android && cgo file. It is nil on other OSes
// and in tests, which install their own Surface.
var nativeSurface func(unsafe.Pointer) Surface
