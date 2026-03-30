package api

import (
	"encoding/json"
	"image"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/skinnyrad/tscm-change-detection/internal/imgproc"
	"github.com/skinnyrad/tscm-change-detection/internal/state"
)

type dimsJSON struct {
	W int `json:"w"`
	H int `json:"h"`
}

type singleUploadResponse struct {
	Dims    dimsJSON `json:"dims"`
	Version int64    `json:"version"`
}

type analyzeResponse struct {
	Stats      imgproc.Stats     `json:"stats"`
	Images     map[string]string `json:"images"`
	BeforeDims dimsJSON          `json:"before_dims"`
	AfterDims  dimsJSON          `json:"after_dims"`
	Resized    bool              `json:"resized"`
}

// HandleUploadBefore decodes and stores the before image, then triggers
// alignment if the after image is already present.
// POST /api/upload/before — multipart/form-data: image
func HandleUploadBefore(c *gin.Context) {
	img, dims, ok := decodeFormImage(c, "image")
	if !ok {
		return
	}
	nrgba := imgproc.ToNRGBA(img)
	state.Global.SetBefore(nrgba, dims)

	// Pre-compute alignment if after is already uploaded.
	if after := state.Global.RawAfter(); after != nil {
		aligned, alignedAfter, resized := imgproc.AlignNRGBA(nrgba, after)
		state.Global.SetAligned(
			imgproc.DownsampleNRGBA(aligned, imgproc.MaxAnalysisDim),
			imgproc.DownsampleNRGBA(alignedAfter, imgproc.MaxAnalysisDim),
			resized,
		)
	}

	c.JSON(http.StatusOK, singleUploadResponse{
		Dims:    dimsJSON{W: dims.W, H: dims.H},
		Version: state.Global.BeforeVersion(),
	})
}

// HandleUploadAfter decodes and stores the after image, then triggers
// alignment if the before image is already present.
// POST /api/upload/after — multipart/form-data: image
func HandleUploadAfter(c *gin.Context) {
	img, dims, ok := decodeFormImage(c, "image")
	if !ok {
		return
	}
	nrgba := imgproc.ToNRGBA(img)
	state.Global.SetAfter(nrgba, dims)

	// Pre-compute alignment if before is already uploaded.
	if before := state.Global.RawBefore(); before != nil {
		aligned, alignedAfter, resized := imgproc.AlignNRGBA(before, nrgba)
		state.Global.SetAligned(
			imgproc.DownsampleNRGBA(aligned, imgproc.MaxAnalysisDim),
			imgproc.DownsampleNRGBA(alignedAfter, imgproc.MaxAnalysisDim),
			resized,
		)
	}

	c.JSON(http.StatusOK, singleUploadResponse{
		Dims:    dimsJSON{W: dims.W, H: dims.H},
		Version: state.Global.AfterVersion(),
	})
}

// HandleAnalyze runs change detection and returns the highlight image + stats.
// POST /api/analyze
func HandleAnalyze(c *gin.Context) {
	if !state.Global.HasImages() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "images not ready — upload both before and after first"})
		return
	}

	opts := parseDiffOpts(c)

	highlightR := uint8(parseIntDefault(c.PostForm("highlight_r"), 255))
	highlightG := uint8(parseIntDefault(c.PostForm("highlight_g"), 60))
	highlightB := uint8(parseIntDefault(c.PostForm("highlight_b"), 60))
	highlightAlpha := parseFloatDefault(c.PostForm("highlight_alpha"), 0.55)
	if highlightAlpha < 0 || highlightAlpha > 1 {
		highlightAlpha = 0.55
	}

	before, after, resized := state.Global.AnalysisPair()
	bDims, aDims, _ := state.Global.Dims()

	_, thresh := imgproc.ComputeDiffV2(before, after, opts)
	stats := imgproc.ChangeStats(thresh)

	highlighted := imgproc.HighlightChanges(after, thresh, [3]uint8{highlightR, highlightG, highlightB}, highlightAlpha)

	images := map[string]string{}
	if s, err := imgproc.EncodeBase64PNG(highlighted); err == nil {
		images["highlight"] = s
	}

	c.JSON(http.StatusOK, analyzeResponse{
		Stats:      stats,
		Images:     images,
		BeforeDims: dimsJSON{W: bDims.W, H: bDims.H},
		AfterDims:  dimsJSON{W: aDims.W, H: aDims.H},
		Resized:    resized,
	})
}

// HandleAnalyzeDiff returns the raw grayscale difference map.
// POST /api/analyze/diff
func HandleAnalyzeDiff(c *gin.Context) {
	if !state.Global.HasImages() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "images not ready"})
		return
	}
	opts := parseDiffOpts(c)
	before, after, _ := state.Global.AnalysisPair()
	diff, _ := imgproc.ComputeDiffV2(before, after, opts)
	s, err := imgproc.EncodeBase64PNG(diff)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "encoding failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"image": s})
}

// HandleAnalyzeSubtraction returns the per-channel colour subtraction image.
// POST /api/analyze/subtraction
func HandleAnalyzeSubtraction(c *gin.Context) {
	if !state.Global.HasImages() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "images not ready"})
		return
	}
	opts := parseDiffOpts(c)
	rawBefore, rawAfter, _ := state.Global.AnalysisPair()
	before, after := imgproc.PrepareImages(rawBefore, rawAfter, opts)
	subtracted := imgproc.Subtract(before, after)
	s, err := imgproc.EncodeBase64PNG(subtracted)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "encoding failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"image": s})
}

