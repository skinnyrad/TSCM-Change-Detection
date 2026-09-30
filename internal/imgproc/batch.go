package imgproc

import (
	"image"
	"math"
	"runtime"
	"sort"
	"sync"
)

// Batch anomaly detection: many photos of the same scene, find the odd ones out
// and where they differ. This ports the ideas of the x-ray reference pipeline
// (robust golden set, per-pixel robust statistics, registration with a
// plausibility gate, border suppression, ranked components) to pure Go:
//
//  1. Pick the medoid image as the anchor frame.
//  2. Register every image to the anchor and match its exposure.
//  3. Iteratively build a "golden" set: per-pixel median + MAD over the golden
//     images, score every image by how far its worst area strays from that,
//     drop images whose score is a robust outlier, repeat.
//  4. Flag images whose score is far above the golden scores, and localize
//     their anomalies as ranked regions of the per-pixel z-map.

// BatchOptions tunes batch analysis. Zero values take defaults.
type BatchOptions struct {
	WorkDim    int     // longest edge images are registered at (default 512)
	StatsDim   int     // longest edge of the per-pixel statistics (default 384)
	ImageZ     float64 // robust z above which an image is anomalous (default 6)
	PixelZ     float64 // per-pixel z above which a pixel is anomalous (default 5)
	GoldenZ    float64 // robust z above which an image is dropped from the golden set (default 3.5)
	Border     float64 // fraction of each edge ignored (default 0.03)
	MaxRegions int     // regions kept per image (default 6)
	// MinScoreFactor: an image's peak z must exceed PixelZ by this factor to be
	// anomalous, so blobs barely over the pixel limit don't flag a shot (default 1.2).
	MinScoreFactor float64
}

func (o *BatchOptions) defaults(n int) {
	if o.WorkDim == 0 {
		o.WorkDim = 512
	}
	if o.StatsDim == 0 {
		// Finer statistics find smaller objects; step down for big batches to
		// bound memory (≈ 3 bytes × StatsDim² × 0.75 per image).
		switch {
		case n <= 300:
			o.StatsDim = 512
		case n <= 700:
			o.StatsDim = 384
		default:
			o.StatsDim = 320
		}
	}
	if o.ImageZ == 0 {
		o.ImageZ = 6
	}
	if o.PixelZ == 0 {
		o.PixelZ = 5
	}
	if o.GoldenZ == 0 {
		o.GoldenZ = 3.5
	}
	if o.Border == 0 {
		o.Border = 0.03
	}
	if o.MaxRegions == 0 {
		o.MaxRegions = 6
	}
	if o.MinScoreFactor == 0 {
		o.MinScoreFactor = 1.2
	}
}

// BatchImage is one image's analysis result.
type BatchImage struct {
	Reg        Registration
	Transform  [9]float64 // original work-image coords → anchor work frame
	Registered bool       // registration succeeded (or the image was already in frame)
	Score      float64    // peak of the blob-filtered per-pixel z-map
	Z          float64    // robust z of Score against the golden set
	Golden     bool       // member of the final golden set
	Anomaly    bool       // Score ≥ Threshold and at least one region
	Regions    []Region   // ranked anomalous regions, in stats-frame pixels

	lab   [3][]uint8 // aligned, exposure-matched CIELAB (stats frame)
	valid []bool     // stats-frame pixels with real data
}

// BatchResult is the output of AnalyzeBatch.
type BatchResult struct {
	Opts         BatchOptions
	Anchor       int
	FrameW       int // anchor work-frame size (registration frame)
	FrameH       int
	W, H         int // stats-frame size
	Images       []BatchImage
	Reference    *image.NRGBA // per-pixel median of the golden set (stats frame)
	GoldenCount  int
	ScoreMedian  float64
	ScoreSpread  float64 // 1.4826·MAD of golden scores
	Threshold    float64 // score above which an image is anomalous: max(median+ImageZ·spread, PixelZ·MinScoreFactor)
	Unregistered int

	med    [3][]uint8
	spread [3][]float32
}

