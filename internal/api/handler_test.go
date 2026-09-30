package api

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/skinnyrad/tscm-change-detection/internal/state"
)

func testRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/api")
	g.POST("/upload/before", HandleUploadBefore)
	g.POST("/upload/after", HandleUploadAfter)
	g.POST("/upload/baseline", HandleUploadBaseline)
	g.POST("/baselines/clear", HandleClearBaselines)
	g.POST("/registration", HandleRegistration)
	g.POST("/analyze", HandleAnalyze)
	g.POST("/analyze/alternate", HandleAnalyzeAlternate)
	g.POST("/warp", HandleWarp)
	g.GET("/image/before", HandleImageBefore)
	return r
}

func pngBytes(t *testing.T, box image.Rectangle) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 160, 120))
	for y := 0; y < 120; y++ {
		for x := 0; x < 160; x++ {
			v := uint8((x*7 + y*13) % 200)
			c := color.NRGBA{v, v, v, 255}
			if image.Pt(x, y).In(box) {
				c = color.NRGBA{255, 0, 0, 255}
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

func upload(t *testing.T, r http.Handler, path string, data []byte) *httptest.ResponseRecorder {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("image", "x.png")
	fw.Write(data)
	mw.Close()
	req := httptest.NewRequest("POST", path, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func post(r http.Handler, path string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAnalyzeBeforeUploadIs400(t *testing.T) {
	state.Global = &state.Store{}
	if w := post(testRouter(), "/api/analyze", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("code %d", w.Code)
	}
}

func TestUploadAnalyzeReturnsRegions(t *testing.T) {
	state.Global = &state.Store{}
	r := testRouter()
	if w := upload(t, r, "/api/upload/before", pngBytes(t, image.Rectangle{})); w.Code != 200 {
		t.Fatalf("upload before: %d %s", w.Code, w.Body)
	}
	if w := upload(t, r, "/api/upload/after", pngBytes(t, image.Rect(40, 30, 90, 80))); w.Code != 200 {
		t.Fatalf("upload after: %d %s", w.Code, w.Body)
	}
	w := post(r, "/api/analyze", url.Values{"strength": {"40"}})
	if w.Code != 200 {
		t.Fatalf("analyze: %d %s", w.Code, w.Body)
	}
	var resp analyzeResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Regions) != 1 || resp.Regions[0].Rank != 1 || resp.Regions[0].Score != 1 {
		t.Fatalf("regions: %+v", resp.Regions)
	}
	if resp.Images["highlight"] == "" || resp.Stats.Regions != 1 {
		t.Fatalf("response: %+v", resp.Stats)
	}
	w = post(r, "/api/analyze/alternate", url.Values{"canny_low": {"300"}, "ignore": {"not json"}})
	if w.Code != 200 {
		t.Fatalf("alternate: %d %s", w.Code, w.Body)
	}
	var alt struct{ Images map[string]string }
	if err := json.Unmarshal(w.Body.Bytes(), &alt); err != nil || len(alt.Images) != 5 {
		t.Fatalf("alternate images: %v %d", err, len(alt.Images))
	}
}

func TestBaselineRequiresBeforeAndClears(t *testing.T) {
	state.Global = &state.Store{}
	r := testRouter()
	if w := upload(t, r, "/api/upload/baseline", pngBytes(t, image.Rectangle{})); w.Code != http.StatusBadRequest {
		t.Fatalf("baseline without before: %d", w.Code)
	}
	upload(t, r, "/api/upload/before", pngBytes(t, image.Rectangle{}))
	upload(t, r, "/api/upload/after", pngBytes(t, image.Rectangle{}))
	if w := upload(t, r, "/api/upload/baseline", pngBytes(t, image.Rectangle{})); w.Code != 200 {
		t.Fatalf("baseline: %d %s", w.Code, w.Body)
	}
	if w := post(r, "/api/analyze", nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"baselines":2`) {
		t.Fatalf("analyze with baselines: %d %s", w.Code, w.Body.String()[:200])
	}
	post(r, "/api/baselines/clear", nil)
	if state.Global.Baselines() != 0 {
		t.Fatal("baselines not cleared")
	}
}

func TestRejectsGarbageUpload(t *testing.T) {
	state.Global = &state.Store{}
	if w := upload(t, testRouter(), "/api/upload/before", []byte("not an image")); w.Code != http.StatusBadRequest {
		t.Fatalf("code %d", w.Code)
	}
}

func TestConcurrentUploadsStayConsistent(t *testing.T) {
	state.Global = &state.Store{}
	r := testRouter()
	a, b := pngBytes(t, image.Rectangle{}), pngBytes(t, image.Rect(10, 10, 40, 40))
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				upload(t, r, "/api/upload/before", a)
			} else {
				upload(t, r, "/api/upload/after", b)
			}
			post(r, "/api/analyze", nil)
		}(i)
	}
	wg.Wait()
	upload(t, r, "/api/upload/before", a)
	upload(t, r, "/api/upload/after", b)
	if w := post(r, "/api/analyze", nil); w.Code != 200 {
		t.Fatalf("final analyze: %d", w.Code)
	}
}
