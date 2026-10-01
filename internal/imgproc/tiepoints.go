package imgproc

import (
	"image"
	"math"
	"sort"
)

// selectTiePoints chooses up to n tie points for the manual-alignment dialog.
//
// Good tie points are spread out (a transform is only constrained where it
// has points: clustered points leave the far side of the image free to
// drift), sit near the edges and corners of the frame (walls, ceilings and
// door frames rarely change; the middle of a scene is where objects get
// moved), and look the same in both images once aligned. So:
//
//   - Candidates are the RANSAC inliers of the full fit.
//   - Each is scored by post-alignment patch agreement (NCC between the
//     before patch and the after patch it maps to), fit residual, corner
//     strength and line support. Points on anything that changed disagree and
//     score low. Line support favours corners where long straight edges meet
//     (cabinet and door frames, wall/ceiling junctions) over the short, curved
//     edges of movable clutter such as chairs, bottles and bins.
//   - One point is taken per border cell of a 3×3 grid (the 8 cells around
//     the centre), favouring points further from the image centre; empty
//     cells are backfilled by farthest-point sampling.
//   - The After position of each point is computed from the full fit rather
//     than the matched feature, so the n points reproduce the accurate
//     all-inlier transform exactly instead of adding their own match error.
func selectTiePoints(est *autoEstimate, n int) []AutoPointPair {
	w, h := est.bGray.Bounds().Dx(), est.bGray.Bounds().Dy()
	cx, cy := float64(w)/2, float64(h)/2
	maxR := math.Hypot(cx, cy)

	type cand struct {
		p       Point
		quality float64 // 0..1
		out     float64 // distance from centre, 0..1
		cell    int
	}
	// Corner strength as a rank (Harris responses span orders of magnitude).
	strengths := make([]float64, 0, len(est.inliers))
	for _, i := range est.inliers {
		strengths = append(strengths, est.bFeatures[est.matches[i].SrcIndex].Score)
	}
	sorted := append([]float64(nil), strengths...)
	sort.Float64s(sorted)
	rank := func(v float64) float64 {
		return float64(sort.SearchFloat64s(sorted, v)+1) / float64(len(sorted))
	}

	var cands []cand
	for k, i := range est.inliers {
		m := est.matches[i]
		p := est.bFeatures[m.SrcIndex].Point
		agree := patchAgreement(est.bGray, est.aGray, est.hWork, p, 12)
		if agree < 0.5 {
			continue // content around the point changed or is ambiguous
		}
		resid := reprojectionError(est.hWork, p, est.aFeatures[m.DstIndex].Point)
		line := lineSupport(est.bGray, p, 40)
		q := agree * agree * (1 / (1 + resid)) * (0.5 + 0.5*rank(strengths[k])) * (0.25 + 0.75*line)
		gx, gy := min(int(3*p.X/float64(w)), 2), min(int(3*p.Y/float64(h)), 2)
		cands = append(cands, cand{p: p, quality: q, out: math.Hypot(p.X-cx, p.Y-cy) / maxR, cell: gy*3 + gx})
	}
	if len(cands) == 0 {
		return nil
	}

	minSep := 0.12 * math.Hypot(float64(w), float64(h))
	var chosen []cand
	farEnough := func(c cand, sep float64) bool {
		for _, o := range chosen {
			if pointDistance(o.p, c.p) < sep {
				return false
			}
		}
		return true
	}
	// Border cells in ring order, corners first (corners constrain perspective best).
	for _, cell := range []int{0, 2, 8, 6, 1, 5, 7, 3} {
		if len(chosen) >= n {
			break
		}
		best, bestScore := -1, 0.0
		for i, c := range cands {
			if c.cell != cell || !farEnough(c, minSep) {
				continue
			}
			if s := c.quality * (0.3 + 0.7*c.out); s > bestScore {
				best, bestScore = i, s
			}
		}
		if best >= 0 {
			chosen = append(chosen, cands[best])
		}
	}
	// Backfill (e.g. a featureless ceiling cell) by farthest-point sampling,
	// relaxing the separation if needed.
	for sep := minSep; len(chosen) < n && sep > 1; sep /= 2 {
		for len(chosen) < n {
			best, bestScore := -1, 0.0
			for i, c := range cands {
				if !farEnough(c, sep) {
					continue
				}
				d := maxR
				for _, o := range chosen {
					d = math.Min(d, pointDistance(o.p, c.p))
				}
				if s := c.quality * d; s > bestScore {
					best, bestScore = i, s
				}
			}
			if best < 0 {
				break
			}
			chosen = append(chosen, cands[best])
		}
	}

	pairs := make([]AutoPointPair, 0, len(chosen))
	for _, c := range chosen {
		src := Point{X: c.p.X * est.bScaleX, Y: c.p.Y * est.bScaleY}
		dx, dy := applyHomography(est.h, src.X, src.Y)
		pairs = append(pairs, AutoPointPair{Src: src, Dst: Point{X: dx, Y: dy}, Score: clampUnit(c.quality)})
	}
	return pairs
}

