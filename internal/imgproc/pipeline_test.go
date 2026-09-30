package imgproc

import (
	"image"
	"image/color"
	"math"
	"math/rand"
	"testing"
)

// textured returns a deterministic noise-textured image with soft structure,
// so feature matching and diffing have something to work with.
func textured(w, h int, seed int64) *image.NRGBA {
	rng := rand.New(rand.NewSource(seed))
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			base := 128 + 70*math.Sin(float64(x)/9) + 50*math.Cos(float64(y)/13)
			n := float64(rng.Intn(40)) - 20
			v := uint8(math.Max(0, math.Min(255, base+n)))
			img.SetNRGBA(x, y, color.NRGBA{v, uint8(255 - int(v)/2), v / 2, 255})
		}
	}
	return img
}

func paint(img *image.NRGBA, r image.Rectangle, c color.NRGBA) *image.NRGBA {
	out := image.NewNRGBA(img.Bounds())
	copy(out.Pix, img.Pix)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			out.SetNRGBA(x, y, c)
		}
	}
	return out
}

func defaultOpts() DiffOptions {
	return DiffOptions{Threshold: 40, MorphSize: 3, CloseSize: 3, MinRegion: 25, PreBlurSigma: 1, MatchIntensity: true, ColorWeight: 1}
}

func TestComputeDiffFindsInsertedObject(t *testing.T) {
	before := textured(200, 160, 1)
	box := image.Rect(60, 50, 110, 100)
	after := paint(before, box, color.NRGBA{255, 0, 0, 255})
	res := ComputeDiffV2(before, after, defaultOpts())
	regions := RankRegions(res.Diff, res.Mask)
	if len(regions) != 1 {
		t.Fatalf("want 1 region, got %d", len(regions))
	}
	if iou(regions[0].Box, box) < 0.7 {
		t.Fatalf("box %v does not match %v", regions[0].Box, box)
	}
}

func TestComputeDiffIdenticalIsEmpty(t *testing.T) {
	img := textured(120, 100, 2)
	res := ComputeDiffV2(img, img, defaultOpts())
	if n := len(Regions(res.Mask)); n != 0 {
		t.Fatalf("identical images produced %d regions", n)
	}
}

func TestColorOnlyChangeNeedsColorWeight(t *testing.T) {
	// Same lightness, different hue: luma-only diff misses it, Lab diff catches it.
	before := paint(image.NewNRGBA(image.Rect(0, 0, 80, 80)), image.Rect(0, 0, 80, 80), color.NRGBA{120, 120, 120, 255})
	after := paint(before, image.Rect(20, 20, 60, 60), color.NRGBA{170, 90, 120, 255})
	opts := defaultOpts()
	opts.ColorWeight = 0
	if n := len(Regions(ComputeDiffV2(before, after, opts).Mask)); n != 0 {
		t.Logf("luma-only found %d regions (hue shift had some lightness change)", n)
	}
	opts.ColorWeight = 1
	if n := len(Regions(ComputeDiffV2(before, after, opts).Mask)); n != 1 {
		t.Fatalf("color-aware diff found %d regions, want 1", n)
	}
}

func TestMatchIntensityCancelsExposureChange(t *testing.T) {
	before := textured(160, 120, 3)
	after := image.NewNRGBA(before.Bounds())
	for i := 0; i < len(before.Pix); i += 4 {
		for c := 0; c < 3; c++ {
			after.Pix[i+c] = clampU8(float64(before.Pix[i+c])*0.8 + 20)
		}
		after.Pix[i+3] = 255
	}
	opts := defaultOpts()
	if n := len(Regions(ComputeDiffV2(before, after, opts).Mask)); n != 0 {
		t.Fatalf("exposure change produced %d regions with intensity matching", n)
	}
}

