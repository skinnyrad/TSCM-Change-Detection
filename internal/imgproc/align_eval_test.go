package imgproc

import (
	"image"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestEvalAlignment measures auto-alignment against known ground truth: each
// source photo is cropped (the "before") and re-rendered through a random
// homography with a lighting change and a pasted "changed object" (the
// "after"). We then compare the estimated transform with the true one.
//
// Set ALIGN_IMAGES to a directory tree of photos (searched recursively for
// *.png/*.jpg); it is skipped otherwise:
//
//	ALIGN_IMAGES=~/.../change-detection go test ./internal/imgproc -run TestEvalAlignment -v
func TestEvalAlignment(t *testing.T) {
	root := os.Getenv("ALIGN_IMAGES")
	if root == "" {
		t.Skip("ALIGN_IMAGES not set")
	}
	var paths []string
	filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && (filepath.Ext(p) == ".png" || filepath.Ext(p) == ".jpg") &&
			!contains(p, "solution.png") && !contains(p, "/venv/") {
			paths = append(paths, p)
		}
		return nil
	})
	sort.Strings(paths)
	if len(paths) > 40 {
		paths = paths[:40]
	}
	rng := rand.New(rand.NewSource(42))
	var fullErrs, pairErrs, spreads []float64
	fails := 0
	for _, p := range paths {
		src, err := loadNRGBA(p)
		if err != nil {
			continue
		}
		src = DownsampleNRGBA(src, 1600)
		for k := 0; k < 2; k++ {
			before, after, htrue := synthPair(src, rng)
			w, h := before.Bounds().Dx(), before.Bounds().Dy()
			est, err := estimateAuto(before, after)
			if err != nil {
				fails++
				t.Logf("%s #%d: FAIL %v", filepath.Base(p), k, err)
				continue
			}
			fe := homographyError(est.h, htrue, w, h)
			res, err := AutoDetectHomography(before, after)
			pe, sp := math.Inf(1), 0.0
			if err == nil {
				var s, d []Point
				for _, pr := range res.Pairs {
					s, d = append(s, pr.Src), append(d, pr.Dst)
				}
				if hp, err := computeHomography(s, d); err == nil {
					pe = homographyError(hp, htrue, w, h)
				}
				sp = spreadScore(s, w, h)
			}
			fullErrs, pairErrs, spreads = append(fullErrs, fe), append(pairErrs, pe), append(spreads, sp)
			t.Logf("%-14s #%d inl=%3d full=%6.2fpx pairs=%7.2fpx spread=%.2f", filepath.Base(p), k, len(est.inliers), fe, pe, sp)
		}
	}
	t.Logf("SUMMARY n=%d fails=%d | full: median %.2f p90 %.2f | 8-pair: median %.2f p90 %.2f | spread median %.2f",
		len(fullErrs)+fails, fails, pct(fullErrs, 0.5), pct(fullErrs, 0.9), pct(pairErrs, 0.5), pct(pairErrs, 0.9), pct(spreads, 0.5))
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

// synthPair crops the centre of src as "before" and renders "after" through a
// random, mild homography (rotation ±3°, scale ±6%, shift ±4%, slight
// perspective), with a gain/offset lighting change and a pasted patch.
func synthPair(src *image.NRGBA, rng *rand.Rand) (*image.NRGBA, *image.NRGBA, [9]float64) {
	W, H := src.Bounds().Dx(), src.Bounds().Dy()
	cx, cy := W/10, H/10
	w, h := W-2*cx, H-2*cy
	before := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		copy(before.Pix[y*before.Stride:y*before.Stride+w*4], src.Pix[(y+cy)*src.Stride+cx*4:])
	}
	u := func(a float64) float64 { return (rng.Float64()*2 - 1) * a }
	th, s := u(3)*math.Pi/180, 1+u(0.06)
	tx, ty := u(0.04)*float64(w), u(0.04)*float64(h)
	// Rotate/scale about the centre, then translate, then add perspective.
	mx, my := float64(w)/2, float64(h)/2
	c, sn := s*math.Cos(th), s*math.Sin(th)
	a := mat3{c, -sn, mx - c*mx + sn*my + tx, sn, c, my - sn*mx - c*my + ty, 0, 0, 1}
	p := mat3{1, 0, 0, 0, 1, 0, u(0.03) / float64(w), u(0.03) / float64(h), 1}
	htrue := mat3Mul(p, a)
	// after(q) = src(C + Htrue⁻¹ q)  ⇒  warp src with Htrue·T(-cx,-cy).
	hsrc := mat3Mul(htrue, mat3{1, 0, -float64(cx), 0, 1, -float64(cy), 0, 0, 1})
	after, _ := WarpHomography(src, [9]float64(hsrc), w, h)
	gain, off := 0.8+rng.Float64()*0.35, u(20)
	for i := 0; i < len(after.Pix); i += 4 {
		for ch := 0; ch < 3; ch++ {
			after.Pix[i+ch] = clampU8(float64(after.Pix[i+ch])*gain + off)
		}
	}
	// Paste a patch from elsewhere: a "change" the matcher must ignore.
	pw, ph := w/6, h/6
	sx, sy := rng.Intn(w-pw), rng.Intn(h-ph)
	dx, dy := rng.Intn(w-pw), rng.Intn(h-ph)
	for y := 0; y < ph; y++ {
		copy(after.Pix[(dy+y)*after.Stride+dx*4:(dy+y)*after.Stride+(dx+pw)*4], before.Pix[(sy+y)*before.Stride+sx*4:])
	}
	return before, after, [9]float64(htrue)
}

// homographyError is the mean distance between where two homographies send a
// grid of points spanning the image.
func homographyError(a, b [9]float64, w, h int) float64 {
	var sum float64
	n := 0
	for gy := 0; gy <= 4; gy++ {
		for gx := 0; gx <= 4; gx++ {
			x, y := float64(w)*float64(gx)/4, float64(h)*float64(gy)/4
			ax, ay := applyHomography(a, x, y)
			bx, by := applyHomography(b, x, y)
			sum += math.Hypot(ax-bx, ay-by)
			n++
		}
	}
	return sum / float64(n)
}

// spreadScore is the fraction of a 3×3 grid of image cells containing a point.
func spreadScore(pts []Point, w, h int) float64 {
	var cells [9]bool
	for _, p := range pts {
		cx, cy := min(int(3*p.X/float64(w)), 2), min(int(3*p.Y/float64(h)), 2)
		if cx >= 0 && cy >= 0 {
			cells[cy*3+cx] = true
		}
	}
	n := 0
	for _, c := range cells {
		if c {
			n++
		}
	}
	return float64(n) / 9
}

func pct(v []float64, q float64) float64 {
	if len(v) == 0 {
		return math.NaN()
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	return s[min(int(q*float64(len(s))), len(s)-1)]
}