// AnalyzeBatch runs batch anomaly detection on in-memory images.
func AnalyzeBatch(imgs []*image.NRGBA, opts BatchOptions, progress func(done, total int)) *BatchResult {
	return AnalyzeBatchFunc(len(imgs), func(i int) (*image.NRGBA, error) { return imgs[i], nil }, opts, progress)
}

// AnalyzeBatchFunc runs batch anomaly detection, loading image i on demand via
// load so hundreds of photos never have to be decoded at once. Images are
// reduced to WorkDim internally. progress (may be nil) is called as images
// are registered. Images that fail to load are reported as unregistered.
func AnalyzeBatchFunc(n int, load func(i int) (*image.NRGBA, error), opts BatchOptions, progress func(done, total int)) *BatchResult {
	opts.defaults(n)
	res := &BatchResult{Opts: opts, Images: make([]BatchImage, n)}
	if n == 0 {
		return res
	}
	loadWork := func(i int) *image.NRGBA {
		im, err := load(i)
		if err != nil || im == nil {
			return nil
		}
		return DownsampleNRGBA(im, opts.WorkDim)
	}

	// Pass 1: thumbnails for the medoid.
	vecs := make([][]float64, n)
	parallelFor(n, func(i int) {
		if im := loadWork(i); im != nil {
			vecs[i] = thumbVector(im)
		}
	})
	res.Anchor = medoid(vecs)
	anchor := loadWork(res.Anchor)
	res.FrameW, res.FrameH = anchor.Bounds().Dx(), anchor.Bounds().Dy()
	stats := DownsampleNRGBA(anchor, opts.StatsDim)
	res.W, res.H = stats.Bounds().Dx(), stats.Bounds().Dy()

	// Pass 2: register everything to the anchor.
	var mu sync.Mutex
	done := 0
	parallelFor(n, func(i int) {
		if i == res.Anchor {
			res.Images[i] = registerForBatch(anchor, anchor, true, res.W, res.H)
		} else if im := loadWork(i); im != nil {
			res.Images[i] = registerForBatch(im, anchor, false, res.W, res.H)
		} else {
			res.Images[i] = BatchImage{Reg: Registration{Mode: "none", Message: "could not be loaded"}, valid: make([]bool, res.W*res.H)}
			for c := range res.Images[i].lab {
				res.Images[i].lab[c] = make([]uint8, res.W*res.H)
			}
		}
		mu.Lock()
		done++
		if progress != nil {
			progress(done, n)
		}
		mu.Unlock()
	})

	for i := range res.Images {
		if !res.Images[i].Registered {
			res.Unregistered++
		}
	}
	res.iterateGolden()
	for i := range res.Images {
		res.localize(i)
		// The score is itself a per-pixel z, so it has an absolute meaning:
		// an anomaly must stand out from the golden images' scores, exceed
		// PixelZ, and be localizable (at least one region to point at).
		im := &res.Images[i]
		im.Anomaly = im.Score >= res.Threshold && len(im.Regions) > 0
	}
	return res
}

func parallelFor(n int, fn func(i int)) {
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < max(1, runtime.NumCPU()-1); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				fn(i)
			}
		}()
	}
	for i := 0; i < n; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
}

// thumbVector is a small blurred, contrast-normalised luma thumbnail used to
// compare images globally (tolerant of small shifts and exposure changes).
func thumbVector(im *image.NRGBA) []float64 {
	const tw, th = 24, 18
	g := ResizeNRGBA(GaussianBlurNRGBA(DownsampleNRGBA(im, 96), 1.5), tw, th)
	v := make([]float64, tw*th)
	var mean float64
	for k := range v {
		p := g.Pix[(k/tw)*g.Stride+(k%tw)*4:]
		v[k] = lumaOf(p[:3])
		mean += v[k]
	}
	mean /= float64(len(v))
	var ss float64
	for k := range v {
		v[k] -= mean
		ss += v[k] * v[k]
	}
	norm := math.Sqrt(ss) + 1e-9
	for k := range v {
		v[k] /= norm
	}
	return v
}

