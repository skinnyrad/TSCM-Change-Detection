package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/skinnyrad/tscm-change-detection/internal/imgproc"
	"github.com/skinnyrad/tscm-change-detection/internal/state"
)

const (
	maxUploadBytes  = 256 << 20 // raw upload size cap
	maxUploadPixels = 120e6     // decoded size cap (width*height)
	maxRegionsShown = 50
)

type dimsJSON struct {
	W int `json:"w"`
	H int `json:"h"`
}

type singleUploadResponse struct {
	Dims         dimsJSON              `json:"dims"`
	Version      int64                 `json:"version"`
	Baselines    int                   `json:"baselines"`
	Registration *imgproc.Registration `json:"registration,omitempty"`
}

// regionJSON is a detected change region in fractions of the analysis image.
type regionJSON struct {
	Rank    int     `json:"rank"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	W       float64 `json:"w"`
	H       float64 `json:"h"`
	AreaPx  int     `json:"area_px"`
	AreaPct float64 `json:"area_pct"`
	Score   float64 `json:"score"`     // heat mass relative to the strongest region
	Heat    float64 `json:"mean_heat"` // mean change magnitude (0-255)
}

type analyzeResponse struct {
	Stats        imgproc.Stats        `json:"stats"`
	Regions      []regionJSON         `json:"regions"`
	Images       map[string]string    `json:"images"`
	BeforeDims   dimsJSON             `json:"before_dims"`
	AfterDims    dimsJSON             `json:"after_dims"`
	AnalysisDims dimsJSON             `json:"analysis_dims"`
	Resized      bool                 `json:"resized"`
	Threshold    int                  `json:"threshold"`
	Baselines    int                  `json:"baselines"`
	Registration imgproc.Registration `json:"registration"`
}

type autoWarpPairResponse struct {
	Src        [2]float64 `json:"src"`
	Dst        [2]float64 `json:"dst"`
	Confidence float64    `json:"confidence"`
}

type autoWarpResponse struct {
	Pairs       []autoWarpPairResponse `json:"pairs"`
	Confidence  float64                `json:"confidence"`
	MatchCount  int                    `json:"match_count"`
	InlierCount int                    `json:"inlier_count"`
}

// realign recomputes the aligned pair from the current raw inputs. The result
// is only stored if the inputs have not changed in the meantime.
func realign() (imgproc.Registration, bool) {
	snap, ok := state.Global.Snapshot()
	if !ok {
		return imgproc.Registration{}, false
	}
	a := imgproc.RegisterPairOpts(snap.Before, snap.After, snap.Settings.AutoRegister, snap.Settings.LocalRefine)
	state.Global.SetAligned(snap.Generation, a)
	return a.Reg, true
}

func uploadHandler(store func(*image.NRGBA, state.Dims) int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		img, dims, ok := decodeFormImage(c, "image")
		if !ok {
			return
		}
		version := store(imgproc.ToNRGBA(img), dims)
		resp := singleUploadResponse{
			Dims: dimsJSON{W: dims.W, H: dims.H}, Version: version, Baselines: state.Global.Baselines(),
		}
		if reg, ok := realign(); ok {
			resp.Registration = &reg
		}
		c.JSON(http.StatusOK, resp)
	}
}

// HandleUploadBefore decodes and stores the before image, then registers the
// pair if the after image is already present.
// POST /api/upload/before — multipart/form-data: image
func HandleUploadBefore(c *gin.Context) {
	uploadHandler(func(i *image.NRGBA, d state.Dims) int64 {
		state.Global.SetBefore(i, d)
		return state.Global.BeforeVersion()
	})(c)
}

// HandleUploadAfter decodes and stores the after image, then registers the
// pair if the before image is already present.
// POST /api/upload/after — multipart/form-data: image
func HandleUploadAfter(c *gin.Context) {
	uploadHandler(func(i *image.NRGBA, d state.Dims) int64 {
		state.Global.SetAfter(i, d)
		return state.Global.AfterVersion()
	})(c)
}

// HandleUploadBaseline adds an extra "before" sweep for multi-baseline mode.
// POST /api/upload/baseline — multipart/form-data: image
func HandleUploadBaseline(c *gin.Context) {
	if state.Global.RawBefore() == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "upload the primary before image first"})
		return
	}
	img, dims, ok := decodeFormImage(c, "image")
	if !ok {
		return
	}
	n := state.Global.AddBaseline(imgproc.ToNRGBA(img))
	resp := singleUploadResponse{Dims: dimsJSON{W: dims.W, H: dims.H}, Baselines: n}
	if reg, ok := realign(); ok {
		resp.Registration = &reg
	}
	c.JSON(http.StatusOK, resp)
}

// HandleClearBaselines removes all extra baselines.
// POST /api/baselines/clear
func HandleClearBaselines(c *gin.Context) {
	state.Global.ClearBaselines()
	realign()
	c.JSON(http.StatusOK, gin.H{"baselines": 0})
}

// HandleRegistration changes alignment settings and re-registers the pair.
// POST /api/registration — form: auto=0|1, local=0|1
func HandleRegistration(c *gin.Context) {
	st := state.Global.Settings()
	if v := c.PostForm("auto"); v != "" {
		st.AutoRegister = v != "0"
	}
	if v := c.PostForm("local"); v != "" {
		st.LocalRefine = v != "0"
	}
	state.Global.Configure(st)
	reg, _ := realign()
	c.JSON(http.StatusOK, gin.H{"registration": reg, "auto": st.AutoRegister, "local": st.LocalRefine})
}

// HandleAnalyze runs change detection and returns the highlight image, ranked
// regions and stats.
// POST /api/analyze
func HandleAnalyze(c *gin.Context) {
	an, ok := analysisPair(c)
	if !ok {
		return
	}
	opts := parseDiffOpts(c, an)

	highlightR := uint8(clampInt(parseIntDefault(c.PostForm("highlight_r"), 255), 0, 255))
	highlightG := uint8(clampInt(parseIntDefault(c.PostForm("highlight_g"), 60), 0, 255))
	highlightB := uint8(clampInt(parseIntDefault(c.PostForm("highlight_b"), 60), 0, 255))
	highlightAlpha := parseFloatDefault(c.PostForm("highlight_alpha"), 0.55)
	if highlightAlpha < 0 || highlightAlpha > 1 {
		highlightAlpha = 0.55
	}

	bDims, aDims, _ := state.Global.Dims()
	res := imgproc.ComputeDiffV2(an.Before, an.After, opts)
	stats := imgproc.ChangeStats(res.Mask)
	highlighted := imgproc.HighlightChanges(an.After, res.Mask, [3]uint8{highlightR, highlightG, highlightB}, highlightAlpha)

	s, err := imgproc.EncodeBase64PNG(highlighted)
	if err != nil {
		encodeFailed(c, err)
		return
	}
	ab := an.After.Bounds()
	c.JSON(http.StatusOK, analyzeResponse{
		Stats:        stats,
		Regions:      regionList(imgproc.RankRegions(res.Diff, res.Mask), ab.Dx(), ab.Dy()),
		Images:       map[string]string{"highlight": s},
		BeforeDims:   dimsJSON{W: bDims.W, H: bDims.H},
		AfterDims:    dimsJSON{W: aDims.W, H: aDims.H},
		AnalysisDims: dimsJSON{W: ab.Dx(), H: ab.Dy()},
		Resized:      an.Resized,
		Threshold:    int(res.Threshold),
		Baselines:    an.Baselines,
		Registration: an.Reg,
	})
}

// HandleAnalyzeAlternate computes every alternate view (difference map, channel
// subtraction, heat map, Canny edges, contours) from a single diff pass.
// POST /api/analyze/alternate — same form fields as /api/analyze plus
// canny_low, canny_high, heat_norm.
func HandleAnalyzeAlternate(c *gin.Context) {
	an, ok := analysisPair(c)
	if !ok {
		return
	}
	opts := parseDiffOpts(c, an)
	res := imgproc.ComputeDiffV2(an.Before, an.After, opts)

	low := clampInt(parseIntDefault(c.PostForm("canny_low"), 100), 0, 255)
	high := clampInt(parseIntDefault(c.PostForm("canny_high"), 200), 0, 255)
	if high < low {
		low, high = high, low
	}
	heat := res.Diff
	if c.PostForm("heat_norm") != "0" {
		heat = imgproc.NormalizeHeat(heat)
	}
	pb, pa := imgproc.PrepareImages(an.Before, an.After, opts)
	contoured, _ := imgproc.DrawContours(an.After, res.Mask, [3]uint8{0, 255, 0})

	views := map[string]image.Image{
		"diff":        res.Diff,
		"subtraction": imgproc.Subtract(pb, pa),
		"heatmap":     imgproc.JETColormap(heat),
		"edges":       imgproc.CannyEdge(res.Diff, uint8(low), uint8(high)),
		"contours":    contoured,
	}
	images := make(map[string]string, len(views))
	for name, img := range views {
		s, err := imgproc.EncodeBase64PNG(img)
		if err != nil {
			encodeFailed(c, err)
			return
		}
		images[name] = s
	}
	c.JSON(http.StatusOK, gin.H{"images": images, "threshold": res.Threshold})
}

// HandleWarp warps the raw stored before image using anchor point pairs and
// makes the result the analysis "before".
// POST /api/warp — form fields: src_pts, dst_pts (JSON pixel-coord arrays)
func HandleWarp(c *gin.Context) {
	snap, ok := state.Global.Snapshot()
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "images not ready — upload both before and after first"})
		return
	}

	if err := c.Request.ParseForm(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse form"})
		return
	}

	var rawSrc, rawDst [][2]float64
	if err := json.Unmarshal([]byte(c.PostForm("src_pts")), &rawSrc); err != nil || len(rawSrc) < 4 || len(rawSrc) > 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "src_pts must be a JSON array of 4–8 [x,y] pairs"})
		return
	}
	if err := json.Unmarshal([]byte(c.PostForm("dst_pts")), &rawDst); err != nil || len(rawDst) != len(rawSrc) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dst_pts must be a JSON array with the same number of pairs as src_pts"})
		return
	}

	srcPts := make([]imgproc.Point, len(rawSrc))
	dstPts := make([]imgproc.Point, len(rawDst))
	for i := range rawSrc {
		srcPts[i] = imgproc.Point{X: rawSrc[i][0], Y: rawSrc[i][1]}
		dstPts[i] = imgproc.Point{X: rawDst[i][0], Y: rawDst[i][1]}
	}

	ab := snap.After.Bounds()
	warped, valid, err := imgproc.WarpPerspectiveMasked(snap.Before, srcPts, dstPts, ab.Dx(), ab.Dy())
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	// Store the analysis-resolution version first, so the preview the client
	// receives always corresponds to state the server actually holds.
	an, ok := state.Global.AnalysisPair()
	if !ok {
		c.JSON(http.StatusConflict, gin.H{"error": "images changed during warp — try again"})
		return
	}
	tw, th := an.After.Bounds().Dx(), an.After.Bounds().Dy()
	if !state.Global.SetWarpedBefore(snap.Generation, imgproc.ResizeNRGBA(warped, tw, th), imgproc.ResizeMask(valid, tw, th)) {
		c.JSON(http.StatusConflict, gin.H{"error": "images changed during warp — try again"})
		return
	}

	// Full-resolution warped image for the client preview.
	c.Header("Content-Type", "image/png")
	c.Header("Content-Disposition", `inline; filename="warped-before.png"`)
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(c.Writer, warped); err != nil {
		log.Printf("warp preview encode: %v", err)
	}
}

// HandleAutoWarp returns candidate correspondence pairs for one-click alignment review.
// POST /api/auto-warp
func HandleAutoWarp(c *gin.Context) {
	snap, ok := state.Global.Snapshot()
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "images not ready — upload both before and after first"})
		return
	}

	result, err := imgproc.AutoDetectHomography(snap.Before, snap.After)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	pairs := make([]autoWarpPairResponse, 0, len(result.Pairs))
	for _, pair := range result.Pairs {
		pairs = append(pairs, autoWarpPairResponse{
			Src:        [2]float64{pair.Src.X, pair.Src.Y},
			Dst:        [2]float64{pair.Dst.X, pair.Dst.Y},
			Confidence: pair.Score,
		})
	}

	c.JSON(http.StatusOK, autoWarpResponse{
		Pairs:       pairs,
		Confidence:  result.Confidence,
		MatchCount:  result.MatchCount,
		InlierCount: result.InlierCount,
	})
}

// HandleClearWarp removes the manual warp so analysis reverts to the automatic alignment.
// POST /api/clear-warp
func HandleClearWarp(c *gin.Context) {
	state.Global.ClearWarp()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// HandleImageBefore serves the stored before image as PNG.
// GET /api/image/before
func HandleImageBefore(c *gin.Context) { serveImage(c, state.Global.RawBefore()) }

// HandleImageAfter serves the stored after image as PNG.
// GET /api/image/after
func HandleImageAfter(c *gin.Context) { serveImage(c, state.Global.RawAfter()) }

func serveImage(c *gin.Context, img *image.NRGBA) {
	if img == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no image uploaded"})
		return
	}
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, img); err != nil {
		encodeFailed(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "image/png", buf.Bytes())
}

func analysisPair(c *gin.Context) (state.Analysis, bool) {
	an, ok := state.Global.AnalysisPair()
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "images not ready — upload both before and after first"})
		return state.Analysis{}, false
	}
	return an, true
}

func respondImage(c *gin.Context, img image.Image) {
	s, err := imgproc.EncodeBase64PNG(img)
	if err != nil {
		encodeFailed(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"image": s})
}

func encodeFailed(c *gin.Context, err error) {
	log.Printf("image encode failed: %v", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "encoding failed"})
}

func regionList(regions []imgproc.Region, w, h int) []regionJSON {
	if len(regions) > maxRegionsShown {
		regions = regions[:maxRegionsShown]
	}
	out := make([]regionJSON, 0, len(regions))
	for i, r := range regions {
		out = append(out, regionJSON{
			Rank: i + 1,
			X:    float64(r.Box.Min.X) / float64(w), Y: float64(r.Box.Min.Y) / float64(h),
			W: float64(r.Box.Dx()) / float64(w), H: float64(r.Box.Dy()) / float64(h),
			AreaPx: r.Area, AreaPct: float64(r.Area) / float64(w*h) * 100,
			Score: r.Score, Heat: r.MeanHeat,
		})
	}
	return out
}

// parseDiffOpts extracts DiffOptions from a form POST, clamping every value
// to a sane range. Masks from the alignment stage are attached from an.
func parseDiffOpts(c *gin.Context, an state.Analysis) imgproc.DiffOptions {
	strength := parseIntDefault(c.PostForm("strength"), 60)
	if strength < 0 || strength > 99 {
		strength = 60
	}
	preBlurSigma := parseFloatDefault(c.PostForm("pre_blur_sigma"), 1.5)
	if preBlurSigma < 0 || preBlurSigma > 4.0 {
		preBlurSigma = 1.5
	}
	opts := imgproc.DiffOptions{
		Threshold:      uint8(strength),
		AutoThreshold:  c.PostForm("auto_threshold") == "1",
		MorphSize:      clampInt(parseIntDefault(c.PostForm("morph_size"), 5), 1, 31),
		CloseSize:      clampInt(parseIntDefault(c.PostForm("close_size"), 3), 1, 31),
		MinRegion:      clampInt(parseIntDefault(c.PostForm("min_region"), 25), 1, 1000000),
		PreBlurSigma:   preBlurSigma,
		NormalizeLuma:  c.PostForm("normalize_luma") != "0",
		MatchIntensity: c.PostForm("match_intensity") != "0",
		ColorWeight:    clampFloat(parseFloatDefault(c.PostForm("color_weight"), 1.0), 0, 4),
		ShiftTol:       clampInt(parseIntDefault(c.PostForm("shift_tol"), 0), 0, 4),
		BorderPct:      clampFloat(parseFloatDefault(c.PostForm("border_pct"), 0.01), 0, 0.2),
		Valid:          an.Valid,
		Spread:         an.Spread,
	}
	var raw [][4]float64
	if err := json.Unmarshal([]byte(c.PostForm("ignore")), &raw); err == nil {
		for _, r := range raw {
			opts.Ignore = append(opts.Ignore, imgproc.Rect{
				X0: clampFloat(r[0], 0, 1), Y0: clampFloat(r[1], 0, 1),
				X1: clampFloat(r[2], 0, 1), Y1: clampFloat(r[3], 0, 1),
			})
		}
	}
	return opts
}

// decodeFormImage reads the uploaded file (size-capped), rejects absurd
// dimensions before decoding, decodes it and applies any EXIF rotation.
func decodeFormImage(c *gin.Context, field string) (image.Image, state.Dims, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes)
	if err := c.Request.ParseMultipartForm(64 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse form: " + err.Error()})
		return nil, state.Dims{}, false
	}
	f, _, err := c.Request.FormFile(field)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing '" + field + "' field"})
		return nil, state.Dims{}, false
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read upload: " + err.Error()})
		return nil, state.Dims{}, false
	}
	img, dims, status, err := decodeImageBytes(data)
	if err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return nil, state.Dims{}, false
	}
	return img, dims, true
}

// decodeImageBytes rejects absurd dimensions before decoding, decodes the
// image and applies any EXIF rotation. On failure it returns an HTTP status.
func decodeImageBytes(data []byte) (image.Image, state.Dims, int, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, state.Dims{}, http.StatusBadRequest, fmt.Errorf("failed to decode image: %w", err)
	}
	if float64(cfg.Width)*float64(cfg.Height) > maxUploadPixels {
		return nil, state.Dims{}, http.StatusRequestEntityTooLarge, fmt.Errorf("image is too large (over 120 megapixels)")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, state.Dims{}, http.StatusBadRequest, fmt.Errorf("failed to decode image: %w", err)
	}
	if o := imgproc.ExifOrientation(data); o > 1 {
		img = imgproc.ApplyOrientation(imgproc.ToNRGBA(img), o)
	}
	b := img.Bounds()
	return img, state.Dims{W: b.Dx(), H: b.Dy()}, http.StatusOK, nil
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func parseFloatDefault(s string, def float64) float64 {
	if s == "" {
		return def
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}
	return v
}
