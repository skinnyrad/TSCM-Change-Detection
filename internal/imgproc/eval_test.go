package imgproc

import (
	"encoding/json"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestEvalLab scores the default pipeline against the change-detection lab.
// It is skipped unless LAB_DIR points at the lab root (the folder holding
// solutions/summary.json). Note summary.json holds the boxes found by the
// course's OpenCV reference solution, not hand-labelled ground truth, so treat
// the numbers as relative (before/after a change), not absolute.
//
//	LAB_DIR=~/.../change-detection go test ./internal/imgproc -run TestEvalLab -v
func TestEvalLab(t *testing.T) {
	root := os.Getenv("LAB_DIR")
	if root == "" {
		t.Skip("LAB_DIR not set")
	}
	raw, err := os.ReadFile(filepath.Join(root, "solutions", "summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	var summary map[string]struct {
		Files []string                   `json:"files"`
		Boxes []struct{ X, Y, W, H int } `json:"boxes"`
	}
	if err := json.Unmarshal(raw, &summary); err != nil {
		t.Fatal(err)
	}
	labs := make([]string, 0, len(summary))
	for k := range summary {
		labs = append(labs, k)
	}
	sort.Strings(labs)

	cfg, opts := evalOpts(os.Getenv("CFG"))
	t.Logf("cfg=%v", cfg)
	var totRef, totHit, totDet, totMatched int
	for _, lab := range labs {
		s := summary[lab]
		dir := filepath.Join(root, "solutions", "solution_"+lab[len("lab"):])
		b, err1 := loadNRGBA(filepath.Join(dir, s.Files[0]))
		a, err2 := loadNRGBA(filepath.Join(dir, s.Files[1]))
		if err1 != nil || err2 != nil {
			t.Logf("%s: skipped (%v %v)", lab, err1, err2)
			continue
		}
		ow := b.Bounds().Dx() // reference boxes are in image-1 coordinates
		al := RegisterPairOpts(b, a, cfg["reg"] > 0, cfg["local"] > 0)
		ab, aa := al.Before, al.After
		opts.Valid = al.Valid
		mask := ComputeDiffV2(ab, aa, opts).Mask
		scale := float64(ab.Bounds().Dx()) / float64(ow)

		det := Regions(mask)
		hit := 0
		for _, rb := range s.Boxes {
			ref := image.Rect(int(float64(rb.X)*scale), int(float64(rb.Y)*scale),
				int(float64(rb.X+rb.W)*scale), int(float64(rb.Y+rb.H)*scale))
			for _, d := range det {
				if iou(ref, d.Box) >= 0.1 {
					hit++
					break
				}
			}
		}
		matched := 0
		for _, d := range det {
			for _, rb := range s.Boxes {
				ref := image.Rect(int(float64(rb.X)*scale), int(float64(rb.Y)*scale),
					int(float64(rb.X+rb.W)*scale), int(float64(rb.Y+rb.H)*scale))
				if iou(ref, d.Box) >= 0.1 {
					matched++
					break
				}
			}
		}
		t.Logf("%-6s ref=%2d recalled=%2d detected=%3d matching=%3d", lab, len(s.Boxes), hit, len(det), matched)
		totRef, totHit, totDet, totMatched = totRef+len(s.Boxes), totHit+hit, totDet+len(det), totMatched+matched
	}
	t.Logf("TOTAL  recall=%d/%d precision=%d/%d", totHit, totRef, totMatched, totDet)
}

func iou(a, b image.Rectangle) float64 {
	i := a.Intersect(b)
	if i.Empty() {
		return 0
	}
	ia := float64(i.Dx() * i.Dy())
	return ia / (float64(a.Dx()*a.Dy()) + float64(b.Dx()*b.Dy()) - ia)
}

func loadNRGBA(path string) (*image.NRGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return ToNRGBA(img), nil
}

// evalOpts reads "key=val,key=val" over the UI's default settings (lib/settings.ts):
// reg (auto-register), local, match (intensity match), color (Lab a/b weight ×10),
// shift (px tolerance), athr (adaptive threshold), thr, border (‰), open, close,
// minreg, blur (σ×10), light (local lighting window, ‰ of the diagonal).
func evalOpts(s string) (map[string]int, DiffOptions) {
	cfg := map[string]int{"reg": 1, "match": 1, "color": 10, "thr": 25, "border": 10, "open": 7, "close": 5, "minreg": 50, "blur": 20}
	for _, kv := range strings.Split(s, ",") {
		if k, v, ok := strings.Cut(kv, "="); ok {
			cfg[k], _ = strconv.Atoi(v)
		}
	}
	return cfg, DiffOptions{
		Threshold: uint8(cfg["thr"]), AutoThreshold: cfg["athr"] > 0, MorphSize: cfg["open"], CloseSize: cfg["close"],
		MinRegion: cfg["minreg"], PreBlurSigma: float64(cfg["blur"]) / 10, NormalizeLuma: true, MatchIntensity: cfg["match"] > 0,
		ColorWeight: float64(cfg["color"]) / 10, ShiftTol: cfg["shift"], BorderPct: float64(cfg["border"]) / 1000,
		LocalLight: float64(cfg["light"]) / 1000,
	}
}
