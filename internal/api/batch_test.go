package api

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/skinnyrad/tscm-change-detection/internal/state"
)

func batchRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/api")
	g.GET("/batch", HandleBatchStatus)
	g.POST("/batch/images", HandleBatchUpload)
	g.POST("/batch/clear", HandleBatchClear)
	g.POST("/batch/analyze", HandleBatchAnalyze)
	g.GET("/batch/results", HandleBatchResults)
	g.GET("/batch/image/:id", HandleBatchImage)
	g.GET("/batch/reference", HandleBatchReference)
	return r
}

// scene renders a textured frame with a small offset (handheld jitter), and
// optionally a planted object.
func scene(t *testing.T, dx int, plant bool) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 320, 240))
	for y := 0; y < 240; y++ {
		for x := 0; x < 320; x++ {
			sx := float64(x + dx)
			v := 128 + 60*math.Sin(sx/7)*math.Cos(float64(y)/11) + 30*math.Sin((sx+float64(y))/23)
			c := color.NRGBA{uint8(v), uint8(255 - v/2), uint8(v / 2), 255}
			if plant && x > 150 && x < 190 && y > 100 && y < 140 {
				c = color.NRGBA{250, 20, 20, 255}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestBatchEndToEnd(t *testing.T) {
	state.Batch.Clear()
	r := batchRouter()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for i := 0; i < 12; i++ {
		fw, _ := mw.CreateFormFile("images", "shot.png")
		fw.Write(scene(t, i%3, i == 7))
	}
	fw, _ := mw.CreateFormFile("images", "broken.png")
	fw.Write([]byte("not an image"))
	mw.Close()
	req := httptest.NewRequest("POST", "/api/batch/images", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("upload: %d %s", w.Code, w.Body)
	}
	var up struct {
		Count  int
		Added  []batchItemJSON
		Failed []struct{ Name string }
	}
	json.Unmarshal(w.Body.Bytes(), &up)
	if up.Count != 12 || len(up.Failed) != 1 {
		t.Fatalf("upload result: count=%d failed=%d", up.Count, len(up.Failed))
	}
	planted := up.Added[7].ID

	if w := post(r, "/api/batch/analyze", nil); w.Code != http.StatusAccepted {
		t.Fatalf("analyze: %d %s", w.Code, w.Body)
	}
	deadline := time.Now().Add(30 * time.Second)
	for state.Batch.Job().State == "running" && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if st := state.Batch.Job(); st.State != "done" {
		t.Fatalf("job: %+v", st)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/batch/results", nil))
	var res batchResultJSON
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil || len(res.Images) != 12 {
		t.Fatalf("results: %v %s", err, w.Body.String()[:min(300, w.Body.Len())])
	}
	top := res.Images[0]
	if top.ID != planted || !top.Anomaly || len(top.Regions) == 0 {
		t.Fatalf("top result should be the planted shot: %+v", top)
	}
	for _, im := range res.Images[1:] {
		if im.Anomaly {
			t.Fatalf("unexpected anomaly: %+v", im)
		}
	}
	for _, path := range []string{"/api/batch/image/" + planted + "?kind=aligned&size=200", "/api/batch/image/" + planted + "?kind=heat", "/api/batch/reference"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || w.Body.Len() == 0 {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	if w := post(r, "/api/batch/clear", nil); w.Code != 200 || len(state.Batch.Items()) != 0 {
		t.Fatalf("clear: %d", w.Code)
	}
}
