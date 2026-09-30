package imgproc

import (
	"image"
	"sort"
)

// Region is one 8-connected changed area in a binary mask.
type Region struct {
	Box  image.Rectangle
	Area int // number of mask pixels

	Mass     float64 // summed diff magnitude inside the region
	MeanHeat float64 // Mass / Area
	Score    float64 // Mass relative to the strongest region (1 = strongest)
}

// Regions labels the 8-connected components of mask (non-zero = changed) and
// returns each with its bounding box and pixel area, in scan order.
func Regions(mask *image.Gray) []Region {
	_, regions := components(mask)
	return regions
}

// components labels 8-connected components. labels is row-major (w*h) with 0 for
// background and 1..n for regions; regions[i] describes label i+1.
func components(mask *image.Gray) (labels []int32, regions []Region) {
	b := mask.Bounds()
	w, h := b.Dx(), b.Dy()
	labels = make([]int32, w*h)
	var stack []int
	for start := 0; start < w*h; start++ {
		if labels[start] != 0 || mask.Pix[(start/w)*mask.Stride+start%w] == 0 {
			continue
		}
		id := int32(len(regions) + 1)
		r := Region{Box: image.Rect(start%w, start/w, start%w+1, start/w+1)}
		labels[start] = id
		stack = append(stack[:0], start)
		for len(stack) > 0 {
			p := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			x, y := p%w, p/w
			r.Area++
			r.Box.Min.X, r.Box.Min.Y = min(r.Box.Min.X, x), min(r.Box.Min.Y, y)
			r.Box.Max.X, r.Box.Max.Y = max(r.Box.Max.X, x+1), max(r.Box.Max.Y, y+1)
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nx, ny := x+dx, y+dy
					if nx < 0 || ny < 0 || nx >= w || ny >= h {
						continue
					}
					n := ny*w + nx
					if labels[n] == 0 && mask.Pix[ny*mask.Stride+nx] != 0 {
						labels[n] = id
						stack = append(stack, n)
					}
				}
			}
		}
		regions = append(regions, r)
	}
	return labels, regions
}

// RankRegions returns the mask's regions with heat statistics from diff,
// sorted by total heat mass (strongest first). Score is each region's mass as
// a fraction of the strongest region's mass.
func RankRegions(diff, mask *image.Gray) []Region {
	w := mask.Bounds().Dx()
	labels, regions := components(mask)
	for y := 0; y < mask.Bounds().Dy(); y++ {
		for x := 0; x < w; x++ {
			if l := labels[y*w+x]; l > 0 {
				regions[l-1].Mass += float64(diff.Pix[y*diff.Stride+x])
			}
		}
	}
	sort.SliceStable(regions, func(i, j int) bool { return regions[i].Mass > regions[j].Mass })
	if len(regions) > 0 && regions[0].Mass > 0 {
		for i := range regions {
			regions[i].Score = regions[i].Mass / regions[0].Mass
			regions[i].MeanHeat = regions[i].Mass / float64(max(regions[i].Area, 1))
		}
	}
	return regions
}
