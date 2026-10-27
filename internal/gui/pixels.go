package gui

// blit copies w×h pixels from src into dst. Identical formats are copied
// per row. BGRA and RGBA differ by a swap of the red and blue bytes;
// components are already premultiplied, so nothing else changes.
func blit(dst []byte, dstStride int, dstFmt PixelFormat, src []byte, srcStride int, srcFmt PixelFormat, w, h int) {
	if w <= 0 || h <= 0 || dstStride < w*4 || srcStride < w*4 {
		return
	}
	swap := dstFmt != srcFmt
	for y := 0; y < h; y++ {
		s := src[y*srcStride : y*srcStride+w*4]
		d := dst[y*dstStride : y*dstStride+w*4]
		if !swap {
			copy(d, s)
			continue
		}
		for i := 0; i < len(s); i += 4 {
			d[i], d[i+1], d[i+2], d[i+3] = s[i+2], s[i+1], s[i], s[i+3]
		}
	}
}

func samePixels(a, b []byte) bool {
	return len(a) > 0 && len(b) > 0 && &a[0] == &b[0]
}