// medoid returns the index of the image most similar to all others (smallest
// median thumbnail distance). Anchoring on a typical image keeps registration
// of the rest easy and keeps odd images out of the reference frame. Images
// that failed to load (nil vectors) are never chosen.
func medoid(vecs [][]float64) int {
	best, bestD := 0, math.Inf(1)
	for i := range vecs {
		if vecs[i] == nil {
			continue
		}
		d := make([]float64, 0, len(vecs))
		for j := range vecs {
			if i == j || vecs[j] == nil {
				continue
			}
			var dot float64
			for k := range vecs[i] {
				dot += vecs[i][k] * vecs[j][k]
			}
			d = append(d, 1-dot)
		}
		sort.Float64s(d)
		m := 0.0
		if len(d) > 0 {
			m = d[len(d)/2]
		}
		if m < bestD {
			best, bestD = i, m
		}
	}
	return best
}

func registerForBatch(img, anchor *image.NRGBA, isAnchor bool, sw, sh int) BatchImage {
	var bi BatchImage
	var aligned *image.NRGBA
	var valid *image.Gray
	if isAnchor {
		aligned = anchor
		bi.Reg = Registration{Mode: "none", Message: "reference frame"}
		bi.Transform = [9]float64{1, 0, 0, 0, 1, 0, 0, 0, 1}
		bi.Registered = true
	} else {
		a := RegisterPair(img, anchor, true)
		aligned, valid, bi.Reg, bi.Transform = a.Before, a.Valid, a.Reg, a.Transform
		// Registered = a transform was found, or the frames already agree.
		bi.Registered = a.Reg.Applied || a.Reg.Message == "images already aligned"
		aligned = MatchIntensity(aligned, anchor, valid)
	}
	s := ResizeNRGBA(aligned, sw, sh)
	lab := toLab(s)
	for c, plane := range [3][]float32{lab.L, lab.A, lab.B} {
		bi.lab[c] = make([]uint8, len(plane))
		for k, v := range plane {
			if c == 0 {
				bi.lab[c][k] = clampU8(float64(v) * 2.55)
			} else {
				bi.lab[c][k] = clampU8(float64(v) + 128)
			}
		}
	}
	bi.valid = make([]bool, sw*sh)
	var vm *image.Gray
	if valid != nil {
		// Erode so interpolated warp edges don't count.
		vm = fastBinaryErode(ResizeMask(valid, sw, sh), 5)
	}
	for k := range bi.valid {
		bi.valid[k] = vm == nil || vm.Pix[(k/sw)*vm.Stride+k%sw] != 0
	}
	return bi
}

// iterateGolden alternates between per-pixel statistics over the golden set
// and image scores against those statistics, trimming outliers each round.
func (r *BatchResult) iterateGolden() {
	for i := range r.Images {
		r.Images[i].Golden = r.Images[i].Registered && coverage(r.Images[i].valid) > 0.6
	}
	for round := 0; round < 6; round++ {
		r.pixelStats()
		var scores []float64
		for i := range r.Images {
			r.Images[i].Score = r.imageScore(i)
			if r.Images[i].Golden {
				scores = append(scores, r.Images[i].Score)
			}
		}
		med, spread := robustCentre(scores)
		r.ScoreMedian, r.ScoreSpread = med, spread
		changed := false
		for i := range r.Images {
			im := &r.Images[i]
			im.Z = (im.Score - med) / spread
			keep := im.Registered && coverage(im.valid) > 0.6 && im.Z < r.Opts.GoldenZ
			if keep != im.Golden {
				im.Golden, changed = keep, true
			}
		}
		if !changed {
			break
		}
	}
	r.GoldenCount = 0
	r.Threshold = math.Max(r.ScoreMedian+r.Opts.ImageZ*r.ScoreSpread, r.Opts.PixelZ*r.Opts.MinScoreFactor)
	for i := range r.Images {
		im := &r.Images[i]
		if im.Golden {
			r.GoldenCount++
		}
	}
}

