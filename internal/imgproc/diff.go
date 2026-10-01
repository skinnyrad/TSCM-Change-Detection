package imgproc

import (
	"image"
	"math"

	"github.com/anthonynsimon/bild/blur"
)

// Rect is an axis-aligned rectangle in fractions (0..1) of the image size.
type Rect struct{ X0, Y0, X1, Y1 float64 }

// DiffOptions controls the change detection pipeline used by ComputeDiffV2.
// Zero values disable optional stages; NormalizeLuma and PreBlurSigma have active
// defaults that must be set explicitly.
type DiffOptions struct {
	Threshold      uint8
	AutoThreshold  bool    // derive the threshold from the noise floor (Threshold acts as a floor offset)
	MorphSize      int     // open kernel side length (1=off)
	CloseSize      int     // close kernel side length (1=off)
	MinRegion      int     // minimum connected-component size in pixels (1=off)
	PreBlurSigma   float64 // Gaussian σ applied to colour images before differencing (0=off)
	NormalizeLuma  bool    // shift per-image mean luma to 128 before diff
	MatchIntensity bool    // per-channel gain/offset fit of before onto after (supersedes NormalizeLuma)
	LocalLight     float64 // match lighting locally over windows of this fraction of the diagonal (0=off)
	ColorWeight    float64 // weight of CIELAB a/b distance relative to lightness (0 = luma only)
	ShiftTol       int     // tolerate ±N px residual misalignment (0=off)
	BorderPct      float64 // ignore this fraction of each edge (0=off)

	Valid  *image.Gray // pixels where the before image holds real data (nil = all)
	Spread *image.Gray // per-pixel baseline variability subtracted from the diff (nil = none)
	Ignore []Rect      // user-drawn exclusion zones
}

// DiffResult is the output of ComputeDiffV2.
type DiffResult struct {
	Diff      *image.Gray // per-pixel change magnitude, 0..255
	Mask      *image.Gray // thresholded, cleaned change mask
	Threshold uint8       // threshold actually used (differs from Threshold when auto)
}

// NormalizeLuma returns a copy of img with each pixel's RGB channels shifted so
// the image mean luma equals 128. Compensates for global brightness drift between
// shots without affecting local contrast. Uses Rec. 601 luma weights.
func NormalizeLuma(img *image.NRGBA) *image.NRGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	// Pass 1: compute mean luma.
	var sum float64
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*img.Stride + x*4
			luma := float64(img.Pix[i])*0.299 +
				float64(img.Pix[i+1])*0.587 +
				float64(img.Pix[i+2])*0.114
			sum += luma
		}
	}
	meanLuma := sum / float64(w*h)
	delta := 128.0 - meanLuma
	// Pass 2: apply delta to RGB, preserving alpha.
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*img.Stride + x*4
			o := y*out.Stride + x*4
			out.Pix[o] = clampU8(float64(img.Pix[i]) + delta)
			out.Pix[o+1] = clampU8(float64(img.Pix[i+1]) + delta)
			out.Pix[o+2] = clampU8(float64(img.Pix[i+2]) + delta)
			out.Pix[o+3] = img.Pix[i+3]
		}
	}
	return out
}

// GaussianBlurNRGBA applies a Gaussian blur with radius sigma to an NRGBA image.
// sigma=0 returns img unchanged. Wraps bild/blur.Gaussian.
func GaussianBlurNRGBA(img *image.NRGBA, sigma float64) *image.NRGBA {
	if sigma <= 0 {
		return img
	}
	blurred := blur.Gaussian(img, sigma) // returns *image.RGBA
	return rgbaToNRGBA(blurred)
}

// MorphologicalClose applies morphological closing (dilation then erosion) to a
// binary mask using a square size×size structuring element.
// Fills small holes within detected regions without expanding their outer boundary.
// size=1 is a no-op.
func MorphologicalClose(mask *image.Gray, size int) *image.Gray {
	if size <= 1 {
		return mask
	}
	return fastBinaryErode(fastBinaryDilate(mask, size), size)
}

// PrepareImages applies the intensity-normalization and blur preprocessing
// steps from opts to copies of before and after. Use this when you need
// preprocessed images without running the full diff pipeline (e.g. Subtract).
func PrepareImages(before, after *image.NRGBA, opts DiffOptions) (*image.NRGBA, *image.NRGBA) {
	b, a := before, after
	switch {
	case opts.MatchIntensity:
		b = MatchIntensity(b, a, opts.Valid)
	case opts.NormalizeLuma:
		b = NormalizeLuma(b)
		a = NormalizeLuma(a)
	}
	if opts.LocalLight > 0 {
		b = MatchLocalLight(b, a, opts.Valid, opts.LocalLight)
	}
	if opts.PreBlurSigma > 0 {
		b = GaussianBlurNRGBA(b, opts.PreBlurSigma)
		a = GaussianBlurNRGBA(a, opts.PreBlurSigma)
	}
	return b, a
}

// ComputeDiffV2 runs the change detection pipeline.
//
//	[match intensity | normalize luma] → [blur] → Lab diff (shift-tolerant) →
//	[− baseline spread] → [zero invalid/border/ignored] → threshold (fixed or
//	auto) → open → close → min region
//
// Does not mutate the input images.
func ComputeDiffV2(before, after *image.NRGBA, opts DiffOptions) DiffResult {
	b, a := PrepareImages(before, after, opts)
	diff := labDiff(toLab(b), toLab(a), opts.ColorWeight, opts.ShiftTol)
	if opts.Spread != nil {
		subtractSpread(diff, opts.Spread, 3)
	}
	applyExclusions(diff, opts)

	thr := opts.Threshold
	if opts.AutoThreshold {
		thr = autoThreshold(diff, opts.Valid, opts.Threshold)
	}
	thresh := BinaryThreshold(diff, thr)
	if opts.MorphSize > 1 {
		thresh = MorphologicalOpen(thresh, opts.MorphSize)
	}
	if opts.CloseSize > 1 {
		thresh = MorphologicalClose(thresh, opts.CloseSize)
	}
	if opts.MinRegion > 1 {
		thresh = FilterByMinRegionSize(thresh, opts.MinRegion)
	}
	return DiffResult{Diff: diff, Mask: thresh, Threshold: thr}
}

