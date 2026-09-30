package imgproc

import (
	"fmt"
	"image"
	"math"

	"github.com/anthonynsimon/bild/blur"
	"github.com/anthonynsimon/bild/transform"
)

// Registration describes how the before image was brought into the after frame.
type Registration struct {
	Mode       string  `json:"mode"`       // "none" | "resize" | "auto" | "manual"
	Applied    bool    `json:"applied"`    // a homography/shift was applied
	Inliers    int     `json:"inliers"`    // feature matches supporting the fit
	Matches    int     `json:"matches"`    // candidate feature matches
	Confidence float64 `json:"confidence"` // 0..1
	ShiftX     int     `json:"shift_x"`    // residual translation applied (analysis px)
	ShiftY     int     `json:"shift_y"`
	Message    string  `json:"message"`
}

// Aligned is an analysis-ready before/after pair in a common pixel frame.
type Aligned struct {
	Before  *image.NRGBA
	After   *image.NRGBA
	Valid   *image.Gray // 255 where Before holds real data (nil = everywhere)
	Resized bool
	Reg     Registration
	// Transform maps pixel coordinates of the original before image into the
	// Before/After frame (resize, then homography, then residual shift).
	// Not set (TransformOK=false) after non-rigid local refinement.
	Transform   [9]float64
	TransformOK bool
}

// RegisterPair brings before into after's frame at analysis resolution.
// Sizes are reconciled first (aspect-preserving when the ratios differ), then,
// if auto is true, a feature-based homography is fitted and a small residual
// translation refined. Every stage is gated: if a fit is implausible the
// pair falls back to the plain resize so a bad registration can never make
// things worse than the old behaviour.
func RegisterPair(before, after *image.NRGBA, auto bool) Aligned {
	return RegisterPairOpts(before, after, auto, false)
}

// RegisterPairOpts is RegisterPair with optional local (non-rigid) refinement.
func RegisterPairOpts(before, after *image.NRGBA, auto, local bool) Aligned {
	out := registerCore(before, after, auto)
	out.TransformOK = true
	if auto && local {
		out.TransformOK = false
		out.Before, out.Valid = RefineLocal(out.Before, out.After, out.Valid)
		if out.Reg.Message != "" {
			out.Reg.Message += "; local refinement applied"
		}
	}
	return out
}

func registerCore(before, after *image.NRGBA, auto bool) Aligned {
	a := DownsampleNRGBA(after, MaxAnalysisDim)
	b, valid, resized := resizeToFrame(before, a.Bounds().Dx(), a.Bounds().Dy())
	out := Aligned{Before: b, After: a, Valid: valid, Resized: resized, Reg: Registration{Mode: "none"},
		Transform: [9]float64(frameTransform(before.Bounds().Dx(), before.Bounds().Dy(), a.Bounds().Dx(), a.Bounds().Dy()))}
	if resized {
		out.Reg.Mode = "resize"
	}
	if !auto {
		out.Reg.Message = "auto-alignment off"
		return out
	}

	est, err := estimateAuto(b, a)
	if err != nil {
		out.Reg.Message = err.Error()
		return refineOnly(out)
	}
	out.Reg.Inliers, out.Reg.Matches, out.Reg.Confidence = len(est.inliers), len(est.matches), est.confidence
	if msg := implausible(est, a.Bounds().Dx(), a.Bounds().Dy()); msg != "" {
		out.Reg.Message = "rejected alignment: " + msg
		return refineOnly(out)
	}
	if nearIdentity(est.h) {
		out.Reg.Message = "images already aligned"
		return refineOnly(out)
	}
	warped, wvalid := WarpHomography(b, est.h, a.Bounds().Dx(), a.Bounds().Dy())
	out.Before = warped
	out.Valid = andMask(out.Valid, wvalid)
	out.Transform = [9]float64(mat3Mul(mat3(est.h), mat3(out.Transform)))
	out.Reg.Mode, out.Reg.Applied = "auto", true
	out.Reg.Message = fmt.Sprintf("aligned with %d feature matches", len(est.inliers))
	return refineOnly(out)
}