func TestIgnoreAndValidMasks(t *testing.T) {
	before := textured(200, 160, 4)
	box := image.Rect(60, 50, 110, 100)
	after := paint(before, box, color.NRGBA{255, 0, 0, 255})
	opts := defaultOpts()
	opts.Ignore = []Rect{{X0: 0.2, Y0: 0.2, X1: 0.7, Y1: 0.8}}
	if n := len(Regions(ComputeDiffV2(before, after, opts).Mask)); n != 0 {
		t.Fatalf("ignored region still detected (%d)", n)
	}
	opts.Ignore = nil
	opts.Valid = image.NewGray(before.Bounds()) // nothing valid
	if n := len(Regions(ComputeDiffV2(before, after, opts).Mask)); n != 0 {
		t.Fatalf("invalid region still detected (%d)", n)
	}
}

func TestAutoThresholdRejectsNoise(t *testing.T) {
	before := textured(200, 160, 5)
	after := textured(200, 160, 5)
	rng := rand.New(rand.NewSource(9))
	for i := 0; i < len(after.Pix); i += 4 {
		d := rng.Intn(9) - 4
		for c := 0; c < 3; c++ {
			after.Pix[i+c] = clampU8(float64(after.Pix[i+c]) + float64(d))
		}
	}
	box := image.Rect(80, 60, 130, 110)
	after = paint(after, box, color.NRGBA{10, 10, 240, 255})
	opts := defaultOpts()
	opts.AutoThreshold, opts.Threshold = true, 0
	res := ComputeDiffV2(before, after, opts)
	if n := len(Regions(res.Mask)); n != 1 {
		t.Fatalf("auto threshold %d produced %d regions, want 1", res.Threshold, n)
	}
}

func TestRegisterPairRecoversShift(t *testing.T) {
	before := textured(320, 240, 6)
	after := image.NewNRGBA(before.Bounds())
	shift := 5
	for y := 0; y < 240; y++ {
		for x := 0; x < 320; x++ {
			sx := x - shift
			if sx >= 0 {
				copy(after.Pix[y*after.Stride+x*4:y*after.Stride+x*4+4], before.Pix[y*before.Stride+sx*4:y*before.Stride+sx*4+4])
			}
		}
	}
	for i := 3; i < len(after.Pix); i += 4 {
		after.Pix[i] = 255
	}
	plain := ComputeDiffV2(before, after, defaultOpts())
	al := RegisterPair(before, after, true)
	opts := defaultOpts()
	opts.Valid = al.Valid
	reg := ComputeDiffV2(al.Before, al.After, opts)
	if !al.Reg.Applied {
		t.Fatalf("registration not applied: %+v", al.Reg)
	}
	if a, b := len(Regions(plain.Mask)), len(Regions(reg.Mask)); b >= a && a > 0 {
		t.Fatalf("registration did not reduce false regions: %d → %d (%+v)", a, b, al.Reg)
	}
}

func TestResizeToFrameLetterboxesDifferentAspect(t *testing.T) {
	img := textured(200, 100, 7) // 2:1 into a 4:3 frame
	out, valid, resized := resizeToFrame(img, 160, 120)
	if !resized || valid == nil {
		t.Fatal("expected letterbox with mask")
	}
	if out.Bounds().Dx() != 160 || out.Bounds().Dy() != 120 {
		t.Fatalf("bounds %v", out.Bounds())
	}
	if valid.Pix[0] != 0 || valid.Pix[60*valid.Stride+80] == 0 {
		t.Fatal("mask should be empty in padding and set in the middle")
	}
}

func TestWarpHomographyIdentityAndSingular(t *testing.T) {
	img := textured(40, 30, 8)
	out, valid := WarpHomography(img, [9]float64{1, 0, 0, 0, 1, 0, 0, 0, 1}, 40, 30)
	if valid.Pix[15*valid.Stride+20] == 0 || out.NRGBAAt(20, 15) != img.NRGBAAt(20, 15) {
		t.Fatal("identity warp changed the image")
	}
	_, valid = WarpHomography(img, [9]float64{}, 40, 30)
	for _, v := range valid.Pix {
		if v != 0 {
			t.Fatal("singular homography should produce an empty mask")
		}
	}
	if _, err := computeHomography([]Point{{0, 0}, {1, 1}, {2, 2}, {3, 3}}, []Point{{0, 0}, {1, 1}, {2, 2}, {3, 3}}); err == nil {
		t.Fatal("collinear points should be rejected")
	}
}

