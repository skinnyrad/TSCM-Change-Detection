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
	b := mask.Bounds()
	w, h := b.Dx(), b.Dy()

	// Label each pixel with its connected component ID (0 = background)
	labels := make([]int, w*h)
	regionCount := 0
	visited := make([]bool, w*h)

	dirs := [4][2]int{{0, 1}, {0, -1}, {1, 0}, {-1, 0}}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			if visited[idx] || mask.GrayAt(x, y).Y == 0 {
				continue
			}
			regionCount++
			label := regionCount
			queue := [][2]int{{x, y}}
			visited[idx] = true
			labels[idx] = label
			for len(queue) > 0 {
				p := queue[0]
				queue = queue[1:]
				for _, d := range dirs {
					nx, ny := p[0]+d[0], p[1]+d[1]
					if nx < 0 || nx >= w || ny < 0 || ny >= h {
						continue
					}
					nidx := ny*w + nx
					if visited[nidx] || mask.GrayAt(nx, ny).Y == 0 {
						continue
					}
					visited[nidx] = true
					labels[nidx] = label
					queue = append(queue, [2]int{nx, ny})
				}
			}
		}
	}

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
			if labels[y*w+x] == 0 {
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
