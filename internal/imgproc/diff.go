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
// binary mask using a square size×size structuring element.
// size=1 is a no-op. Even sizes are supported (kernel is slightly left/top biased).
// Uses 2D prefix sums for O(w·h) complexity independent of size.
func MorphologicalOpen(mask *image.Gray, size int) *image.Gray {
	if size <= 1 {
		return mask
	}
	return fastBinaryDilate(fastBinaryErode(mask, size), size)
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

// fastBinaryErode erodes a binary image using a size×size structuring element.
// A pixel survives only if every pixel in the kernel window is white.
// half = size/2 so that odd sizes are centered and even sizes are left/top biased.
func fastBinaryErode(src *image.Gray, size int) *image.Gray {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	psum := buildPrefixSum(src, w, h)
	ps := w + 1
	full := int32(size * size)
	half := size / 2
	tail := size - 1 - half // for odd: tail==half; for even: tail==half-1
	dst := image.NewGray(image.Rect(0, 0, w, h))
	for y := half; y < h-tail; y++ {
		for x := half; x < w-tail; x++ {
			if boxSum(psum, ps, x-half, y-half, x+tail, y+tail) == full {
				dst.Pix[y*dst.Stride+x] = 255
			}
		}
	}
	return dst
}

// fastBinaryDilate dilates a binary image using a size×size structuring element.
// A pixel becomes white if any pixel in its kernel window is white.
func fastBinaryDilate(src *image.Gray, size int) *image.Gray {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	psum := buildPrefixSum(src, w, h)
	ps := w + 1
	half := size / 2
	tail := size - 1 - half
	dst := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			x0, x1 := max(0, x-half), min(w-1, x+tail)
			y0, y1 := max(0, y-half), min(h-1, y+tail)
			if boxSum(psum, ps, x0, y0, x1, y1) > 0 {
				dst.Pix[y*dst.Stride+x] = 255
			}
		}
	}
	return dst
}

// FilterByMinRegionSize removes connected components smaller than minPx pixels
// from a binary mask. Unlike morphological opening, this preserves thin objects
// (e.g. wires, cables) as long as they have enough total pixel area.
func FilterByMinRegionSize(mask *image.Gray, minPx int) *image.Gray {
	b := mask.Bounds()
	w, h := b.Dx(), b.Dy()

	labels := make([]int, w*h)
	sizes := []int{0} // index 0 unused; label IDs start at 1
	visited := make([]bool, w*h)
	dirs := [4][2]int{{0, 1}, {0, -1}, {1, 0}, {-1, 0}}

	// BFS flood-fill to label each connected component and record its size.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			if visited[idx] || mask.Pix[y*mask.Stride+x] == 0 {
				continue
			}
			label := len(sizes)
			sizes = append(sizes, 0)
			queue := [][2]int{{x, y}}
			visited[idx] = true
			labels[idx] = label
			for len(queue) > 0 {
				p := queue[0]
				queue = queue[1:]
				sizes[label]++
				for _, d := range dirs {
					nx, ny := p[0]+d[0], p[1]+d[1]
					if nx < 0 || nx >= w || ny < 0 || ny >= h {
						continue
					}
					nidx := ny*w + nx
					if visited[nidx] || mask.Pix[ny*mask.Stride+nx] == 0 {
						continue
					}
					visited[nidx] = true
					labels[nidx] = label
					queue = append(queue, [2]int{nx, ny})
				}
			}
		}
	}

	out := image.NewGray(b)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			label := labels[y*w+x]
			if label > 0 && sizes[label] >= minPx {
				out.Pix[y*out.Stride+x] = 255
			}
		}
	}
	return out
}

// ComputeDiff runs the full change detection pipeline:
//
//	AbsDiff → BinaryThreshold → MorphologicalOpen(morphRadius) → FilterByMinRegionSize(minRegion)
//
// morphSize is the side length of the square structuring element used for
// morphological opening (1=off, 2=2×2, 3=3×3, 5=5×5, …). A pixel must be
// part of a solid morphSize×morphSize block to survive erosion, which suppresses
// minor camera-shake artifacts. Set to 1 to skip.
//
// minRegion is the minimum connected-component size in pixels. Regions smaller
// than this are discarded as noise after morphological opening. Set to 1 to skip.
func ComputeDiff(before, after *image.NRGBA, threshold uint8, morphSize, minRegion int) (*image.Gray, *image.Gray) {
	diff := AbsDiff(before, after)
	thresh := BinaryThreshold(diff, threshold)
	if morphSize > 1 {
		thresh = MorphologicalOpen(thresh, morphSize)
	}
	if minRegion > 1 {
		thresh = FilterByMinRegionSize(thresh, minRegion)
	}
	return diff, thresh
}

