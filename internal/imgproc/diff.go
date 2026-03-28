package imgproc

import (
	"image"
)

// AbsDiff computes the absolute per-pixel grayscale difference between two
// NRGBA images. Uses direct Pix access for speed.
func AbsDiff(a, b *image.NRGBA) *image.Gray {
	bounds := a.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	out := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ai := y*a.Stride + x*4
			bi := y*b.Stride + x*4
			ga := uint8(float64(a.Pix[ai])*0.299 + float64(a.Pix[ai+1])*0.587 + float64(a.Pix[ai+2])*0.114 + 0.5)
			gb := uint8(float64(b.Pix[bi])*0.299 + float64(b.Pix[bi+1])*0.587 + float64(b.Pix[bi+2])*0.114 + 0.5)
			if ga > gb {
				out.Pix[y*out.Stride+x] = ga - gb
			} else {
				out.Pix[y*out.Stride+x] = gb - ga
			}
		}
	}
	return out
}

// BinaryThreshold applies a binary threshold to a grayscale image.
// Pixels above threshold become 255, others become 0.
func BinaryThreshold(gray *image.Gray, threshold uint8) *image.Gray {
	b := gray.Bounds()
	w, h := b.Dx(), b.Dy()
	out := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		src := gray.Pix[y*gray.Stride : y*gray.Stride+w]
		dst := out.Pix[y*out.Stride : y*out.Stride+w]
		for x, v := range src {
			if v > threshold {
				dst[x] = 255
			}
		}
	}
	return out
}

// MorphologicalOpen applies morphological opening (erosion then dilation) to a
// binary mask using a square (2r+1)×(2r+1) structuring element.
// Uses 2D prefix sums for O(w·h) complexity independent of radius.
func MorphologicalOpen(mask *image.Gray, r int) *image.Gray {
	return fastBinaryDilate(fastBinaryErode(mask, r), r)
}

// buildPrefixSum builds a (w+1)×(h+1) integral image over the binary mask
// (1 for white pixels, 0 for black).
func buildPrefixSum(src *image.Gray, w, h int) []int32 {
	ps := w + 1
	psum := make([]int32, ps*(h+1))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var v int32
			if src.Pix[y*src.Stride+x] > 0 {
				v = 1
			}
			psum[(y+1)*ps+(x+1)] = v +
				psum[y*ps+(x+1)] +
				psum[(y+1)*ps+x] -
				psum[y*ps+x]
		}
	}
	return psum
}

func boxSum(psum []int32, ps, x0, y0, x1, y1 int) int32 {
	return psum[(y1+1)*ps+(x1+1)] -
		psum[y0*ps+(x1+1)] -
		psum[(y1+1)*ps+x0] +
		psum[y0*ps+x0]
}

// fastBinaryErode erodes a binary image: a pixel survives only if every
// pixel in its r-radius neighborhood is white. Border pixels always erode to black.
func fastBinaryErode(src *image.Gray, r int) *image.Gray {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	psum := buildPrefixSum(src, w, h)
	ps := w + 1
	side := int32(2*r + 1)
	full := side * side
	dst := image.NewGray(image.Rect(0, 0, w, h))
	for y := r; y < h-r; y++ {
		for x := r; x < w-r; x++ {
			if boxSum(psum, ps, x-r, y-r, x+r, y+r) == full {
				dst.Pix[y*dst.Stride+x] = 255
			}
		}
	}
	return dst
}

// fastBinaryDilate dilates a binary image: a pixel becomes white if any
// pixel in its r-radius neighborhood is white.
func fastBinaryDilate(src *image.Gray, r int) *image.Gray {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	psum := buildPrefixSum(src, w, h)
	ps := w + 1
	dst := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			x0, x1 := max(0, x-r), min(w-1, x+r)
			y0, y1 := max(0, y-r), min(h-1, y+r)
			if boxSum(psum, ps, x0, y0, x1, y1) > 0 {
				dst.Pix[y*dst.Stride+x] = 255
			}
		}
	}
	return dst
}

// ComputeDiff runs the full change detection pipeline: AbsDiff → BinaryThreshold
// → MorphologicalOpen. Returns (diffGray, thresholdMask).
func ComputeDiff(before, after *image.NRGBA, threshold uint8) (*image.Gray, *image.Gray) {
	diff := AbsDiff(before, after)
	thresh := BinaryThreshold(diff, threshold)
	thresh = MorphologicalOpen(thresh, 2)
	return diff, thresh
}

