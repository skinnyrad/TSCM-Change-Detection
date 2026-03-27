package imgproc

import (
	"image"
	"image/color"
)

// Subtract performs float32 per-channel subtraction (after - before) and
// normalizes the result to [0, 255] per channel.
// Matches Python's apply_image_subtraction() with cv2.normalize(NORM_MINMAX).
func Subtract(before, after *image.NRGBA) *image.NRGBA {
	b := after.Bounds()
	w, h := b.Dx(), b.Dy()

	// Compute float32 differences per channel
	rDiff := make([]float32, w*h)
	gDiff := make([]float32, w*h)
	bDiff := make([]float32, w*h)

	rMin, gMin, bMin := float32(1e9), float32(1e9), float32(1e9)
	rMax, gMax, bMax := float32(-1e9), float32(-1e9), float32(-1e9)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ca := before.NRGBAAt(x, y)
			cb := after.NRGBAAt(x, y)
			idx := y*w + x

			r := float32(cb.R) - float32(ca.R)
			g := float32(cb.G) - float32(ca.G)
			bv := float32(cb.B) - float32(ca.B)

			rDiff[idx] = r
			gDiff[idx] = g
			bDiff[idx] = bv

			if r < rMin {
				rMin = r
			}
			if r > rMax {
				rMax = r
			}
			if g < gMin {
				gMin = g
			}
			if g > gMax {
				gMax = g
			}
			if bv < bMin {
				bMin = bv
			}
			if bv > bMax {
				bMax = bv
			}
		}
	}

	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			out.SetNRGBA(x, y, color.NRGBA{
				R: normalizeChannel(rDiff[idx], rMin, rMax),
				G: normalizeChannel(gDiff[idx], gMin, gMax),
				B: normalizeChannel(bDiff[idx], bMin, bMax),
				A: 255,
			})
		}
	}
	return out
}

func normalizeChannel(v, min, max float32) uint8 {
	if max == min {
		return 0
	}
	normalized := (v - min) / (max - min) * 255.0
	return clampU8(float64(normalized))
}
