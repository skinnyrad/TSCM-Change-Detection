package imgproc

import (
	"image"

	"github.com/anthonynsimon/bild/transform"
)

// MaxAnalysisDim is the longest edge (pixels) images are downsampled to before
// running diff/encode operations. Full-resolution images are kept in state for
// display serving and warp input; only the analysis pair is capped here.
const MaxAnalysisDim = 1920

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

// ResizeNRGBA resizes img to exactly w×h with bilinear resampling.
func ResizeNRGBA(img *image.NRGBA, w, h int) *image.NRGBA {
	if img.Bounds().Dx() == w && img.Bounds().Dy() == h {
		return img
	}
	return rgbaToNRGBA(transform.Resize(img, w, h, transform.Linear))
}

// ResizeMask resizes a binary validity mask to w×h using nearest-neighbour.
func ResizeMask(m *image.Gray, w, h int) *image.Gray {
	sw, sh := m.Bounds().Dx(), m.Bounds().Dy()
	if sw == w && sh == h {
		return m
	}
	out := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			out.Pix[y*out.Stride+x] = m.Pix[(y*sh/h)*m.Stride+x*sw/w]
		}
	}
	return out
}
