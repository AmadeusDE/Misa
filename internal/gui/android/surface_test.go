package android

import "sync"

type memSurface struct {
	mu       sync.Mutex
	pix      []byte
	stride   int
	w, h     int
	presents int
}

func newMemSurface(w, h int) *memSurface {
	stride := w * 4
	return &memSurface{pix: make([]byte, stride*h), stride: stride, w: w, h: h}
}

func (m *memSurface) Present(f Frame) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if f.WindowW != m.w || f.WindowH != m.h || f.WindowW <= 0 || f.WindowH <= 0 {
		return
	}
	Blit(m.pix, m.stride, m.w, m.h, f.Src, f.SrcStride, f.SrcW, f.SrcH, f.OriginX, f.OriginY, f.Bounds, f.Swap)
	m.presents++
}

func (m *memSurface) Release() {}

var _ Surface = (*memSurface)(nil)
