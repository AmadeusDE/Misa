//go:build android && cgo

package android

/*
#cgo LDFLAGS: -landroid -llog

#include <android/log.h>
#include <android/native_window.h>

static void *misa_lock(void *win, int w, int h,
    int inL, int inT, int inR, int inB,
    int *outStride, int *outW, int *outH,
    int *outL, int *outT, int *outR, int *outB,
    int *err) {
    ANativeWindow *nw = (ANativeWindow *)win;
    *err = 0;
    if (nw == 0 || w <= 0 || h <= 0) {
        *err = 1;
        return 0;
    }
    if (ANativeWindow_setBuffersGeometry(nw, w, h, WINDOW_FORMAT_RGBA_8888) != 0) {
        __android_log_print(ANDROID_LOG_WARN, "misagui", "setBuffersGeometry failed");
        *err = 2;
        return 0;
    }
    ARect rect;
    rect.left = inL;
    rect.top = inT;
    rect.right = inR;
    rect.bottom = inB;
    ANativeWindow_Buffer buf;
    if (ANativeWindow_lock(nw, &buf, &rect) != 0) {
        __android_log_print(ANDROID_LOG_WARN, "misagui", "ANativeWindow_lock failed");
        *err = 3;
        return 0;
    }
    if (buf.bits == 0 || buf.stride <= 0 || buf.width <= 0 || buf.height <= 0) {
        ANativeWindow_unlockAndPost(nw);
        *err = 4;
        return 0;
    }
    *outStride = buf.stride * 4;
    *outW = buf.width;
    *outH = buf.height;
    *outL = rect.left;
    *outT = rect.top;
    *outR = rect.right;
    *outB = rect.bottom;
    return buf.bits;
}

static void misa_unlock(void *win) {
    ANativeWindow_unlockAndPost((ANativeWindow *)win);
}

static void misa_release(void *win) {
    ANativeWindow_release((ANativeWindow *)win);
}
*/
import "C"
import (
	"image"
	"sync"
	"unsafe"
)

func init() {
	nativeSurface = func(p unsafe.Pointer) Surface {
		if p == nil {
			return nil
		}
		return &cWindow{ptr: p}
	}
}

// cWindow blits into an ANativeWindow. ANativeWindow_fromSurface already
// acquired the reference; Release calls ANativeWindow_release. Lock is
// given the damage rect; the copy fills whatever region the lock reports
// back, which may be larger, and honours buffer.stride.
type cWindow struct {
	mu  sync.Mutex
	ptr unsafe.Pointer
}

func (w *cWindow) Present(f Frame) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.ptr == nil || f.WindowW <= 0 || f.WindowH <= 0 {
		return
	}
	var stride, bw, bh, left, top, right, bottom, errn C.int
	bits := C.misa_lock(w.ptr, C.int(f.WindowW), C.int(f.WindowH),
		C.int(f.Bounds.Min.X), C.int(f.Bounds.Min.Y), C.int(f.Bounds.Max.X), C.int(f.Bounds.Max.Y),
		&stride, &bw, &bh, &left, &top, &right, &bottom, &errn)
	if errn != 0 || bits == nil {
		return
	}
	defer C.misa_unlock(w.ptr)
	out := image.Rect(int(left), int(top), int(right), int(bottom))
	if out.Empty() {
		out = image.Rect(0, 0, int(bw), int(bh))
	}
	n := int(stride) * int(bh)
	if n <= 0 {
		return
	}
	dst := unsafe.Slice((*byte)(bits), n)
	Blit(dst, int(stride), int(bw), int(bh), f.Src, f.SrcStride, f.SrcW, f.SrcH, f.OriginX, f.OriginY, out, f.Swap)
}

func (w *cWindow) Release() {
	w.mu.Lock()
	p := w.ptr
	w.ptr = nil
	w.mu.Unlock()
	if p != nil {
		C.misa_release(p)
	}
}