func robustCentre(v []float64) (med, spread float64) {
	if len(v) == 0 {
		return 0, 1
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	med = s[len(s)/2]
	dev := make([]float64, len(s))
	for i, x := range s {
		dev[i] = math.Abs(x - med)
	}
	sort.Float64s(dev)
	// Floor the spread (scores are in per-pixel z units): with a very uniform
	// golden set the MAD can be ~0, which would turn noise into huge z-scores.
	spread = math.Max(1.4826*dev[len(dev)/2], 0.25)
	return med, spread
}

func coverage(v []bool) float64 {
	n := 0
	for _, b := range v {
		if b {
			n++
		}
	}
	return float64(n) / float64(max(len(v), 1))
}

// pixelStats computes the per-pixel median and scaled MAD of each Lab channel
// over the golden images (histogram-based, O(pixels × (images + 256))).
func (r *BatchResult) pixelStats() {
	np := r.W * r.H
	var golden []int
	for i := range r.Images {
		if r.Images[i].Golden {
			golden = append(golden, i)
		}
	}
	if len(golden) == 0 {
		for i := range r.Images {
			golden = append(golden, i)
		}
	}
	for c := 0; c < 3; c++ {
		r.med[c] = make([]uint8, np)
		r.spread[c] = make([]float32, np)
	}
	var wg sync.WaitGroup
	chunk := (np + runtime.NumCPU() - 1) / runtime.NumCPU()
	for start := 0; start < np; start += chunk {
		wg.Add(1)
		go func(lo, hi int) {
			defer wg.Done()
			var hist, dev [256]int
			for k := lo; k < hi; k++ {
				for c := 0; c < 3; c++ {
					hist = [256]int{}
					cnt := 0
					for _, i := range golden {
						if r.Images[i].valid[k] {
							hist[r.Images[i].lab[c][k]]++
							cnt++
						}
					}
					if cnt == 0 {
						r.med[c][k], r.spread[c][k] = 0, 255
						continue
					}
					m := histQuantile(&hist, cnt)
					dev = [256]int{}
					for v, h := range hist {
						if h > 0 {
							dev[absInt(v-m)] += h
						}
					}
					r.med[c][k] = uint8(m)
					r.spread[c][k] = float32(1.4826 * float64(histQuantile(&dev, cnt)))
				}
			}
		}(start, min(start+chunk, np))
	}
	wg.Wait()
	ref := image.NewNRGBA(image.Rect(0, 0, r.W, r.H))
	for k := 0; k < np; k++ {
		rgb := labToRGB(float64(r.med[0][k])/2.55, float64(r.med[1][k])-128, float64(r.med[2][k])-128)
		copy(ref.Pix[(k/r.W)*ref.Stride+(k%r.W)*4:], []uint8{rgb[0], rgb[1], rgb[2], 255})
	}
	r.Reference = ref
}

func histQuantile(h *[256]int, n int) int {
	acc := 0
	for v, c := range h {
		acc += c
		if 2*acc >= n {
			return v
		}
	}
	return 255
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// rawZ is image i's per-pixel anomaly z (max over Lab channels of the
// deviation from the golden median in units of the golden spread).
func (r *BatchResult) rawZ(i int) []float32 {
	im := &r.Images[i]
	z := make([]float32, r.W*r.H)
	// Spread floors (in Lab-uint8 units) keep perfectly static pixels — where
	// every golden image agrees exactly — from producing infinite z on noise.
	floors := [3]float32{3, 2, 2}
	for k := range z {
		// Skip blown-out highlights: clipped pixels (lamps, glare, windows)
		// can't be exposure-matched and would flag every brighter shot.
		if !im.valid[k] || im.lab[0][k] >= 245 || r.med[0][k] >= 245 {
			continue
		}
		var best float32
		for c := 0; c < 3; c++ {
			d := float32(absInt(int(im.lab[c][k]) - int(r.med[c][k])))
			s := r.spread[c][k]
			if s < floors[c] {
				s = floors[c]
			}
			if v := d / s; v > best {
				best = v
			}
		}
		z[k] = best
	}
	return z
}

// ZMap returns image i's anomaly z-map (stats frame) with invalid pixels and
// the border zeroed. It combines two detectors:
//
//   - Blobs: a grey-level opening (min then max filter) keeps compact
//     anomalies (objects) and removes thin residue along edges that imperfect
//     registration leaves in every photo taken from a slightly different spot.
//   - Thin structures: wires, cables and leads are exactly what the opening
//     would erase, so thin components of the raw map are kept separately when
//     they are long and mostly lie where the image has an edge the golden
//     reference does not. Registration residue sits on edges the reference
//     already has; a newly run wire is a new edge on a smooth background.
func (r *BatchResult) ZMap(i int) []float32 {
	im := &r.Images[i]
	w, h := r.W, r.H
	raw := boxBlur(r.rawZ(i), w, h, 1)
	z := boxBlur(maxFilter(minFilter(raw, w, h, 2), w, h, 2), w, h, 1)

	// Thin-structure pass.
	thr := float32(r.Opts.PixelZ) * 1.2
	mask := image.NewGray(image.Rect(0, 0, w, h))
	for k, v := range raw {
		if v >= thr && z[k] < thr { // only what the blob pass dropped
			mask.Pix[k] = 255
		}
	}
	labels, comps := components(mask)
	if len(comps) > 0 {
		// Compare with the strongest reference edge nearby: the golden median
		// is slightly soft wherever the shots disagree, so a per-pixel
		// comparison would mistake real scene edges for new ones.
		gradI, gradR := gradMag(im.lab[0], w, h), maxFilter(gradMag(r.med[0], w, h), w, h, 2)
		minLen := 0.04 * math.Hypot(float64(w), float64(h))
		newEdge := make([]int, len(comps))
		for k, l := range labels {
			if l > 0 && gradI[k] > 2.5*gradR[k]+8 {
				newEdge[l-1]++
			}
		}
		keep := make([]bool, len(comps))
		for c, reg := range comps {
			long := math.Hypot(float64(reg.Box.Dx()), float64(reg.Box.Dy()))
			keep[c] = long >= minLen && float64(newEdge[c]) >= 0.4*float64(reg.Area)
		}
		for k, l := range labels {
			if l > 0 && keep[l-1] && raw[k] > z[k] {
				z[k] = raw[k]
			}
		}
	}

	bx, by := int(float64(w)*r.Opts.Border), int(float64(h)*r.Opts.Border)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x < bx || y < by || x >= w-bx || y >= h-by || !im.valid[y*w+x] {
				z[y*w+x] = 0
			}
		}
	}
	return z
}

