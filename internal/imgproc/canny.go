package imgproc

import (
	"image"
	"image/color"
	"math"

	"github.com/anthonynsimon/bild/blur"
)

// CannyEdge applies Canny edge detection to a grayscale image.
// Matches cv2.Canny(diff, low, high).
// Note: results are visually equivalent to OpenCV but not pixel-identical.
func CannyEdge(gray *image.Gray, low, high uint8) *image.Gray {
	// Step 1: Gaussian blur to reduce noise (sigma=1.0, matches OpenCV's default)
	blurred := blur.Gaussian(gray, 1.0)
	blurGray := rgbaToGray(blurred)

	b := blurGray.Bounds()
	w, h := b.Dx(), b.Dy()

	// Step 2: Sobel gradients
	mag := make([]float64, w*h)
	dir := make([]uint8, w*h) // quantized direction: 0, 1, 2, 3 (0°, 45°, 90°, 135°)

	sobelX := [3][3]float64{
		{-1, 0, 1},
		{-2, 0, 2},
		{-1, 0, 1},
	}
	sobelY := [3][3]float64{
		{-1, -2, -1},
		{0, 0, 0},
		{1, 2, 1},
	}

	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			var gx, gy float64
			for ky := -1; ky <= 1; ky++ {
				for kx := -1; kx <= 1; kx++ {
					v := float64(blurGray.GrayAt(x+kx, y+ky).Y)
					gx += sobelX[ky+1][kx+1] * v
					gy += sobelY[ky+1][kx+1] * v
				}
			}
			idx := y*w + x
			mag[idx] = math.Sqrt(gx*gx + gy*gy)
			// Quantize angle to 4 directions
			angle := math.Atan2(gy, gx) * 180 / math.Pi
			if angle < 0 {
				angle += 180
			}
			switch {
			case angle < 22.5 || angle >= 157.5:
				dir[idx] = 0 // horizontal
			case angle < 67.5:
				dir[idx] = 1 // 45°
			case angle < 112.5:
				dir[idx] = 2 // vertical
			default:
				dir[idx] = 3 // 135°
			}
		}
	}

	// Step 3: Non-maximum suppression
	nms := make([]float64, w*h)
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			idx := y*w + x
			m := mag[idx]
			var n1, n2 float64
			switch dir[idx] {
			case 0: // horizontal
				n1, n2 = mag[y*w+(x-1)], mag[y*w+(x+1)]
			case 1: // 45°
				n1, n2 = mag[(y-1)*w+(x-1)], mag[(y+1)*w+(x+1)]
			case 2: // vertical
				n1, n2 = mag[(y-1)*w+x], mag[(y+1)*w+x]
			case 3: // 135°
				n1, n2 = mag[(y-1)*w+(x+1)], mag[(y+1)*w+(x-1)]
			}
			if m >= n1 && m >= n2 {
				nms[idx] = m
			}
		}
	}

	// Step 4: Double threshold
	const strong uint8 = 255
	const weak uint8 = 128
	result := make([]uint8, w*h)
	lowF := float64(low)
	highF := float64(high)

	for i, v := range nms {
		switch {
		case v >= highF:
			result[i] = strong
		case v >= lowF:
			result[i] = weak
		}
	}

	// Step 5: Hysteresis — promote weak pixels connected to strong pixels
	// Use iterative flood-fill from all strong pixels
	changed := true
	for changed {
		changed = false
		for y := 1; y < h-1; y++ {
			for x := 1; x < w-1; x++ {
				if result[y*w+x] != weak {
					continue
				}
				// Check 8-connected neighbors for a strong pixel
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						if result[(y+dy)*w+(x+dx)] == strong {
							result[y*w+x] = strong
							changed = true
							goto nextPixel
						}
					}
				}
			nextPixel:
			}
		}
	}

	// Build output: only strong pixels survive
	out := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if result[y*w+x] == strong {
				out.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}
	return out
}
