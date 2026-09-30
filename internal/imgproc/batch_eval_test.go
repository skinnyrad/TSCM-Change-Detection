package imgproc

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestEvalBatch runs batch anomaly detection on a folder of images of the same
// scene and, if BATCH_TRUTH points at the x-ray reference results.json, scores
// image-level detection and localization against it.
//
//	BATCH_DIR=tmp/xrays/xrays BATCH_TRUTH=~/.../xray_analysis/results.json \
//	  go test ./internal/imgproc -run TestEvalBatch -v
func TestEvalBatch(t *testing.T) {
	dir := os.Getenv("BATCH_DIR")
	if dir == "" {
		t.Skip("BATCH_DIR not set")
	}
	paths, _ := filepath.Glob(filepath.Join(dir, "*.png"))
	jp, _ := filepath.Glob(filepath.Join(dir, "*.jpg"))
	paths = append(paths, jp...)
	sort.Strings(paths)
	var imgs []*image.NRGBA
	var names []string
	var dims []image.Point
	for _, p := range paths {
		im, err := loadNRGBA(p)
		if err != nil {
			continue
		}
		dims = append(dims, im.Bounds().Size())
		imgs = append(imgs, DownsampleNRGBA(im, 512))
		names = append(names, filepath.Base(p))
	}
	start := time.Now()
	res := AnalyzeBatch(imgs, BatchOptions{}, nil)
	t.Logf("analyzed %d images in %s; anchor=%s golden=%d unregistered=%d median=%.2f spread=%.2f",
		len(imgs), time.Since(start).Round(time.Millisecond), names[res.Anchor], res.GoldenCount, res.Unregistered, res.ScoreMedian, res.ScoreSpread)

	type truth struct {
		anomalous bool
		boxes     [][4]float64
	}
	gt := map[string]*truth{}
	if tp := os.Getenv("BATCH_TRUTH"); tp != "" {
		raw, err := os.ReadFile(tp)
		if err != nil {
			t.Fatal(err)
		}
		var r struct {
			Images []struct {
				File      string
				Anomalous bool
			}
			Findings []struct {
				File    string
				Regions []struct{ Box [4]float64 }
			}
		}
		if err := json.Unmarshal(raw, &r); err != nil {
			t.Fatal(err)
		}
		for _, im := range r.Images {
			gt[im.File] = &truth{anomalous: im.Anomalous}
		}
		for _, f := range r.Findings {
			for _, reg := range f.Regions {
				gt[f.File].boxes = append(gt[f.File].boxes, reg.Box)
			}
		}
	}

	order := make([]int, len(imgs))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(a, b int) bool { return res.Images[order[a]].Z > res.Images[order[b]].Z })
	tp, fp, fn, locHit, locTotal := 0, 0, 0, 0, 0
	for rank, i := range order {
		im := res.Images[i]
		g := gt[names[i]]
		mark := ""
		if g != nil {
			switch {
			case g.anomalous && im.Anomaly:
				tp++
				mark = "TP"
			case !g.anomalous && im.Anomaly:
				fp++
				mark = "FP"
			case g.anomalous && !im.Anomaly:
				fn++
				mark = "FN"
			}
			// Localization: does any of our regions overlap a reference box?
			if g.anomalous && len(g.boxes) > 0 {
				hit := 0
				for _, b := range g.boxes {
					if regionsHitTruth(res, i, dims[i], [][4]float64{b}) {
						hit++
					}
				}
				locTotal += len(g.boxes)
				locHit += hit
				mark += fmt.Sprintf(" boxes %d/%d", hit, len(g.boxes))
			}
		}
		if rank < 14 || mark == "FN" || strings.HasPrefix(mark, "FP") {
			t.Logf("#%2d %-38s z=%8.1f score=%6.2f reg=%-6s regions=%d %s", rank+1, names[i], im.Z, im.Score, im.Reg.Mode, len(im.Regions), mark)
		}
	}
	if len(gt) > 0 {
		t.Logf("IMAGE LEVEL: TP=%d FP=%d FN=%d | LOCALIZATION: %d/%d reference boxes covered by a detected region", tp, fp, fn, locHit, locTotal)
	}
}