// implausible returns a non-empty reason if the homography should not be trusted.
func implausible(est *autoEstimate, w, h int) string {
	if len(est.inliers) < 10 {
		return "too few consistent feature matches"
	}
	if est.avgErr > 2.0 {
		return "feature matches disagree"
	}
	if float64(len(est.inliers))/float64(len(est.matches)) < 0.3 {
		return "too many inconsistent matches"
	}
	// Map the image corners; the result must stay a convex, similarly sized
	// quad (limits scale, shear and perspective).
	corners := []Point{{0, 0}, {float64(w), 0}, {float64(w), float64(h)}, {0, float64(h)}}
	var q [4]Point
	for i, c := range corners {
		x, y := applyHomography(est.h, c.X, c.Y)
		if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
			return "degenerate transform"
		}
		q[i] = Point{x, y}
	}
	var area float64
	for i := 0; i < 4; i++ {
		j := (i + 1) % 4
		area += q[i].X*q[j].Y - q[j].X*q[i].Y
		k := (i + 2) % 4
		cross := (q[j].X-q[i].X)*(q[k].Y-q[j].Y) - (q[j].Y-q[i].Y)*(q[k].X-q[j].X)
		if cross <= 0 {
			return "non-convex transform"
		}
	}
	ratio := math.Abs(area) / 2 / float64(w*h)
	if ratio < 0.6 || ratio > 1.6 {
		return "implausible scale change"
	}
	return ""
}

func nearIdentity(h [9]float64) bool {
	return math.Abs(h[0]-1) < 1e-3 && math.Abs(h[4]-1) < 1e-3 && math.Abs(h[1]) < 1e-3 &&
		math.Abs(h[3]) < 1e-3 && math.Abs(h[2]) < 0.5 && math.Abs(h[5]) < 0.5 &&
		math.Abs(h[6]) < 1e-5 && math.Abs(h[7]) < 1e-5
}

// refineOnly runs the residual translation search on an Aligned pair.
func refineOnly(a Aligned) Aligned {
	limit := 8
	if a.Reg.Applied {
		limit = 3
	}
	dx, dy := EstimateTranslation(a.Before, a.After, limit)
	if dx == 0 && dy == 0 {
		return a
	}
	a.Before, a.Valid = shiftImage(a.Before, a.Valid, dx, dy)
	a.Reg.ShiftX, a.Reg.ShiftY = dx, dy
	a.Transform = [9]float64(mat3Mul(mat3{1, 0, float64(dx), 0, 1, float64(dy), 0, 0, 1}, mat3(a.Transform)))
	if !a.Reg.Applied {
		a.Reg.Mode, a.Reg.Applied = "auto", true
		a.Reg.Message = fmt.Sprintf("corrected %d,%d px camera shift", dx, dy)
	}
	return a
}

// EstimateTranslation searches integer shifts (±limit analysis pixels) that best
// register before onto after, comparing blurred grayscale at half resolution.
// Returns (0,0) unless the shift improves the match by a clear margin.
func EstimateTranslation(before, after *image.NRGBA, limit int) (int, int) {
	const scale = 2
	bg := prepGray(before, scale)
	ag := prepGray(after, scale)
	w, h := bg.Bounds().Dx(), bg.Bounds().Dy()
	if w < 32 || h < 32 {
		return 0, 0
	}
	lim := (limit + scale - 1) / scale
	score := func(dx, dy int) float64 {
		var sum float64
		var n int
		for y := lim; y < h-lim; y += 2 {
			for x := lim; x < w-lim; x += 2 {
				d := float64(ag.Pix[y*ag.Stride+x]) - float64(bg.Pix[(y-dy)*bg.Stride+(x-dx)])
				sum += math.Abs(d)
				n++
			}
		}
		return sum / float64(n)
	}
	base := score(0, 0)
	best, bdx, bdy := base, 0, 0
	for dy := -lim; dy <= lim; dy++ {
		for dx := -lim; dx <= lim; dx++ {
			if s := score(dx, dy); s < best {
				best, bdx, bdy = s, dx, dy
			}
		}
	}
	if best > base*0.93 {
		return 0, 0
	}
	// Refine the half-resolution winner to full resolution (±1 px).
	rx, ry := bdx*scale, bdy*scale
	rs := scoreFull(before, after, rx, ry)
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if s := scoreFull(before, after, bdx*scale+dx, bdy*scale+dy); s < rs {
				rx, ry, rs = bdx*scale+dx, bdy*scale+dy, s
			}
		}
	}
	return rx, ry
}

func scoreFull(before, after *image.NRGBA, dx, dy int) float64 {
	bw, bh := before.Bounds().Dx(), before.Bounds().Dy()
	var sum float64
	var n int
	m := 12
	for y := m; y < bh-m; y += 4 {
		for x := m; x < bw-m; x += 4 {
			sx, sy := x-dx, y-dy
			if sx < 0 || sy < 0 || sx >= bw || sy >= bh {
				continue
			}
			ai := y*after.Stride + x*4
			bi := sy*before.Stride + sx*4
			sum += math.Abs(lumaOf(after.Pix[ai:ai+3]) - lumaOf(before.Pix[bi:bi+3]))
			n++
		}
	}
	if n == 0 {
		return math.MaxFloat64
	}
	return sum / float64(n)
}