// gradMag is the central-difference gradient magnitude of an 8-bit plane,
// lightly smoothed.
func gradMag(p []uint8, w, h int) []float32 {
	g := make([]float32, w*h)
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			gx := float32(p[y*w+x+1]) - float32(p[y*w+x-1])
			gy := float32(p[(y+1)*w+x]) - float32(p[(y-1)*w+x])
			g[y*w+x] = float32(math.Sqrt(float64(gx*gx + gy*gy)))
		}
	}
	return boxBlur(g, w, h, 1)
}

// imageScore is the peak of the image's blob-filtered z-map. A percentile
// would miss small objects (a 1/20-of-frame object covers ~0.25% of the
// pixels); the grey-level opening in ZMap already removes the thin, isolated
// responses that make a raw maximum unreliable.
func (r *BatchResult) imageScore(i int) float64 {
	var peak float32
	for _, v := range r.ZMap(i) {
		if v > peak {
			peak = v
		}
	}
	return float64(peak)
}

// localize finds ranked anomalous regions in image i.
func (r *BatchResult) localize(i int) {
	z := r.ZMap(i)
	w, h := r.W, r.H
	mask := image.NewGray(image.Rect(0, 0, w, h))
	heat := image.NewGray(image.Rect(0, 0, w, h))
	thr := float32(r.Opts.PixelZ)
	for k, v := range z {
		if v >= thr {
			mask.Pix[k] = 255
		}
		heat.Pix[k] = uint8(math.Min(255, float64(v)*10))
	}
	// No opening here: ZMap already removed noise, and opening would delete
	// the thin structures it deliberately kept.
	mask = MorphologicalClose(mask, 5)
	mask = FilterByMinRegionSize(mask, max(12, w*h/4000))
	regions := RankRegions(heat, mask)
	kept := regions[:0]
	for _, reg := range regions {
		if reg.Score >= 0.25 && len(kept) < r.Opts.MaxRegions {
			kept = append(kept, reg)
		}
	}
	r.Images[i].Regions = kept
}