// HandleAnalyzeHeatmap returns the JET-colourmap heat map of differences.
// POST /api/analyze/heatmap
func HandleAnalyzeHeatmap(c *gin.Context) {
	if !state.Global.HasImages() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "images not ready"})
		return
	}
	opts := parseDiffOpts(c)
	before, after, _ := state.Global.AnalysisPair()
	diff, _ := imgproc.ComputeDiffV2(before, after, opts)
	heatmap := imgproc.JETColormap(diff)
	s, err := imgproc.EncodeBase64PNG(heatmap)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "encoding failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"image": s})
}

// HandleAnalyzeCanny returns Canny edge detection on the diff map and contours
// drawn on the after image. Canny thresholds are fixed at sensible defaults.
// POST /api/analyze/canny
func HandleAnalyzeCanny(c *gin.Context) {
	if !state.Global.HasImages() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "images not ready"})
		return
	}
	opts := parseDiffOpts(c)
	before, after, _ := state.Global.AnalysisPair()
	diff, thresh := imgproc.ComputeDiffV2(before, after, opts)

	const cannyLow, cannyHigh = uint8(100), uint8(200)
	edges := imgproc.CannyEdge(diff, cannyLow, cannyHigh)
	contoured, _ := imgproc.DrawContours(after, thresh, [3]uint8{0, 255, 0})

	images := map[string]string{}
	if s, err := imgproc.EncodeBase64PNG(edges); err == nil {
		images["edges"] = s
	}
	if s, err := imgproc.EncodeBase64PNG(contoured); err == nil {
		images["contours"] = s
	}
	c.JSON(http.StatusOK, images)
}

// HandleWarp warps the raw stored before image using anchor point pairs.
// POST /api/warp — form fields: src_pts, dst_pts (JSON pixel-coord arrays)
func HandleWarp(c *gin.Context) {
	if !state.Global.HasImages() {
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

	rawBefore := state.Global.RawBefore()
	rawAfter := state.Global.RawAfter()
	ab := rawAfter.Bounds()

	warped, err := imgproc.WarpPerspective(rawBefore, srcPts, dstPts, ab.Dx(), ab.Dy())
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	// Send full-resolution warped image to client for preview display.
	c.Header("Content-Type", "image/png")
	c.Header("Content-Disposition", `inline; filename="warped-before.png"`)
	warpEnc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := warpEnc.Encode(c.Writer, warped); err != nil {
		return
	}

	// Store downsampled version so it matches the analysis-resolution aligned pair.
	state.Global.SetWarpedBefore(imgproc.DownsampleNRGBA(warped, imgproc.MaxAnalysisDim))
}

// HandleClearWarp removes the stored warp so analysis reverts to alignedBefore.
// POST /api/clear-warp
func HandleClearWarp(c *gin.Context) {
	state.Global.ClearWarp()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// HandleImageBefore serves the stored before image as PNG.
// GET /api/image/before
func HandleImageBefore(c *gin.Context) {
	img := state.Global.RawBefore()
	if img == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no image uploaded"})
		return
	}
	c.Header("Content-Type", "image/png")
	c.Header("Cache-Control", "no-store")
	(&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(c.Writer, img) //nolint:errcheck
}

// HandleImageAfter serves the stored after image as PNG.
// GET /api/image/after
func HandleImageAfter(c *gin.Context) {
	img := state.Global.RawAfter()
	if img == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no image uploaded"})
		return
	}
	c.Header("Content-Type", "image/png")
	c.Header("Cache-Control", "no-store")
	(&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(c.Writer, img) //nolint:errcheck
}

// parseDiffOpts extracts shared DiffOptions parameters from a form POST.
func parseDiffOpts(c *gin.Context) imgproc.DiffOptions {
	strength := parseIntDefault(c.PostForm("strength"), 60)
	if strength < 0 || strength > 99 {
		strength = 60
	}
	minRegion := parseIntDefault(c.PostForm("min_region"), 25)
	if minRegion < 1 {
		minRegion = 1
	}
	morphSize := parseIntDefault(c.PostForm("morph_size"), 5)
	if morphSize < 1 {
		morphSize = 1
	}
	closeSize := parseIntDefault(c.PostForm("close_size"), 3)
	if closeSize < 1 {
		closeSize = 1
	}
	preBlurSigma := parseFloatDefault(c.PostForm("pre_blur_sigma"), 1.5)
	if preBlurSigma < 0 || preBlurSigma > 3.0 {
		preBlurSigma = 1.5
	}
	normLuma := c.PostForm("normalize_luma") != "0"
	return imgproc.DiffOptions{
		Threshold:     uint8(strength),
		MorphSize:     morphSize,
		CloseSize:     closeSize,
		MinRegion:     minRegion,
		PreBlurSigma:  preBlurSigma,
		NormalizeLuma: normLuma,
	}
}

// decodeFormImage parses the multipart form and decodes the named image file.
func decodeFormImage(c *gin.Context, field string) (image.Image, state.Dims, bool) {
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
	img, _, err := image.Decode(f)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to decode image: " + err.Error()})
		return nil, state.Dims{}, false
	}
	b := img.Bounds()
	return img, state.Dims{W: b.Dx(), H: b.Dy()}, true
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

