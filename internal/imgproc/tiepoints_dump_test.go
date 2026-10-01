package imgproc

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

// TestDumpTiePoints draws the auto-detected tie points on both images of a
// pair (TIE_A, TIE_B) into tmp/ for visual inspection. Skipped unless set.
func TestDumpTiePoints(t *testing.T) {
	pa, pb, out := os.Getenv("TIE_A"), os.Getenv("TIE_B"), os.Getenv("TIE_OUT")
	if pa == "" || pb == "" || out == "" {
		t.Skip()
	}
	b, _ := loadNRGBA(pa)
	a, _ := loadNRGBA(pb)
	res, err := AutoDetectHomography(b, a)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("inliers=%d matches=%d conf=%.2f", res.InlierCount, res.MatchCount, res.Confidence)
	var s []Point
	for _, p := range res.Pairs {
		s = append(s, p.Src)
		t.Logf("src=(%.0f,%.0f) dst=(%.0f,%.0f) score=%.2f", p.Src.X, p.Src.Y, p.Dst.X, p.Dst.Y, p.Score)
	}
	t.Logf("spread=%.2f", spreadScore(s, b.Bounds().Dx(), b.Bounds().Dy()))
	img := DownsampleNRGBA(b, 1200)
	sc := float64(img.Bounds().Dx()) / float64(b.Bounds().Dx())
	for i, p := range res.Pairs {
		c := []color.NRGBA{{244, 67, 54, 255}, {76, 175, 80, 255}, {33, 150, 243, 255}, {255, 152, 0, 255}, {156, 39, 176, 255}, {0, 188, 212, 255}, {205, 220, 57, 255}, {255, 87, 34, 255}}[i%8]
		disc(img, int(p.Src.X*sc), int(p.Src.Y*sc), 12, c)
	}
	f, _ := os.Create(out)
	png.Encode(f, img)
	f.Close()
}

func disc(img *image.NRGBA, cx, cy, r int, c color.NRGBA) {
	for y := -r; y <= r; y++ {
		for x := -r; x <= r; x++ {
			if x*x+y*y <= r*r && image.Pt(cx+x, cy+y).In(img.Bounds()) {
				if x*x+y*y >= (r-3)*(r-3) {
					img.SetNRGBA(cx+x, cy+y, color.NRGBA{255, 255, 255, 255})
				} else {
					img.SetNRGBA(cx+x, cy+y, c)
				}
			}
		}
	}
}

// TestDumpFeatures draws every detected feature (grey), match (yellow) and
// inlier (green) of the before image. Skipped unless TIE_* are set.
func TestDumpFeatures(t *testing.T) {
	pa, pb, out := os.Getenv("TIE_A"), os.Getenv("TIE_B"), os.Getenv("TIE_FEAT")
	if pa == "" || pb == "" || out == "" {
		t.Skip()
	}
	b, _ := loadNRGBA(pa)
	a, _ := loadNRGBA(pb)
	est, err := estimateAuto(b, a)
	if err != nil {
		t.Fatal(err)
	}
	img := DownsampleNRGBA(b, 1200)
	sc := float64(img.Bounds().Dx()) / float64(b.Bounds().Dx())
	for _, f := range est.bFeatures {
		disc(img, int(f.X*est.bScaleX*sc), int(f.Y*est.bScaleY*sc), 4, color.NRGBA{150, 150, 150, 255})
	}
	for _, m := range est.matches {
		f := est.bFeatures[m.SrcIndex]
		disc(img, int(f.X*est.bScaleX*sc), int(f.Y*est.bScaleY*sc), 6, color.NRGBA{255, 220, 0, 255})
	}
	for _, i := range est.inliers {
		f := est.bFeatures[est.matches[i].SrcIndex]
		disc(img, int(f.X*est.bScaleX*sc), int(f.Y*est.bScaleY*sc), 7, color.NRGBA{0, 220, 0, 255})
	}
	t.Logf("features=%d matches=%d inliers=%d", len(est.bFeatures), len(est.matches), len(est.inliers))
	f, _ := os.Create(out)
	png.Encode(f, img)
	f.Close()
}
