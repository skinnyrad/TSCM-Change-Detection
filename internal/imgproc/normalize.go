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

// MatchLocalLight scales each RGB channel of src by the ratio of the local
// means of ref and src over a window of frac × the image diagonal, so slowly
// varying lighting (shading, a lamp switched on, uneven white balance) stops
// reading as change. Only pixels where valid is set count towards the means,
// and a second pass drops pixels the first fit cannot explain (the changes
// themselves), so an object does not drag its surroundings' gain and leave a
// halo. Gains are limited to [1/3, 3].
func MatchLocalLight(src, ref *image.NRGBA, valid *image.Gray, frac float64) *image.NRGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	r := int(frac*math.Hypot(float64(w), float64(h))/2 + 0.5)
	if r < 2 {
		return src
	}
	ps := w + 1
	integ := func(val func(i int) float64) []float64 {
		s := make([]float64, ps*(h+1))
		for y := 0; y < h; y++ {
			row := 0.0
			for x := 0; x < w; x++ {
				row += val(y*w + x)
				s[(y+1)*ps+x+1] = s[y*ps+x+1] + row
			}
		}
		return s
	}
	box := func(s []float64, x, y int) float64 {
		x0, y0, x1, y1 := max(x-r, 0), max(y-r, 0), min(x+r+1, w), min(y+r+1, h)
		return s[y1*ps+x1] - s[y0*ps+x1] - s[y1*ps+x0] + s[y0*ps+x0]
	}
	px := func(img *image.NRGBA, i, c int) float64 { return float64(img.Pix[(i/w)*img.Stride+(i%w)*4+c]) }

	use := make([]bool, w*h)
	for i := range use {
		use[i] = valid == nil || valid.Pix[(i/w)*valid.Stride+i%w] != 0
	}
	gains := func() [3][]float32 {
		var g [3][]float32
		for c := 0; c < 3; c++ {
			sumS := integ(func(i int) float64 {
				if use[i] {
					return px(src, i, c)
				}
				return 0
			})
			sumR := integ(func(i int) float64 {
				if use[i] {
					return px(ref, i, c)
				}
				return 0
			})
			g[c] = make([]float32, w*h)
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					v := (box(sumR, x, y) + 8) / (box(sumS, x, y) + 8) // the pixel counts cancel
					g[c][y*w+x] = float32(math.Max(1.0/3, math.Min(3, v)))
				}
			}
		}
		return g
	}
	g := gains()
	// Drop pixels whose residual after the first fit is well above typical.
	res := make([]float64, w*h)
	var hist [256]int
	n := 0
	for i := range res {
		if !use[i] {
			continue
		}
		for c := 0; c < 3; c++ {
			res[i] += math.Abs(px(ref, i, c) - float64(g[c][i])*px(src, i, c))
		}
		hist[min(int(res[i]/3), 255)]++
		n++
	}
	med, acc := 0, 0
	for v, k := range hist {
		if acc += k; acc > n/2 {
			med = v
			break
		}
	}
	cut := 3 * math.Max(3*float64(med), 12)
	for i := range res {
		if res[i] > cut {
			use[i] = false
		}
	}
	g = gains()

	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	copy(out.Pix, src.Pix)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			for c := 0; c < 3; c++ {
				o := y*out.Stride + x*4 + c
				out.Pix[o] = clampU8(float64(g[c][y*w+x]) * float64(src.Pix[y*src.Stride+x*4+c]))
			}
		}
	}
	return out
}