// patchAgreement is the normalised cross-correlation between the before patch
// around p and the after patch that h maps it to (each pixel mapped through h,
// so rotation/scale/perspective are accounted for). 1 = identical structure.
func patchAgreement(before, after *image.Gray, h [9]float64, p Point, r int) float64 {
	aw, ah := after.Bounds().Dx(), after.Bounds().Dy()
	bw, bh := before.Bounds().Dx(), before.Bounds().Dy()
	var sa, sb, saa, sbb, sab, n float64
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			bx, by := int(p.X)+dx, int(p.Y)+dy
			if bx < 0 || by < 0 || bx >= bw || by >= bh {
				continue
			}
			ax, ay := applyHomography(h, float64(bx), float64(by))
			if ax < 0 || ay < 0 || ax > float64(aw-1) || ay > float64(ah-1) {
				continue
			}
			vb := float64(before.Pix[by*before.Stride+bx])
			va := grayBilinear(after, ax, ay)
			sa += va
			sb += vb
			saa += va * va
			sbb += vb * vb
			sab += va * vb
			n++
		}
	}
	if n < float64((2*r+1)*(2*r+1))/2 {
		return 0
	}
	cov := sab/n - (sa/n)*(sb/n)
	va, vb := saa/n-(sa/n)*(sa/n), sbb/n-(sb/n)*(sb/n)
	if va < 1 || vb < 1 {
		return 0
	}
	return cov / math.Sqrt(va*vb)
}

func grayBilinear(g *image.Gray, x, y float64) float64 {
	x0, y0 := int(x), int(y)
	x1, y1 := min(x0+1, g.Bounds().Dx()-1), min(y0+1, g.Bounds().Dy()-1)
	fx, fy := x-float64(x0), y-float64(y0)
	at := func(xx, yy int) float64 { return float64(g.Pix[yy*g.Stride+xx]) }
	top := at(x0, y0)*(1-fx) + at(x1, y0)*fx
	bot := at(x0, y1)*(1-fx) + at(x1, y1)*fx
	return top*(1-fy) + bot*fy
}

// lineSupport measures how much the corner at p is formed by long straight
// edges: for the two dominant edge orientations in the neighbourhood, it walks
// the line through p (±r px) and counts samples that are strong edges with a
// matching orientation. Returns 0..1 (1 = two unbroken straight edges).
func lineSupport(g *image.Gray, p Point, r int) float64 {
	w, h := g.Bounds().Dx(), g.Bounds().Dy()
	at := func(x, y int) float64 { return float64(g.Pix[y*g.Stride+x]) }
	grad := func(x, y int) (mag, ang float64, ok bool) {
		if x < 1 || y < 1 || x >= w-1 || y >= h-1 {
			return 0, 0, false
		}
		gx := at(x+1, y-1) + 2*at(x+1, y) + at(x+1, y+1) - at(x-1, y-1) - 2*at(x-1, y) - at(x-1, y+1)
		gy := at(x-1, y+1) + 2*at(x, y+1) + at(x+1, y+1) - at(x-1, y-1) - 2*at(x, y-1) - at(x+1, y-1)
		a := math.Atan2(gy, gx)
		if a < 0 {
			a += math.Pi
		}
		return math.Hypot(gx, gy), a, true
	}

	// Magnitude-weighted orientation histogram (gradient direction mod π).
	const bins = 18
	var hist [bins]float64
	var magSum float64
	cnt := 0
	px, py := int(p.X), int(p.Y)
	for y := py - r/2; y <= py+r/2; y++ {
		for x := px - r/2; x <= px+r/2; x++ {
			if m, a, ok := grad(x, y); ok {
				hist[min(int(a/math.Pi*bins), bins-1)] += m
				magSum += m
				cnt++
			}
		}
	}
	if cnt == 0 || magSum == 0 {
		return 0
	}
	thr := math.Max(2*magSum/float64(cnt), 40)
	first := 0
	for i := range hist {
		if hist[i] > hist[first] {
			first = i
		}
	}
	second := -1
	for i := range hist {
		d := abs(float64(i - first))
		if math.Min(d, bins-d) < 3 { // at least 30° apart
			continue
		}
		if second < 0 || hist[i] > hist[second] {
			second = i
		}
	}

	support := func(bin int) float64 {
		if bin < 0 {
			return 0
		}
		ga := (float64(bin) + 0.5) / bins * math.Pi // gradient direction
		ex, ey := -math.Sin(ga), math.Cos(ga)       // edge runs perpendicular to it
		nx, ny := math.Cos(ga), math.Sin(ga)
		hits := 0
		for t := -r; t <= r; t++ {
			found := false
			for off := -1; off <= 1 && !found; off++ {
				x := int(math.Round(p.X + float64(t)*ex + float64(off)*nx))
				y := int(math.Round(p.Y + float64(t)*ey + float64(off)*ny))
				if m, a, ok := grad(x, y); ok && m >= thr {
					d := math.Abs(a - ga)
					if math.Min(d, math.Pi-d) < math.Pi/12 {
						found = true
					}
				}
			}
			if found {
				hits++
			}
		}
		// A corner is the END of its edges, so only one half of each line
		// need be present: normalise by r, not 2r.
		return math.Min(1, float64(hits)/float64(r))
	}
	return (support(first) + support(second)) / 2
}