func lumaOf(p []uint8) float64 {
	return float64(p[0])*0.299 + float64(p[1])*0.587 + float64(p[2])*0.114
}

// prepGray returns a blurred grayscale copy downscaled by an integer factor.
func prepGray(img *image.NRGBA, scale int) *image.Gray {
	b := img.Bounds()
	w, h := b.Dx()/scale, b.Dy()/scale
	g := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sum float64
			for j := 0; j < scale; j++ {
				for i := 0; i < scale; i++ {
					p := (y*scale+j)*img.Stride + (x*scale+i)*4
					sum += lumaOf(img.Pix[p : p+3])
				}
			}
			g.Pix[y*g.Stride+x] = uint8(sum/float64(scale*scale) + 0.5)
		}
	}
	return rgbaToGray(blur.Gaussian(g, 1.5))
}

// shiftImage translates img by (dx,dy), filling exposed edges with black and
// clearing them in the validity mask.
func shiftImage(img *image.NRGBA, valid *image.Gray, dx, dy int) (*image.NRGBA, *image.Gray) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	nv := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		sy := y - dy
		if sy < 0 || sy >= h {
			continue
		}
		for x := 0; x < w; x++ {
			sx := x - dx
			if sx < 0 || sx >= w {
				continue
			}
			copy(out.Pix[y*out.Stride+x*4:y*out.Stride+x*4+4], img.Pix[sy*img.Stride+sx*4:sy*img.Stride+sx*4+4])
			if valid == nil {
				nv.Pix[y*nv.Stride+x] = 255
			} else {
				nv.Pix[y*nv.Stride+x] = valid.Pix[sy*valid.Stride+sx]
			}
		}
	}
	return out, nv
}

func andMask(a, b *image.Gray) *image.Gray {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	out := image.NewGray(a.Bounds())
	for i := range out.Pix {
		if a.Pix[i] != 0 && b.Pix[i] != 0 {
			out.Pix[i] = 255
		}
	}
	return out
}

// frameTransform is the affine map resizeToFrame applies to an iw×ih image.
func frameTransform(iw, ih, w, h int) mat3 {
	if iw == w && ih == h {
		return mat3{1, 0, 0, 0, 1, 0, 0, 0, 1}
	}
	ar := (float64(iw) / float64(ih)) / (float64(w) / float64(h))
	if math.Abs(ar-1) < 0.02 {
		return mat3{float64(w) / float64(iw), 0, 0, 0, float64(h) / float64(ih), 0, 0, 0, 1}
	}
	s := math.Min(float64(w)/float64(iw), float64(h)/float64(ih))
	nw, nh := max(1, int(float64(iw)*s+0.5)), max(1, int(float64(ih)*s+0.5))
	return mat3{s, 0, float64((w - nw) / 2), 0, s, float64((h - nh) / 2), 0, 0, 1}
}

// resizeToFrame scales img to w×h. When the aspect ratios agree within 2% it
// stretches (the old behaviour, fixing rounding-level size differences);
// otherwise it scales uniformly and letterboxes, returning a mask of the real
// pixels so the padding never registers as change.
func resizeToFrame(img *image.NRGBA, w, h int) (*image.NRGBA, *image.Gray, bool) {
	iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
	if iw == w && ih == h {
		return img, nil, false
	}
	ar := (float64(iw) / float64(ih)) / (float64(w) / float64(h))
	if math.Abs(ar-1) < 0.02 {
		return rgbaToNRGBA(transform.Resize(img, w, h, transform.Lanczos)), nil, true
	}
	s := math.Min(float64(w)/float64(iw), float64(h)/float64(ih))
	nw, nh := max(1, int(float64(iw)*s+0.5)), max(1, int(float64(ih)*s+0.5))
	scaled := rgbaToNRGBA(transform.Resize(img, nw, nh, transform.Lanczos))
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	valid := image.NewGray(image.Rect(0, 0, w, h))
	ox, oy := (w-nw)/2, (h-nh)/2
	for y := 0; y < nh; y++ {
		copy(out.Pix[(y+oy)*out.Stride+ox*4:], scaled.Pix[y*scaled.Stride:y*scaled.Stride+nw*4])
		for x := 0; x < nw; x++ {
			valid.Pix[(y+oy)*valid.Stride+ox+x] = 255
		}
	}
	for i := 3; i < len(out.Pix); i += 4 {
		out.Pix[i] = 255
	}
	return out, valid, true
}
