package imgproc

import (
	"image"
	"image/color"
)

// HighlightChanges blends a color overlay onto after wherever mask > 0.
// alpha controls the overlay strength (0.55 matches OpenCV's addWeighted).
// Matches Python's highlight_changes(img2, thresh, color=(255,60,60), alpha=0.55).
func HighlightChanges(after *image.NRGBA, mask *image.Gray, overlayColor [3]uint8, alpha float64) *image.NRGBA {
	b := after.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	inv := 1.0 - alpha

	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c := after.NRGBAAt(x, y)
			if mask.GrayAt(x, y).Y > 0 {
				out.SetNRGBA(x, y, color.NRGBA{
					R: clampU8(float64(c.R)*inv + float64(overlayColor[0])*alpha),
					G: clampU8(float64(c.G)*inv + float64(overlayColor[1])*alpha),
					B: clampU8(float64(c.B)*inv + float64(overlayColor[2])*alpha),
					A: 255,
				})
			} else {
				out.SetNRGBA(x, y, color.NRGBA{R: c.R, G: c.G, B: c.B, A: 255})
			}
		}
	}
	return out
}

func clampU8(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v + 0.5)
}