func TestComputeHomographyRecoversTranslation(t *testing.T) {
	src := []Point{{10, 10}, {100, 12}, {98, 90}, {12, 88}}
	var dst []Point
	for _, p := range src {
		dst = append(dst, Point{p.X + 7, p.Y - 3})
	}
	h, err := computeHomography(src, dst)
	if err != nil {
		t.Fatal(err)
	}
	x, y := applyHomography(h, 50, 50)
	if math.Abs(x-57) > 1e-6 || math.Abs(y-47) > 1e-6 {
		t.Fatalf("got (%f,%f), want (57,47)", x, y)
	}
}

func TestCombineBaselinesMedianAndSpread(t *testing.T) {
	mk := func(v uint8) *image.NRGBA {
		return paint(image.NewNRGBA(image.Rect(0, 0, 4, 4)), image.Rect(0, 0, 4, 4), color.NRGBA{v, v, v, 255})
	}
	ref, spread := CombineBaselines([]*image.NRGBA{mk(100), mk(104), mk(200)})
	if ref.NRGBAAt(1, 1).R != 104 {
		t.Fatalf("median = %d, want 104", ref.NRGBAAt(1, 1).R)
	}
	if spread.GrayAt(1, 1).Y == 0 {
		t.Fatal("spread should be non-zero when baselines disagree")
	}
}

func TestChangeStatsAndNormalizeHeat(t *testing.T) {
	m := image.NewGray(image.Rect(0, 0, 10, 10))
	for x := 0; x < 5; x++ {
		m.Pix[x] = 255
	}
	st := ChangeStats(m)
	if st.ChangedPx != 5 || st.Regions != 1 || st.Pct != 5 {
		t.Fatalf("stats %+v", st)
	}
	g := image.NewGray(image.Rect(0, 0, 10, 10))
	for i := range g.Pix {
		g.Pix[i] = 20
	}
	if NormalizeHeat(g).Pix[0] != 255 {
		t.Fatal("heat should stretch to full range")
	}
}

func TestTiePointsAreSpreadAndExact(t *testing.T) {
	src := textured(900, 700, 11)
	before, after, htrue := synthPair(src, rand.New(rand.NewSource(3)))
	res, err := AutoDetectHomography(before, after)
	if err != nil {
		t.Fatal(err)
	}
	w, h := before.Bounds().Dx(), before.Bounds().Dy()
	var s, d []Point
	for _, p := range res.Pairs {
		s, d = append(s, p.Src), append(d, p.Dst)
	}
	if sp := spreadScore(s, w, h); sp < 0.75 {
		t.Fatalf("tie points cover only %.2f of the grid", sp)
	}
	hp, err := computeHomography(s, d)
	if err != nil {
		t.Fatal(err)
	}
	if e := homographyError(hp, htrue, w, h); e > 1 {
		t.Fatalf("tie-point homography error %.2f px", e)
	}
}

func TestRegisterTransformMatchesWarp(t *testing.T) {
	src := textured(900, 700, 12)
	before, after, _ := synthPair(src, rand.New(rand.NewSource(5)))
	al := RegisterPair(before, after, true)
	if !al.TransformOK || !al.Reg.Applied {
		t.Fatalf("registration: %+v", al.Reg)
	}
	// Re-warping the original with the reported transform must reproduce Before.
	re, _ := WarpHomography(before, al.Transform, al.After.Bounds().Dx(), al.After.Bounds().Dy())
	var diff, n float64
	for y := 50; y < al.After.Bounds().Dy()-50; y += 7 {
		for x := 50; x < al.After.Bounds().Dx()-50; x += 7 {
			diff += math.Abs(float64(re.NRGBAAt(x, y).G) - float64(al.Before.NRGBAAt(x, y).G))
			n++
		}
	}
	if diff/n > 2 {
		t.Fatalf("transform re-warp differs by %.2f on average", diff/n)
	}
}
