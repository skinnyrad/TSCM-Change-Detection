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
// Matches Python's change_stats() function.
func ChangeStats(mask *image.Gray) Stats {
	b := mask.Bounds()
	w, h := b.Dx(), b.Dy()
	total := w * h
	changed := 0

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if mask.GrayAt(x, y).Y > 0 {
				changed++
			}
		}
	}

	pct := 0.0
	if total > 0 {
		pct = math.Round(float64(changed)/float64(total)*100*100) / 100
	}

	return Stats{
		Pct:       pct,
		ChangedPx: changed,
		Regions:   countRegions(mask),
	}
}

// countRegions counts distinct connected components in a binary mask using BFS.
func countRegions(mask *image.Gray) int {
	b := mask.Bounds()
	w, h := b.Dx(), b.Dy()
	visited := make([]bool, w*h)
	count := 0

	dirs := [4][2]int{{0, 1}, {0, -1}, {1, 0}, {-1, 0}}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			if visited[idx] || mask.GrayAt(x, y).Y == 0 {
				continue
			}
			count++
			queue := [][2]int{{x, y}}
			visited[idx] = true
			for len(queue) > 0 {
				p := queue[0]
				queue = queue[1:]
				for _, d := range dirs {
					nx, ny := p[0]+d[0], p[1]+d[1]
					if nx < 0 || nx >= w || ny < 0 || ny >= h {
						continue
					}
					nidx := ny*w + nx
					if visited[nidx] || mask.GrayAt(nx, ny).Y == 0 {
						continue
					}
					visited[nidx] = true
					queue = append(queue, [2]int{nx, ny})
				}
			}
		}
	}
	return count
}
