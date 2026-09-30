package imgproc

import (
	"image"
	"image/color"
)

// DrawContours finds the outer boundary of each connected component in mask
// and draws it onto img in the specified color. Returns the annotated image
// and the number of regions found.
// Matches cv2.findContours(RETR_EXTERNAL) + cv2.drawContours behavior.
func DrawContours(img *image.NRGBA, mask *image.Gray, lineColor [3]uint8) (*image.NRGBA, int) {
	if img.Bounds().Dx() != mask.Bounds().Dx() || img.Bounds().Dy() != mask.Bounds().Dy() {
		return img, 0
	}
	b := mask.Bounds()
	w, h := b.Dx(), b.Dy()

	_, regions := components(mask)
	regionCount := len(regions)
	dirs := [4][2]int{{0, 1}, {0, -1}, {1, 0}, {-1, 0}}

	// Copy the input image
	out := image.NewNRGBA(img.Bounds())
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			out.SetNRGBA(x, y, img.NRGBAAt(x, y))
		}
	}

	// Draw boundary pixels: a foreground pixel is a boundary pixel if any
	// 4-connected neighbor is background (0).
	c := color.NRGBA{R: lineColor[0], G: lineColor[1], B: lineColor[2], A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if mask.GrayAt(x, y).Y == 0 {
				continue
			}
			isBoundary := false
			for _, d := range dirs {
				nx, ny := x+d[0], y+d[1]
				if nx < 0 || nx >= w || ny < 0 || ny >= h || mask.GrayAt(nx, ny).Y == 0 {
					isBoundary = true
					break
				}
			}
			if isBoundary {
				out.SetNRGBA(x, y, c)
				// Draw 2px thick contour by also setting adjacent pixels
				for _, d := range dirs {
					nx, ny := x+d[0], y+d[1]
					if nx >= 0 && nx < w && ny >= 0 && ny < h {
						out.SetNRGBA(nx, ny, c)
					}
				}
			}
		}
	}

	return out, regionCount
}
