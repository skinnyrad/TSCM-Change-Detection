package imgproc

import (
	"image"

	"github.com/anthonynsimon/bild/transform"
)

// MaxAnalysisDim is the longest edge (pixels) images are downsampled to before
// running diff/encode operations. Full-resolution images are kept in state for
// display serving and warp input; only the analysis pair is capped here.
const MaxAnalysisDim = 1920

// Align resizes before to match after's dimensions if they differ.
// Returns (alignedBefore, after, wasResized).
// Uses Lanczos resampling to match OpenCV's INTER_LANCZOS4.
func Align(before, after image.Image) (*image.NRGBA, *image.NRGBA, bool) {
	ab := before.Bounds()
	bb := after.Bounds()

	afterNRGBA := toNRGBA(after)

	if ab.Dx() == bb.Dx() && ab.Dy() == bb.Dy() {
		return toNRGBA(before), afterNRGBA, false
	}

	resized := transform.Resize(before, bb.Dx(), bb.Dy(), transform.Lanczos)
	return rgbaToNRGBA(resized), afterNRGBA, true
}

// AlignNRGBA is like Align but accepts already-converted *image.NRGBA inputs,
// avoiding redundant type conversions when images have already been decoded.
func AlignNRGBA(before, after *image.NRGBA) (*image.NRGBA, *image.NRGBA, bool) {
	ab := before.Bounds()
	bb := after.Bounds()

	if ab.Dx() == bb.Dx() && ab.Dy() == bb.Dy() {
		return before, after, false
	}

	resized := transform.Resize(before, bb.Dx(), bb.Dy(), transform.Lanczos)
	return rgbaToNRGBA(resized), after, true
}

// DownsampleNRGBA resizes img so its longest dimension is at most maxDim.
// Returns img unchanged if it already fits. Uses bilinear resampling (fast).
func DownsampleNRGBA(img *image.NRGBA, maxDim int) *image.NRGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxDim && h <= maxDim {
		return img
	}
	var nw, nh int
	if w >= h {
		nw = maxDim
		nh = int(float64(h)*float64(maxDim)/float64(w) + 0.5)
	} else {
		nh = maxDim
		nw = int(float64(w)*float64(maxDim)/float64(h) + 0.5)
	}
	if nh < 1 {
		nh = 1
	}
	if nw < 1 {
		nw = 1
	}
	return rgbaToNRGBA(transform.Resize(img, nw, nh, transform.Linear))
}