// labDiff computes the per-pixel CIELAB distance (scaled so a full lightness
// swing is 255). colorWeight=0 compares lightness only. With shiftTol>0 each
// pixel takes the minimum distance over the ±shiftTol neighbourhood of before,
// which suppresses edge halos caused by sub-pixel/few-pixel misalignment.
func labDiff(b, a labPlanes, colorWeight float64, shiftTol int) *image.Gray {
	w, h := a.W, a.H
	out := image.NewGray(image.Rect(0, 0, w, h))
	cw := float32(colorWeight)
	const scale = 2.55
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ai := y*w + x
			best := float32(math.MaxFloat32)
			for dy := -shiftTol; dy <= shiftTol; dy++ {
				yy := min(max(y+dy, 0), h-1)
				for dx := -shiftTol; dx <= shiftTol; dx++ {
					xx := min(max(x+dx, 0), w-1)
					bi := yy*w + xx
					dl := b.L[bi] - a.L[ai]
					d := dl * dl
					if cw > 0 {
						da, db := b.A[bi]-a.A[ai], b.B[bi]-a.B[ai]
						d += cw * (da*da + db*db)
					}
					if d < best {
						best = d
					}
				}
			}
			v := float32(math.Sqrt(float64(best))) * scale
			if v > 255 {
				v = 255
			}
			out.Pix[y*out.Stride+x] = uint8(v + 0.5)
		}
	}
	return out
}

// subtractSpread lowers diff by k× the baseline spread, so pixels that vary
// naturally between baseline sweeps need a larger change to register.
func subtractSpread(diff, spread *image.Gray, k float64) {
	for y := 0; y < diff.Bounds().Dy(); y++ {
		for x := 0; x < diff.Bounds().Dx(); x++ {
			d := float64(diff.Pix[y*diff.Stride+x]) - k*float64(spread.Pix[y*spread.Stride+x])
			if d < 0 {
				d = 0
			}
			diff.Pix[y*diff.Stride+x] = uint8(d)
		}
	}
}

// applyExclusions zeroes diff where the before image has no data (eroded so
// interpolated warp edges are excluded), near the borders, or inside
// user-drawn ignore rectangles.
func applyExclusions(diff *image.Gray, opts DiffOptions) {
	w, h := diff.Bounds().Dx(), diff.Bounds().Dy()
	if opts.Valid != nil {
		inner := fastBinaryErode(opts.Valid, 2*(int(math.Ceil(opts.PreBlurSigma*2))+opts.ShiftTol+1)+1)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if inner.Pix[y*inner.Stride+x] == 0 {
					diff.Pix[y*diff.Stride+x] = 0
				}
			}
		}
	}
	zero := func(x0, y0, x1, y1 int) {
		for y := max(y0, 0); y < min(y1, h); y++ {
			for x := max(x0, 0); x < min(x1, w); x++ {
				diff.Pix[y*diff.Stride+x] = 0
			}
		}
	}
	if opts.BorderPct > 0 {
		bx, by := int(float64(w)*opts.BorderPct), int(float64(h)*opts.BorderPct)
		zero(0, 0, w, by)
		zero(0, h-by, w, h)
		zero(0, 0, bx, h)
		zero(w-bx, 0, w, h)
	}
	for _, r := range opts.Ignore {
		zero(int(r.X0*float64(w)), int(r.Y0*float64(h)), int(math.Ceil(r.X1*float64(w))), int(math.Ceil(r.Y1*float64(h))))
	}
}

// autoThreshold picks a threshold from the diff's own noise floor:
// median + (6 + offset/10)·σ, where σ is the robust MAD estimate over valid,
// non-excluded pixels. offset is the user's strength slider; higher values
// demand larger changes. The result is clamped to [8, 99].
func autoThreshold(diff *image.Gray, valid *image.Gray, offset uint8) uint8 {
	var hist [256]int
	total := 0
	for y := 0; y < diff.Bounds().Dy(); y++ {
		for x := 0; x < diff.Bounds().Dx(); x++ {
			if valid != nil && valid.Pix[y*valid.Stride+x] == 0 {
				continue
			}
			hist[diff.Pix[y*diff.Stride+x]]++
			total++
		}
	}
	if total == 0 {
		return offset
	}
	quantile := func(q float64) int {
		target, acc := int(q*float64(total)), 0
		for v, n := range hist {
			acc += n
			if acc > target {
				return v
			}
		}
		return 255
	}
	med := quantile(0.5)
	// MAD: median of |v - med|.
	var devHist [256]int
	for v, n := range hist {
		devHist[int(math.Abs(float64(v-med)))] += n
	}
	acc, mad := 0, 0
	for v, n := range devHist {
		acc += n
		if acc > total/2 {
			mad = v
			break
		}
	}
	sigma := math.Max(1.4826*float64(mad), 1)
	t := float64(med) + (6+float64(offset)/10)*sigma
	return uint8(math.Max(8, math.Min(99, t)))
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
	labels, regions := components(mask)
	out := image.NewGray(b)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if l := labels[y*w+x]; l > 0 && regions[l-1].Area >= minPx {
				out.Pix[y*out.Stride+x] = 255
			}
		}
	}
	return out
}