// HeatImage renders image i's z-map as a JET heat map with alpha growing with
// the anomaly strength, suitable for overlaying on the aligned image.
func (r *BatchResult) HeatImage(i int) *image.NRGBA {
	z := r.ZMap(i)
	out := image.NewNRGBA(image.Rect(0, 0, r.W, r.H))
	// Start colouring just below the detection threshold so the overlay shows
	// significant deviation, not the low-level speckle every image has.
	lo, hi := r.Opts.PixelZ*0.8, r.Opts.PixelZ*3
	for k, v := range z {
		t := (float64(v) - lo) / (hi - lo)
		if t <= 0 {
			continue
		}
		t = math.Min(1, t)
		c := jetLUT[int(t*255)]
		o := (k/r.W)*out.Stride + (k%r.W)*4
		out.Pix[o], out.Pix[o+1], out.Pix[o+2] = c[0], c[1], c[2]
		out.Pix[o+3] = uint8(60 + 170*t)
	}
	return out
}

func boxBlur(v []float32, w, h, r int) []float32 {
	tmp := make([]float32, len(v))
	out := make([]float32, len(v))
	for y := 0; y < h; y++ {
		var s float32
		n := 0
		for x := -r; x < w; x++ {
			if x+r < w {
				s += v[y*w+x+r]
				n++
			}
			if x-r-1 >= 0 {
				s -= v[y*w+x-r-1]
				n--
			}
			if x >= 0 {
				tmp[y*w+x] = s / float32(n)
			}
		}
	}
	for x := 0; x < w; x++ {
		var s float32
		n := 0
		for y := -r; y < h; y++ {
			if y+r < h {
				s += tmp[(y+r)*w+x]
				n++
			}
			if y-r-1 >= 0 {
				s -= tmp[(y-r-1)*w+x]
				n--
			}
			if y >= 0 {
				out[y*w+x] = s / float32(n)
			}
		}
	}
	return out
}

// labToRGB converts CIELAB (D65) back to 8-bit sRGB.
func labToRGB(L, a, b float64) [3]uint8 {
	fy := (L + 16) / 116
	fx, fz := fy+a/500, fy-b/200
	inv := func(t float64) float64 {
		if t*t*t > 216.0/24389.0 {
			return t * t * t
		}
		return (116*t - 16) * 27.0 / 24389.0
	}
	x, y, z := 0.95047*inv(fx), inv(fy), 1.08883*inv(fz)
	lin := [3]float64{
		3.2404542*x - 1.5371385*y - 0.4985314*z,
		-0.9692660*x + 1.8760108*y + 0.0415560*z,
		0.0556434*x - 0.2040259*y + 1.0572252*z,
	}
	var out [3]uint8
	for i, c := range lin {
		c = math.Max(0, math.Min(1, c))
		if c <= 0.0031308 {
			c *= 12.92
		} else {
			c = 1.055*math.Pow(c, 1/2.4) - 0.055
		}
		out[i] = uint8(c*255 + 0.5)
	}
	return out
}

func minFilter(v []float32, w, h, r int) []float32 { return rankFilter(v, w, h, r, true) }
func maxFilter(v []float32, w, h, r int) []float32 { return rankFilter(v, w, h, r, false) }

// rankFilter is a separable (2r+1)² min or max filter.
func rankFilter(v []float32, w, h, r int, isMin bool) []float32 {
	better := func(a, b float32) bool {
		if isMin {
			return a < b
		}
		return a > b
	}
	tmp := make([]float32, len(v))
	out := make([]float32, len(v))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			best := v[y*w+x]
			for d := max(0, x-r); d <= min(w-1, x+r); d++ {
				if better(v[y*w+d], best) {
					best = v[y*w+d]
				}
			}
			tmp[y*w+x] = best
		}
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			best := tmp[y*w+x]
			for d := max(0, y-r); d <= min(h-1, y+r); d++ {
				if better(tmp[d*w+x], best) {
					best = tmp[d*w+x]
				}
			}
			out[y*w+x] = best
		}
	}
	return out
}
