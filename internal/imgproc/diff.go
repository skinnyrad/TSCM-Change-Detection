package imgproc

import (
	"image"
	"image/color"

	"github.com/anthonynsimon/bild/effect"
)

// AbsDiff computes the absolute per-pixel difference between two NRGBA images
// in grayscale (Rec. 601 luma). Matches cv2.absdiff on grayscale inputs.
func AbsDiff(a, b *image.NRGBA) *image.Gray {
	bounds := a.Bounds()
	out := image.NewGray(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			ca := a.NRGBAAt(x, y)
			cb := b.NRGBAAt(x, y)
			ga := uint8(float64(ca.R)*0.299 + float64(ca.G)*0.587 + float64(ca.B)*0.114 + 0.5)
			gb := uint8(float64(cb.R)*0.299 + float64(cb.G)*0.587 + float64(cb.B)*0.114 + 0.5)
			var d uint8
			if ga > gb {
				d = ga - gb
			} else {
				d = gb - ga
			}
			out.SetGray(x, y, color.Gray{Y: d})
		}
	}
	return out
}

// BinaryThreshold applies a binary threshold to a grayscale image.
// Pixels above threshold become 255, others become 0.
// Matches cv2.threshold with THRESH_BINARY.
func BinaryThreshold(gray *image.Gray, threshold uint8) *image.Gray {
	b := gray.Bounds()
	out := image.NewGray(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			if gray.GrayAt(x, y).Y > threshold {
				out.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}
	return out
}

// MorphologicalOpen applies morphological opening (erosion then dilation) to a
// binary mask. radius=2.0 approximates OpenCV's 5×5 ones kernel.
// Matches cv2.morphologyEx(mask, cv2.MORPH_OPEN, np.ones((5,5))).
func MorphologicalOpen(mask *image.Gray, radius float64) *image.Gray {
	eroded := effect.Erode(mask, radius)
	dilated := effect.Dilate(eroded, radius)
	return rgbaToGray(dilated)
}

// ComputeDiff combines AbsDiff, BinaryThreshold, and MorphologicalOpen into
// the full change detection pipeline, returning (diffGray, thresholdMask).
func ComputeDiff(before, after *image.NRGBA, threshold uint8) (*image.Gray, *image.Gray) {
	diff := AbsDiff(before, after)
	thresh := BinaryThreshold(diff, threshold)
	thresh = MorphologicalOpen(thresh, 2.0)
	return diff, thresh
}
