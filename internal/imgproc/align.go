package imgproc

import (
	"image"

	"github.com/anthonynsimon/bild/transform"
)

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
