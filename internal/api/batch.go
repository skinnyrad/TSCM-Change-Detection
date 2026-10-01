package api

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/skinnyrad/tscm-change-detection/internal/imgproc"
	"github.com/skinnyrad/tscm-change-detection/internal/state"
)

// batchDisplayDim is the longest edge kept for each batch image (display and
// analysis source). Originals are not retained.
const batchDisplayDim = 1600

type batchItemJSON struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	W    int    `json:"w"`
	H    int    `json:"h"`
}

type batchRegionJSON struct {
	Rank    int     `json:"rank"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	W       float64 `json:"w"`
	H       float64 `json:"h"`
	AreaPct float64 `json:"area_pct"`
	Score   float64 `json:"score"`
}

type batchImageJSON struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Score      float64           `json:"score"`
	Z          float64           `json:"z"`
	Anomaly    bool              `json:"anomaly"`
	Golden     bool              `json:"golden"`
	Registered bool              `json:"registered"`
	Anchor     bool              `json:"anchor"`
	Message    string            `json:"message"`
	Regions    []batchRegionJSON `json:"regions"`
}

type batchResultJSON struct {
	Images       []batchImageJSON `json:"images"`
	AnchorID     string           `json:"anchor_id"`
	GoldenCount  int              `json:"golden_count"`
	Unregistered int              `json:"unregistered"`
	ScoreMedian  float64          `json:"score_median"`
	ScoreSpread  float64          `json:"score_spread"`
	PixelZ       float64          `json:"pixel_z"`
	MinScore     float64          `json:"min_score"` // absolute floor on an anomalous image's score
	ImageZ       float64          `json:"image_z"`
	Threshold    float64          `json:"threshold"`
	FrameW       int              `json:"frame_w"`
	FrameH       int              `json:"frame_h"`
}

// HandleBatchUpload adds images to the batch.
// POST /api/batch/images — multipart/form-data: images (repeated)
func HandleBatchUpload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes)
	if err := c.Request.ParseMultipartForm(64 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse form: " + err.Error()})
		return
	}
	files := c.Request.MultipartForm.File["images"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no 'images' files in the request"})
		return
	}
	type failure struct {
		Name  string `json:"name"`
		Error string `json:"error"`
	}
	var items []*state.BatchItem
	var failures []failure
	for _, fh := range files {
		item, err := ingestBatchFile(fh.Filename, func() (io.ReadCloser, error) { return fh.Open() })
		if err != nil {
			failures = append(failures, failure{Name: fh.Filename, Error: err.Error()})
			continue
		}
		items = append(items, item)
	}
	count, ok := state.Global.Batch().Add(items...)
	if !ok {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("cannot add images while an analysis is running or beyond %d images", state.MaxBatchImages), "count": count})
		return
	}
	added := make([]batchItemJSON, 0, len(items))
	for _, it := range items {
		added = append(added, batchItemJSON{ID: it.ID, Name: it.Name, W: it.W, H: it.H})
	}
	c.JSON(http.StatusOK, gin.H{"count": count, "added": added, "failed": failures})
}

func ingestBatchFile(name string, open func() (io.ReadCloser, error)) (*state.BatchItem, error) {
	f, err := open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	img, dims, _, err := decodeImageBytes(data)
	if err != nil {
		return nil, err
	}
	disp := imgproc.DownsampleNRGBA(imgproc.ToNRGBA(img), batchDisplayDim)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, disp, &jpeg.Options{Quality: 90}); err != nil {
		return nil, err
	}
	id := make([]byte, 8)
	rand.Read(id)
	return &state.BatchItem{ID: hex.EncodeToString(id), Name: name, W: dims.W, H: dims.H, Display: buf.Bytes()}, nil
}

// HandleBatchClear removes all batch images and results.
// POST /api/batch/clear
func HandleBatchClear(c *gin.Context) {
	if !state.Global.Batch().Clear() {
		c.JSON(http.StatusConflict, gin.H{"error": "an analysis is running"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": 0})
}

// HandleBatchStatus returns the batch contents and job progress.
// GET /api/batch
func HandleBatchStatus(c *gin.Context) {
	b := state.Global.Batch()
	items := b.Items()
	list := make([]batchItemJSON, 0, len(items))
	for _, it := range items {
		list = append(list, batchItemJSON{ID: it.ID, Name: it.Name, W: it.W, H: it.H})
	}
	res, _ := b.Result()
	c.JSON(http.StatusOK, gin.H{"items": list, "job": b.Job(), "has_results": res != nil})
}

// HandleBatchAnalyze starts analysing the batch in the background.
// POST /api/batch/analyze — optional form: pixel_z, image_z
func HandleBatchAnalyze(c *gin.Context) {
	b := state.Global.Batch()
	items, ok := b.Start()
	if !ok {
		c.JSON(http.StatusConflict, gin.H{"error": "need at least 3 images, and no analysis already running"})
		return
	}
	opts := imgproc.BatchOptions{
		PixelZ: clampFloat(parseFloatDefault(c.PostForm("pixel_z"), 5), 2, 20),
		ImageZ: clampFloat(parseFloatDefault(c.PostForm("image_z"), 6), 1, 50),
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("batch analysis panic: %v", r)
				b.Finish(nil, nil, fmt.Errorf("analysis failed: %v", r))
			}
		}()
		load := func(i int) (*image.NRGBA, error) {
			img, err := jpeg.Decode(bytes.NewReader(items[i].Display))
			if err != nil {
				return nil, err
			}
			return imgproc.ToNRGBA(img), nil
		}
		res := imgproc.AnalyzeBatchFunc(len(items), load, opts, func(done, _ int) { b.Progress(done) })
		ids := make([]string, len(items))
		for i, it := range items {
			ids[i] = it.ID
		}
		b.Finish(res, ids, nil)
	}()
	c.JSON(http.StatusAccepted, gin.H{"job": b.Job()})
}

// HandleBatchResults returns ranked results (most anomalous first).
// GET /api/batch/results
func HandleBatchResults(c *gin.Context) {
	b := state.Global.Batch()
	res, ids := b.Result()
	if res == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no results yet"})
		return
	}
	out := batchResultJSON{
		AnchorID: ids[res.Anchor], GoldenCount: res.GoldenCount, Unregistered: res.Unregistered,
		ScoreMedian: res.ScoreMedian, ScoreSpread: res.ScoreSpread, PixelZ: res.Opts.PixelZ, MinScore: res.Opts.PixelZ * res.Opts.MinScoreFactor,
		ImageZ: res.Opts.ImageZ, Threshold: res.Threshold, FrameW: res.FrameW, FrameH: res.FrameH,
	}
	for i, im := range res.Images {
		item := b.Item(ids[i])
		name := ""
		if item != nil {
			name = item.Name
		}
		regions := make([]batchRegionJSON, 0, len(im.Regions))
		for k, r := range im.Regions {
			regions = append(regions, batchRegionJSON{
				Rank: k + 1,
				X:    float64(r.Box.Min.X) / float64(res.W), Y: float64(r.Box.Min.Y) / float64(res.H),
				W: float64(r.Box.Dx()) / float64(res.W), H: float64(r.Box.Dy()) / float64(res.H),
				AreaPct: float64(r.Area) / float64(res.W*res.H) * 100, Score: r.Score,
			})
		}
		out.Images = append(out.Images, batchImageJSON{
			ID: ids[i], Name: name, Score: im.Score, Z: im.Z, Anomaly: im.Anomaly, Golden: im.Golden,
			Registered: im.Registered, Anchor: i == res.Anchor, Message: im.Reg.Message, Regions: regions,
		})
	}
	sort.SliceStable(out.Images, func(a, b int) bool { return out.Images[a].Score > out.Images[b].Score })
	c.JSON(http.StatusOK, out)
}

// HandleBatchImage serves a batch image.
// GET /api/batch/image/:id?kind=aligned|original|heat&size=N
//   - aligned: warped into the reference frame (so result boxes line up), JPEG
//   - original: the uploaded image as stored, JPEG
//   - heat: RGBA heat overlay in the reference frame, PNG
func HandleBatchImage(c *gin.Context) {
	b := state.Global.Batch()
	id, kind := c.Param("id"), c.DefaultQuery("kind", "aligned")
	size := clampInt(parseIntDefault(c.Query("size"), 1024), 64, batchDisplayDim)
	item := b.Item(id)
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no such image"})
		return
	}
	if kind == "original" {
		c.Header("Cache-Control", "private, max-age=3600")
		c.Data(http.StatusOK, "image/jpeg", item.Display)
		return
	}
	res, ids := b.Result()
	idx := -1
	for i, v := range ids {
		if v == id {
			idx = i
		}
	}
	if res == nil || idx < 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "no results for this image yet"})
		return
	}
	key := kind + ":" + id + ":" + strconv.Itoa(size)
	data, err := b.Cached(key, func() ([]byte, error) {
		var buf bytes.Buffer
		switch kind {
		case "heat":
			err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, res.HeatImage(idx))
			return buf.Bytes(), err
		case "aligned":
			img, err := alignedDisplay(res, idx, item, size)
			if err != nil {
				return nil, err
			}
			err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 88})
			return buf.Bytes(), err
		default:
			return nil, fmt.Errorf("unknown kind %q", kind)
		}
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ct := "image/jpeg"
	if kind == "heat" {
		ct = "image/png"
	}
	c.Header("Cache-Control", "private, max-age=3600")
	c.Data(http.StatusOK, ct, data)
}

// alignedDisplay warps a batch image's display JPEG into the reference frame
// at up to size px on the long edge, composing: display → work coords (scale),
// work → reference work frame (registration transform), frame → output (scale).
func alignedDisplay(res *imgproc.BatchResult, idx int, item *state.BatchItem, size int) (*image.NRGBA, error) {
	src, err := jpeg.Decode(bytes.NewReader(item.Display))
	if err != nil {
		return nil, err
	}
	disp := imgproc.ToNRGBA(src)
	dw, dh := disp.Bounds().Dx(), disp.Bounds().Dy()
	fi := min(1, float64(res.Opts.WorkDim)/float64(max(dw, dh))) // display → work
	long := max(res.FrameW, res.FrameH)
	ko := min(float64(size), float64(batchDisplayDim)) / float64(long) // frame → output
	ow, oh := max(1, int(float64(res.FrameW)*ko+0.5)), max(1, int(float64(res.FrameH)*ko+0.5))
	t := res.Images[idx].Transform
	m := imgproc.ComposeHomography([9]float64{ko, 0, 0, 0, ko, 0, 0, 0, 1}, t, [9]float64{fi, 0, 0, 0, fi, 0, 0, 0, 1})
	out, _ := imgproc.WarpHomography(disp, m, ow, oh)
	return out, nil
}

// HandleBatchReference serves the golden-set median image (reference frame).
// GET /api/batch/reference
func HandleBatchReference(c *gin.Context) {
	b := state.Global.Batch()
	res, _ := b.Result()
	if res == nil || res.Reference == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no results yet"})
		return
	}
	data, err := b.Cached("reference", func() ([]byte, error) {
		var buf bytes.Buffer
		err := png.Encode(&buf, res.Reference)
		return buf.Bytes(), err
	})
	if err != nil {
		encodeFailed(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "image/png", data)
}