// regionsHitTruth maps reference boxes (original-image pixels) into the stats
// frame and checks overlap with any detected region.
func regionsHitTruth(res *BatchResult, i int, orig image.Point, boxes [][4]float64) bool {
	im := res.Images[i]
	f := math.Min(1, 512/math.Max(float64(orig.X), float64(orig.Y))) // orig → work
	sx, sy := float64(res.W)/float64(res.FrameW), float64(res.H)/float64(res.FrameH)
	for _, b := range boxes {
		var r image.Rectangle
		first := true
		for _, c := range [][2]float64{{b[0], b[1]}, {b[2], b[1]}, {b[0], b[3]}, {b[2], b[3]}} {
			x, y := applyHomography(im.Transform, c[0]*f, c[1]*f)
			p := image.Pt(int(x*sx), int(y*sy))
			if first {
				r, first = image.Rectangle{Min: p, Max: p.Add(image.Pt(1, 1))}, false
			} else {
				r = r.Union(image.Rectangle{Min: p, Max: p.Add(image.Pt(1, 1))})
			}
		}
		for _, reg := range im.Regions {
			if reg.Box.Overlaps(r) {
				return true
			}
		}
	}
	return false
}

// TestEvalBatchSynthetic builds a handheld-style batch from one photo (random
// camera jitter, exposure/white-balance drift, sensor noise) and plants small
// objects (patches cut from a second photo) in a few shots, then checks that
// exactly those shots are flagged and the planted objects are localized.
//
//	BATCH_SCENE=test-images/kitchen1.png BATCH_DONOR=test-images/glasses1.jpg \
//	  go test ./internal/imgproc -run TestEvalBatchSynthetic -v
func TestEvalBatchSynthetic(t *testing.T) {
	scene, donorPath := os.Getenv("BATCH_SCENE"), os.Getenv("BATCH_DONOR")
	if scene == "" || donorPath == "" {
		t.Skip("BATCH_SCENE/BATCH_DONOR not set")
	}
	src, err := loadNRGBA(scene)
	if err != nil {
		t.Fatal(err)
	}
	donor, err := loadNRGBA(donorPath)
	if err != nil {
		t.Fatal(err)
	}
	src = DownsampleNRGBA(src, 700)
	donor = DownsampleNRGBA(donor, 700)
	rng := rand.New(rand.NewSource(7))
	const n, planted = 60, 6
	var imgs []*image.NRGBA
	truth := map[int]image.Rectangle{}
	for i := 0; i < n; i++ {
		after := jitterView(src, rng)
		if i%(n/planted) == 3 && os.Getenv("BATCH_WIRE") != "" {
			truth[i] = plantWire(after, rng)
		} else if i%(n/planted) == 3 {
			w, h := after.Bounds().Dx(), after.Bounds().Dy()
			div := 10
			if v, err := strconv.Atoi(os.Getenv("BATCH_OBJ_DIV")); err == nil && v > 0 {
				div = v // object size as a fraction of the frame: 1/div
			}
			pw, ph := w/div+rng.Intn(w/(2*div)+1), h/div+rng.Intn(h/(2*div)+1)
			x0, y0 := w/8+rng.Intn(w*3/4-pw), h/8+rng.Intn(h*3/4-ph)
			sx, sy := rng.Intn(donor.Bounds().Dx()-pw), rng.Intn(donor.Bounds().Dy()-ph)
			for y := 0; y < ph; y++ {
				copy(after.Pix[(y0+y)*after.Stride+x0*4:(y0+y)*after.Stride+(x0+pw)*4], donor.Pix[(sy+y)*donor.Stride+sx*4:])
			}
			truth[i] = image.Rect(x0, y0, x0+pw, y0+ph)
		}
		imgs = append(imgs, after)
	}
	start := time.Now()
	sd, _ := strconv.Atoi(os.Getenv("BATCH_STATS_DIM"))
	res := AnalyzeBatch(imgs, BatchOptions{StatsDim: sd}, nil)
	t.Logf("analyzed %d in %s; golden=%d unregistered=%d threshold=%.2f median=%.2f spread=%.2f", n, time.Since(start).Round(time.Millisecond), res.GoldenCount, res.Unregistered, res.Threshold, res.ScoreMedian, res.ScoreSpread)
	tp, fp, fn, loc := 0, 0, 0, 0
	for i, im := range res.Images {
		box, planted := truth[i]
		switch {
		case planted && im.Anomaly:
			tp++
			b := [4]float64{float64(box.Min.X), float64(box.Min.Y), float64(box.Max.X), float64(box.Max.Y)}
			if regionsHitTruth(res, i, imgs[i].Bounds().Size(), [][4]float64{b}) {
				loc++
			}
		case planted:
			fn++
		case im.Anomaly:
			fp++
			if out := os.Getenv("BATCH_DUMP_FP"); out != "" && fp == 1 {
				dumpOverlay(t, res, i, out)
			}
		}
		if planted || im.Anomaly || im.Z > 3 {
			t.Logf("img %2d planted=%v score=%6.2f z=%7.1f anomaly=%v golden=%v reg=%s regions=%d", i, planted, im.Score, im.Z, im.Anomaly, im.Golden, im.Reg.Mode, len(im.Regions))
		}
	}
	t.Logf("SYNTHETIC: TP=%d FP=%d FN=%d localized=%d/%d", tp, fp, fn, loc, len(truth))
}

