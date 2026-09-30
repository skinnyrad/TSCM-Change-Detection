package imgproc

import (
	"image"
	"math"
	"sort"
)

// MatchIntensity returns src with each RGB channel remapped by a robust
// gain+offset (least squares against ref over pixels where valid is set and
// neither image is clipped). This compensates exposure and white-balance drift
// between shots far better than a global mean shift. Changed regions are
// trimmed out over a few iterations so they do not bias the fit.
func MatchIntensity(src, ref *image.NRGBA, valid *image.Gray) *image.NRGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	copy(out.Pix, src.Pix)
	for c := 0; c < 3; c++ {
		gain, offset, sigma := 1.0, 0.0, math.Inf(1)
		keep := func(x, y int) bool {
			if valid != nil && valid.Pix[y*valid.Stride+x] == 0 {
				return false
			}
			s, r := src.Pix[y*src.Stride+x*4+c], ref.Pix[y*ref.Stride+x*4+c]
			return s > 4 && s < 251 && r > 4 && r < 251
		}
		for iter := 0; iter < 3; iter++ {
			var n, sx, sy, sxx, sxy float64
			for y := 0; y < h; y += 3 {
				for x := 0; x < w; x += 3 {
					if !keep(x, y) {
						continue
					}
					s := float64(src.Pix[y*src.Stride+x*4+c])
					r := float64(ref.Pix[y*ref.Stride+x*4+c])
					if math.Abs(r-(gain*s+offset)) > 2.5*sigma {
						continue
					}
					n++
					sx += s
					sy += r
					sxx += s * s
					sxy += s * r
				}
			}
			if n < 100 {
				break
			}
			den := n*sxx - sx*sx
			if math.Abs(den) < 1e-6 {
				break
			}
			gain = (n*sxy - sx*sy) / den
			offset = (sy - gain*sx) / n
			gain = math.Max(0.6, math.Min(1.6, gain))
			// Residual sigma for the next trimming round.
			var ss, cnt float64
			for y := 0; y < h; y += 3 {
				for x := 0; x < w; x += 3 {
					if !keep(x, y) {
						continue
					}
					d := float64(ref.Pix[y*ref.Stride+x*4+c]) - (gain*float64(src.Pix[y*src.Stride+x*4+c]) + offset)
					ss += d * d
					cnt++
				}
			}
			sigma = math.Max(math.Sqrt(ss/math.Max(cnt, 1)), 2)
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				i := y*out.Stride + x*4 + c
				out.Pix[i] = clampU8(gain*float64(src.Pix[i]) + offset)
			}
		}
	}
	return out
}

// CombineBaselines merges several registered, same-size baseline images into
// a per-pixel median reference plus a spread map (scaled MAD of luma across the
// baselines), so naturally varying areas need a bigger change to register.
func CombineBaselines(set []*image.NRGBA) (*image.NRGBA, *image.Gray) {
	w, h := set[0].Bounds().Dx(), set[0].Bounds().Dy()
	ref := image.NewNRGBA(image.Rect(0, 0, w, h))
	spread := image.NewGray(image.Rect(0, 0, w, h))
	n := len(set)
	vals := make([]float64, n)
	med := func(v []float64) float64 {
		sort.Float64s(v)
		if len(v)%2 == 1 {
			return v[len(v)/2]
		}
		return (v[len(v)/2-1] + v[len(v)/2]) / 2
	}
	luma := make([]float64, n)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			o := y*ref.Stride + x*4
			for c := 0; c < 3; c++ {
				for i, im := range set {
					vals[i] = float64(im.Pix[y*im.Stride+x*4+c])
				}
				ref.Pix[o+c] = uint8(med(vals) + 0.5)
			}
			ref.Pix[o+3] = 255
			for i, im := range set {
				p := y*im.Stride + x*4
				luma[i] = lumaOf(im.Pix[p : p+3])
				vals[i] = luma[i]
			}
			m := med(vals)
			for i := range luma {
				vals[i] = math.Abs(luma[i] - m)
			}
			spread.Pix[y*spread.Stride+x] = uint8(math.Min(255, 1.4826*med(vals)+0.5))
		}
	}
	return ref, spread
}
