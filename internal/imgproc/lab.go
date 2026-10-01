package imgproc

import (
	"image"
	"math"
)

// labPlanes holds an image as separate CIE L*a*b* float planes (D65).
type labPlanes struct {
	W, H    int
	L, A, B []float32
}

var srgbToLinear [256]float32

func init() {
	for i := range srgbToLinear {
		c := float64(i) / 255
		if c <= 0.04045 {
			srgbToLinear[i] = float32(c / 12.92)
		} else {
			srgbToLinear[i] = float32(math.Pow((c+0.055)/1.055, 2.4))
		}
	}
}

func labF(t float64) float64 {
	if t > 216.0/24389.0 {
		return math.Cbrt(t)
	}
	return (24389.0/27.0*t + 16) / 116
}

func toLab(img *image.NRGBA) labPlanes {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	p := labPlanes{W: w, H: h, L: make([]float32, w*h), A: make([]float32, w*h), B: make([]float32, w*h)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*img.Stride + x*4
			r, g, b := float64(srgbToLinear[img.Pix[i]]), float64(srgbToLinear[img.Pix[i+1]]), float64(srgbToLinear[img.Pix[i+2]])
			fx := labF((0.4124564*r + 0.3575761*g + 0.1804375*b) / 0.95047)
			fy := labF(0.2126729*r + 0.7151522*g + 0.0721750*b)
			fz := labF((0.0193339*r + 0.1191920*g + 0.9503041*b) / 1.08883)
			o := y*w + x
			p.L[o] = float32(116*fy - 16)
			p.A[o] = float32(500 * (fx - fy))
			p.B[o] = float32(200 * (fy - fz))
		}
	}
	return p
}
