package android

import "image"

// DamageBounds unions damage, shifts it by the terminal's origin in the
// window, and clips it to the window. A nil or empty damage list is the
// whole window: the first frame and any frame whose damage was discarded.
func DamageBounds(damage []image.Rectangle, ox, oy, dstW, dstH int) (image.Rectangle, bool) {
	if dstW <= 0 || dstH <= 0 {
		return image.Rectangle{}, false
	}
	full := image.Rect(0, 0, dstW, dstH)
	if len(damage) == 0 {
		return full, true
	}
	var u image.Rectangle
	any := false
	for _, r := range damage {
		if r.Empty() {
			continue
		}
		r = image.Rect(r.Min.X+ox, r.Min.Y+oy, r.Max.X+ox, r.Max.Y+oy)
		if !any {
			u = r
			any = true
		} else {
			u = u.Union(r)
		}
	}
	if !any {
		return image.Rectangle{}, false
	}
	u = u.Intersect(full)
	if u.Empty() {
		return image.Rectangle{}, false
	}
	return u, true
}

// Blit copies src into dst at (ox, oy) for region, which is in dst pixels.
// Rows respect each stride. Pixels of region that fall outside src are
// opaque black, so a dirty inset bar is cleared. swap exchanges the red
// and blue bytes (BGRA source, RGBA window); both are already premultiplied.
func Blit(dst []byte, dstStride, dstW, dstH int, src []byte, srcStride, srcW, srcH, ox, oy int, region image.Rectangle, swap bool) {
	if dstW <= 0 || dstH <= 0 || srcW <= 0 || srcH <= 0 || dstStride < dstW*4 || srcStride < srcW*4 {
		return
	}
	if len(dst) < dstStride*(dstH-1)+dstW*4 || len(src) < srcStride*(srcH-1)+srcW*4 {
		return
	}
	region = region.Intersect(image.Rect(0, 0, dstW, dstH))
	if region.Empty() {
		return
	}
	for y := region.Min.Y; y < region.Max.Y; y++ {
		sy := y - oy
		for x := region.Min.X; x < region.Max.X; {
			sx := x - ox
			if sy < 0 || sy >= srcH || sx < 0 || sx >= srcW {
				di := y*dstStride + x*4
				dst[di], dst[di+1], dst[di+2], dst[di+3] = 0, 0, 0, 255
				x++
				continue
			}
			n := region.Max.X - x
			if sx+n > srcW {
				n = srcW - sx
			}
			si := sy*srcStride + sx*4
			di := y*dstStride + x*4
			if !swap {
				copy(dst[di:di+n*4], src[si:si+n*4])
			} else {
				s := src[si : si+n*4]
				d := dst[di : di+n*4]
				for i := 0; i < len(s); i += 4 {
					d[i], d[i+1], d[i+2], d[i+3] = s[i+2], s[i+1], s[i], s[i+3]
				}
			}
			x += n
		}
	}
}
