package imgproc

import (
	"image"
	"math"
)

// Stats holds the change statistics for a threshold mask.
type Stats struct {
	Pct       float64 `json:"pct"`
	ChangedPx int     `json:"changed_px"`
	Regions   int     `json:"regions"`
}

// ChangeStats computes change statistics from a binary threshold mask.
func ChangeStats(mask *image.Gray) Stats {
	b := mask.Bounds()
	w, h := b.Dx(), b.Dy()
	changed := 0
	for y := 0; y < h; y++ {
		for _, v := range mask.Pix[y*mask.Stride : y*mask.Stride+w] {
			if v > 0 {
				changed++
			}
		}
	}
	pct := 0.0
	if w*h > 0 {
		pct = math.Round(float64(changed)/float64(w*h)*100*100) / 100
	}
	return Stats{Pct: pct, ChangedPx: changed, Regions: len(Regions(mask))}
}
