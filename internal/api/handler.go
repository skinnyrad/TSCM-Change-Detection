package api

import (
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	_ "github.com/jdeng/goheif"
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

// HandleAnalyze runs change detection against the pre-aligned stored images.
// POST /api/analyze — form fields: method, strength, canny_low, canny_high
func HandleAnalyze(c *gin.Context) {
	if !state.Global.HasImages() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "images not ready — upload both before and after first"})
		return
	}

	method := c.PostForm("method")
	if method == "" {
		method = "basic"
	}
	validMethods := map[string]bool{
		"basic": true, "subtraction": true, "threshold": true,
		"heatmap": true, "advanced": true,
	}
	if !validMethods[method] {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid method: %q", method)})
		return
	}

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

	cannyLow := parseIntDefault(c.PostForm("canny_low"), 100)
	if cannyLow < 0 || cannyLow > 255 {
		cannyLow = 100
	}
	cannyHigh := parseIntDefault(c.PostForm("canny_high"), 200)
	if cannyHigh < 0 || cannyHigh > 255 {
		cannyHigh = 200
	}

	if method == "advanced" && cannyLow >= cannyHigh {
		c.JSON(http.StatusBadRequest, gin.H{"error": "canny_low must be less than canny_high"})
		return
	}

	before, after, resized := state.Global.AnalysisPair()
	bDims, aDims, _ := state.Global.Dims()

	diff, thresh := imgproc.ComputeDiff(before, after, uint8(strength), morphSize, minRegion)
	stats := imgproc.ChangeStats(thresh)

	images := map[string]string{}
	highlightColor := [3]uint8{255, 60, 60}

	switch method {
	case "basic":
		if s, err := imgproc.EncodeBase64PNG(diff); err == nil {
			images["diff_map"] = s
		}
		if s, err := imgproc.EncodeBase64PNG(thresh); err == nil {
			images["threshold_mask"] = s
		}
		highlighted := imgproc.HighlightChanges(after, thresh, highlightColor, 0.55)
		if s, err := imgproc.EncodeBase64PNG(highlighted); err == nil {
			images["highlight"] = s
		}

	case "subtraction":
		subtracted := imgproc.Subtract(before, after)
		if s, err := imgproc.EncodeBase64PNG(subtracted); err == nil {
			images["subtraction"] = s
		}
		highlighted := imgproc.HighlightChanges(after, thresh, highlightColor, 0.55)
		if s, err := imgproc.EncodeBase64PNG(highlighted); err == nil {
			images["highlight"] = s
		}

	case "threshold":
		if s, err := imgproc.EncodeBase64PNG(thresh); err == nil {
			images["threshold_mask"] = s
		}
		highlighted := imgproc.HighlightChanges(after, thresh, highlightColor, 0.55)
		if s, err := imgproc.EncodeBase64PNG(highlighted); err == nil {
			images["highlight"] = s
		}

	case "heatmap":
		heatmap := imgproc.JETColormap(diff)
		if s, err := imgproc.EncodeBase64PNG(heatmap); err == nil {
			images["heatmap"] = s
		}
		highlighted := imgproc.HighlightChanges(after, thresh, highlightColor, 0.55)
		if s, err := imgproc.EncodeBase64PNG(highlighted); err == nil {
			images["highlight"] = s
		}

	case "advanced":
		edges := imgproc.CannyEdge(diff, uint8(cannyLow), uint8(cannyHigh))
		if s, err := imgproc.EncodeBase64PNG(edges); err == nil {
			images["edges"] = s
		}
		contoured, _ := imgproc.DrawContours(after, thresh, [3]uint8{0, 255, 0})
		if s, err := imgproc.EncodeBase64PNG(contoured); err == nil {
			images["contours"] = s
		}
	}

	c.JSON(http.StatusOK, analyzeResponse{
		Stats:      stats,
		Images:     images,
		BeforeDims: dimsJSON{W: bDims.W, H: bDims.H},
		AfterDims:  dimsJSON{W: aDims.W, H: aDims.H},
		Resized:    resized,
	})
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
