package imgproc

import (
	"image"
	"image/color"
)

// jetLUT is the precomputed JET colormap lookup table mapping [0,255] → RGB.
var jetLUT [256][3]uint8

func init() {
	for i := 0; i < 256; i++ {
		v := float64(i) / 255.0
		jetLUT[i] = [3]uint8{
			clampU8(jetR(v) * 255),
			clampU8(jetG(v) * 255),
			clampU8(jetB(v) * 255),
		}
	}
}

func jetR(v float64) float64 { return clamp01(1.5 - abs(4*v-3)) }
func jetG(v float64) float64 { return clamp01(1.5 - abs(4*v-2)) }
func jetB(v float64) float64 { return clamp01(1.5 - abs(4*v-1)) }

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// JETColormap applies the JET colormap to a grayscale image.
// Matches cv2.applyColorMap(diff, cv2.COLORMAP_JET) converted to RGB.
func JETColormap(gray *image.Gray) *image.NRGBA {
	b := gray.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			rgb := jetLUT[gray.GrayAt(x, y).Y]
			out.SetNRGBA(x, y, color.NRGBA{R: rgb[0], G: rgb[1], B: rgb[2], A: 255})
		}
	}
	return out
}

// NormalizeHeat stretches a diff map so its 99.5th percentile maps to 255,
// making faint change visible instead of rendering uniformly blue. Returns
// the input unchanged if it is essentially empty.
func NormalizeHeat(gray *image.Gray) *image.Gray {
	b := gray.Bounds()
	w, h := b.Dx(), b.Dy()
	var hist [256]int
	for y := 0; y < h; y++ {
		for _, v := range gray.Pix[y*gray.Stride : y*gray.Stride+w] {
			hist[v]++
		}
	}
	target, acc, p := int(0.995*float64(w*h)), 0, 255
	for v, n := range hist {
		acc += n
		if acc > target {
			p = v
			break
		}
	}
	if p < 8 {
		return gray
	}
	out := image.NewGray(b)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			out.Pix[y*out.Stride+x] = uint8(min(255, int(gray.Pix[y*gray.Stride+x])*255/p))
		}
	}
	return out
}