// jitterView renders src from a slightly different camera pose with exposure,
// white-balance and noise changes (no content changes).
func jitterView(src *image.NRGBA, rng *rand.Rand) *image.NRGBA {
	W, H := src.Bounds().Dx(), src.Bounds().Dy()
	cx, cy := W/10, H/10
	w, h := W-2*cx, H-2*cy
	u := func(a float64) float64 { return (rng.Float64()*2 - 1) * a }
	th, s := u(1.5)*math.Pi/180, 1+u(0.03)
	tx, ty := u(0.02)*float64(w), u(0.02)*float64(h)
	mx, my := float64(w)/2, float64(h)/2
	c, sn := s*math.Cos(th), s*math.Sin(th)
	a := mat3{c, -sn, mx - c*mx + sn*my + tx, sn, c, my - sn*mx - c*my + ty, 0, 0, 1}
	p := mat3{1, 0, 0, 0, 1, 0, u(0.015) / float64(w), u(0.015) / float64(h), 1}
	hsrc := mat3Mul(mat3Mul(p, a), mat3{1, 0, -float64(cx), 0, 1, -float64(cy), 0, 0, 1})
	out, _ := WarpHomography(src, [9]float64(hsrc), w, h)
	gain := [3]float64{0.85 + rng.Float64()*0.3, 0.85 + rng.Float64()*0.3, 0.85 + rng.Float64()*0.3}
	off := u(12)
	for i := 0; i < len(out.Pix); i += 4 {
		for ch := 0; ch < 3; ch++ {
			out.Pix[i+ch] = clampU8(float64(out.Pix[i+ch])*gain[ch] + off + rng.NormFloat64()*3)
		}
	}
	return out
}

// plantWire draws a thin (≈2–3 px) dark, gently curving cable across part of
// the frame and returns its bounding box.
func plantWire(img *image.NRGBA, rng *rand.Rand) image.Rectangle {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	x0, y0 := float64(w/6+rng.Intn(w/3)), float64(h/6+rng.Intn(h/3))
	ang := rng.Float64() * 2 * math.Pi
	length := float64(w) * (0.2 + 0.15*rng.Float64())
	curve := (rng.Float64() - 0.5) * 0.01
	box := image.Rect(int(x0), int(y0), int(x0)+1, int(y0)+1)
	x, y := x0, y0
	for t := 0.0; t < length; t += 0.5 {
		ang += curve
		x, y = x+0.5*math.Cos(ang), y+0.5*math.Sin(ang)
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				px, py := int(x)+dx, int(y)+dy
				if image.Pt(px, py).In(img.Bounds()) {
					o := py*img.Stride + px*4
					img.Pix[o], img.Pix[o+1], img.Pix[o+2] = 25, 25, 28
					box = box.Union(image.Rect(px, py, px+1, py+1))
				}
			}
		}
	}
	return box
}

// dumpOverlay writes image i's aligned stats-frame luma with the heat overlay.
func dumpOverlay(t *testing.T, res *BatchResult, i int, path string) {
	heat := res.HeatImage(i)
	base := image.NewNRGBA(heat.Bounds())
	for k := 0; k < res.W*res.H; k++ {
		L := float64(res.Images[i].lab[0][k])
		o := k * 4
		base.Pix[o], base.Pix[o+1], base.Pix[o+2], base.Pix[o+3] = uint8(L), uint8(L), uint8(L), 255
		a := float64(heat.Pix[o+3]) / 255
		for c := 0; c < 3; c++ {
			base.Pix[o+c] = uint8(float64(base.Pix[o+c])*(1-a) + float64(heat.Pix[o+c])*a)
		}
	}
	f, _ := os.Create(path)
	defer f.Close()
	png.Encode(f, base)
	t.Logf("dumped FP image %d (score %.1f) to %s", i, res.Images[i].Score, path)
}
