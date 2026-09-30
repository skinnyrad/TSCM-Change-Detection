package imgproc

import (
	"fmt"
	"image"
	"math"
	"math/rand"
	"sort"
)

const (
	autoHomographyMaxDim    = 960
	autoHomographyPatchSize = 11
	autoHomographyMaxPoints = 500
	autoHomographyMinPoints = 4
	autoMaxMatches          = 300
	autoRatio               = 0.85
	autoRansacIters         = 1500
	autoRansacThresh        = 3.0
	autoRefitThresh         = 2.5
	autoRansacSeed          = 7
)

// AutoPointPair is a candidate correspondence between the raw before and after images.
type AutoPointPair struct {
	Src   Point
	Dst   Point
	Score float64
}

// AutoHomographyResult contains the best auto-detected correspondences plus summary metadata.
type AutoHomographyResult struct {
	Pairs       []AutoPointPair
	Confidence  float64
	MatchCount  int
	InlierCount int
}

type autoFeature struct {
	Point
	Score      float64
	Descriptor []float64
}

type autoMatch struct {
	SrcIndex int
	DstIndex int
	Score    float64
	Distance float64
}

// autoEstimate is the full result of feature-based alignment, in the
// coordinates of the (possibly downsampled) working images.
type autoEstimate struct {
	bGray, aGray         *image.Gray // working-resolution grayscale
	hWork                [9]float64  // before → after, working coordinates
	bFeatures, aFeatures []autoFeature
	matches              []autoMatch
	inliers              []int
	avgErr               float64
	h                    [9]float64 // before → after, full-resolution coordinates
	bScaleX, bScaleY     float64
	aScaleX, aScaleY     float64
	confidence           float64
}

// estimateAuto detects features, matches them, and fits a homography with
// RANSAC followed by a refit on all inliers.
func estimateAuto(before, after *image.NRGBA) (*autoEstimate, error) {
	if before == nil || after == nil {
		return nil, fmt.Errorf("images not ready")
	}

	bScaled, bScaleX, bScaleY := downsampleWithScale(before, autoHomographyMaxDim)
	aScaled, aScaleX, aScaleY := downsampleWithScale(after, autoHomographyMaxDim)

	bGray, aGray := toGray(bScaled), toGray(aScaled)
	bFeatures := detectAutoFeatures(bGray, autoHomographyMaxPoints)
	aFeatures := detectAutoFeatures(aGray, autoHomographyMaxPoints)
	if len(bFeatures) < autoHomographyMinPoints || len(aFeatures) < autoHomographyMinPoints {
		return nil, fmt.Errorf("could not find enough distinctive points for auto alignment")
	}

	matches := matchAutoFeatures(bFeatures, aFeatures)
	if len(matches) < autoHomographyMinPoints {
		return nil, fmt.Errorf("could not find enough matching points for auto alignment")
	}

	inliers, avgErr, h, err := ransacAutoHomography(bFeatures, aFeatures, matches)
	if err != nil {
		return nil, err
	}

	// Convert H from working-image coordinates to full-resolution coordinates:
	// Hfull = Sa · H · Sb⁻¹.
	sa := mat3{aScaleX, 0, 0, 0, aScaleY, 0, 0, 0, 1}
	sbInv := mat3{1 / bScaleX, 0, 0, 0, 1 / bScaleY, 0, 0, 0, 1}
	hf := mat3Mul(mat3Mul(sa, mat3(h)), sbInv)
	if math.Abs(hf[8]) > 1e-15 {
		for i := range hf {
			hf[i] /= hf[8]
		}
	}

	confidence := clampUnit((float64(len(inliers)) / float64(len(matches)) * 0.65) + (1/(1+avgErr))*0.35)
	return &autoEstimate{
		bGray: bGray, aGray: aGray, hWork: h,
		bFeatures: bFeatures, aFeatures: aFeatures, matches: matches,
		inliers: inliers, avgErr: avgErr, h: [9]float64(hf),
		bScaleX: bScaleX, bScaleY: bScaleY, aScaleX: aScaleX, aScaleY: aScaleY,
		confidence: confidence,
	}, nil
}

// AutoDetectHomography finds 4-8 correspondence pairs that can seed manual warp review.
func AutoDetectHomography(before, after *image.NRGBA) (*AutoHomographyResult, error) {
	est, err := estimateAuto(before, after)
	if err != nil {
		return nil, err
	}
	pairs := selectTiePoints(est, 8)
	if len(pairs) < autoHomographyMinPoints {
		return nil, fmt.Errorf("auto alignment confidence too low")
	}
	return &AutoHomographyResult{
		Pairs:       pairs,
		Confidence:  est.confidence,
		MatchCount:  len(est.matches),
		InlierCount: len(est.inliers),
	}, nil
}

func downsampleWithScale(img *image.NRGBA, maxDim int) (*image.NRGBA, float64, float64) {
	b := img.Bounds()
	if b.Dx() <= maxDim && b.Dy() <= maxDim {
		return img, 1, 1
	}
	down := DownsampleNRGBA(img, maxDim)
	db := down.Bounds()
	return down, float64(b.Dx()) / float64(db.Dx()), float64(b.Dy()) / float64(db.Dy())
}

func detectAutoFeatures(gray *image.Gray, limit int) []autoFeature {
	b := gray.Bounds()
	w, h := b.Dx(), b.Dy()
	patchRadius := autoHomographyPatchSize / 2
	margin := patchRadius + 3
	if w < margin*2+1 || h < margin*2+1 {
		return nil
	}

	gx := make([]float64, w*h)
	gy := make([]float64, w*h)
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			idx := y*w + x
			gx[idx] = sobelX(gray, x, y)
			gy[idx] = sobelY(gray, x, y)
		}
	}

	responses := make([]float64, w*h)
	const k = 0.04
	for y := margin; y < h-margin; y++ {
		for x := margin; x < w-margin; x++ {
			var sumXX, sumYY, sumXY float64
			for wy := -1; wy <= 1; wy++ {
				for wx := -1; wx <= 1; wx++ {
					idx := (y+wy)*w + (x + wx)
					ix := gx[idx]
					iy := gy[idx]
					sumXX += ix * ix
					sumYY += iy * iy
					sumXY += ix * iy
				}
			}
			det := sumXX*sumYY - sumXY*sumXY
			trace := sumXX + sumYY
			responses[y*w+x] = det - k*trace*trace
		}
	}

	return selectCornerCandidates(gray, responses, w, h, margin, patchRadius, limit)
}

// selectCornerCandidates keeps Harris peaks with a threshold relative to the
// strongest corner in each grid cell rather than in the whole image. A single
// global threshold lets high-contrast clutter (dark chairs on a white wall,
// bottles) suppress every lower-contrast but stable corner — cabinet and door
// frames, ceiling fixtures — so features, and therefore tie points, pile up on
// the busiest spot. Each cell gets an equal quota first; leftover capacity is
// filled by the strongest remaining corners anywhere.
func selectCornerCandidates(gray *image.Gray, responses []float64, w, h, margin, patchRadius, limit int) []autoFeature {
	const gridX, gridY = 6, 5
	maxResp := 0.0
	cellMax := make([]float64, gridX*gridY)
	cellOf := func(x, y int) int { return min(y*gridY/h, gridY-1)*gridX + min(x*gridX/w, gridX-1) }
	for y := margin; y < h-margin; y++ {
		for x := margin; x < w-margin; x++ {
			r := responses[y*w+x]
			maxResp = math.Max(maxResp, r)
			c := cellOf(x, y)
			cellMax[c] = math.Max(cellMax[c], r)
		}
	}
	if maxResp <= 0 {
		return nil
	}

	type candidate struct {
		x, y, cell int
		score      float64
	}
	var candidates []candidate
	for y := margin; y < h-margin; y++ {
		for x := margin; x < w-margin; x++ {
			resp := responses[y*w+x]
			c := cellOf(x, y)
			// Relative to the local maximum, with a small global floor so
			// flat cells (bare wall, sky) don't promote noise.
			if resp < math.Max(cellMax[c]*0.05, maxResp*0.0005) || !isLocalPeak(responses, w, h, x, y, 2) {
				continue
			}
			candidates = append(candidates, candidate{x: x, y: y, cell: c, score: resp})
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })

	minDist := math.Max(6, float64(minInt(w, h))/80)
	selected := make([]autoFeature, 0, limit)
	used := make([]bool, len(candidates))
	take := func(i int) bool {
		c := candidates[i]
		point := Point{X: float64(c.x), Y: float64(c.y)}
		if tooCloseToExisting(selected, point, minDist) {
			return false
		}
		descriptor := makePatchDescriptor(gray, c.x, c.y, patchRadius)
		if descriptor == nil {
			return false
		}
		selected = append(selected, autoFeature{Point: point, Score: c.score, Descriptor: descriptor})
		return true
	}
	quota := limit / (gridX * gridY)
	perCell := make([]int, gridX*gridY)
	for i, c := range candidates {
		if perCell[c.cell] < quota && take(i) {
			perCell[c.cell]++
			used[i] = true
		}
	}
	for i := range candidates {
		if len(selected) >= limit {
			break
		}
		if !used[i] {
			take(i)
		}
	}
	return selected
}

func makePatchDescriptor(gray *image.Gray, cx, cy, radius int) []float64 {
	size := radius*2 + 1
	values := make([]float64, 0, size*size)
	var sum float64
	for y := cy - radius; y <= cy+radius; y++ {
		for x := cx - radius; x <= cx+radius; x++ {
			v := float64(gray.GrayAt(x, y).Y) / 255.0
			values = append(values, v)
			sum += v
		}
	}
	mean := sum / float64(len(values))
	var variance float64
	for _, value := range values {
		delta := value - mean
		variance += delta * delta
	}
	variance /= float64(len(values))
	if variance < 1e-6 {
		return nil
	}
	std := math.Sqrt(variance)
	for i := range values {
		values[i] = (values[i] - mean) / std
	}
	return values
}

func matchAutoFeatures(before, after []autoFeature) []autoMatch {
	if len(before) == 0 || len(after) == 0 {
		return nil
	}

	reverseBest := make([]int, len(after))
	reverseDist := make([]float64, len(after))
	for i := range reverseBest {
		reverseBest[i] = -1
		reverseDist[i] = math.MaxFloat64
	}

	forward := make([]autoMatch, 0, len(before))
	for srcIdx, srcFeature := range before {
		bestIdx, secondIdx := -1, -1
		bestDist, secondDist := math.MaxFloat64, math.MaxFloat64
		for dstIdx, dstFeature := range after {
			distance := descriptorDistance(srcFeature.Descriptor, dstFeature.Descriptor)
			if distance < bestDist {
				secondDist, secondIdx = bestDist, bestIdx
				bestDist, bestIdx = distance, dstIdx
			} else if distance < secondDist {
				secondDist, secondIdx = distance, dstIdx
			}
		}
		if bestIdx == -1 || secondIdx == -1 || bestDist >= secondDist*autoRatio {
			continue
		}
		if bestDist < reverseDist[bestIdx] {
			reverseDist[bestIdx] = bestDist
			reverseBest[bestIdx] = srcIdx
		}
		forward = append(forward, autoMatch{SrcIndex: srcIdx, DstIndex: bestIdx, Distance: bestDist, Score: 1 / (1 + bestDist)})
	}

	matches := make([]autoMatch, 0, len(forward))
	for _, match := range forward {
		if reverseBest[match.DstIndex] == match.SrcIndex {
			matches = append(matches, match)
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score == matches[j].Score {
			return matches[i].Distance < matches[j].Distance
		}
		return matches[i].Score > matches[j].Score
	})
	if len(matches) > autoMaxMatches {
		matches = matches[:autoMaxMatches]
	}
	return matches
}

func descriptorDistance(a, b []float64) float64 {
	var sum float64
	for i := range a {
		delta := a[i] - b[i]
		sum += delta * delta
	}
	return sum / float64(len(a))
}

func ransacAutoHomography(before, after []autoFeature, matches []autoMatch) ([]int, float64, [9]float64, error) {
	var zero [9]float64
	if len(matches) < autoHomographyMinPoints {
		return nil, 0, zero, fmt.Errorf("could not find enough matching points for auto alignment")
	}

	rng := rand.New(rand.NewSource(autoRansacSeed))
	bestInliers := make([]int, 0)
	bestAvgErr := math.MaxFloat64

	for i := 0; i < autoRansacIters; i++ {
		sampleIdx := sampleMatchIndices(rng, len(matches), 4)
		srcPts := make([]Point, 0, 4)
		dstPts := make([]Point, 0, 4)
		for _, idx := range sampleIdx {
			match := matches[idx]
			srcPts = append(srcPts, before[match.SrcIndex].Point)
			dstPts = append(dstPts, after[match.DstIndex].Point)
		}
		h, err := computeHomography(srcPts, dstPts)
		if err != nil {
			continue
		}

		inliers, avgErr := scoreHomography(h, before, after, matches, autoRansacThresh)
		if len(inliers) > len(bestInliers) || (len(inliers) == len(bestInliers) && avgErr < bestAvgErr) {
			bestInliers = inliers
			bestAvgErr = avgErr
		}
	}

	if len(bestInliers) < autoHomographyMinPoints {
		return nil, 0, zero, fmt.Errorf("auto alignment confidence too low")
	}

	// Refit on all inliers, then re-score (twice) so the final model uses every
	// supporting correspondence rather than a 4-point sample.
	var h [9]float64
	finalInliers := bestInliers
	var avgErr float64
	for round := 0; round < 2; round++ {
		srcPts := make([]Point, 0, len(finalInliers))
		dstPts := make([]Point, 0, len(finalInliers))
		for _, idx := range finalInliers {
			match := matches[idx]
			srcPts = append(srcPts, before[match.SrcIndex].Point)
			dstPts = append(dstPts, after[match.DstIndex].Point)
		}
		var err error
		h, err = computeHomography(srcPts, dstPts)
		if err != nil {
			return nil, 0, zero, fmt.Errorf("auto alignment confidence too low")
		}
		finalInliers, avgErr = scoreHomography(h, before, after, matches, autoRefitThresh)
		if len(finalInliers) < autoHomographyMinPoints {
			return nil, 0, zero, fmt.Errorf("auto alignment confidence too low")
		}
	}
	return finalInliers, avgErr, h, nil
}

func scoreHomography(h [9]float64, before, after []autoFeature, matches []autoMatch, threshold float64) ([]int, float64) {
	inliers := make([]int, 0, len(matches))
	var errSum float64
	for idx, match := range matches {
		err := reprojectionError(h, before[match.SrcIndex].Point, after[match.DstIndex].Point)
		if err <= threshold {
			inliers = append(inliers, idx)
			errSum += err
		}
	}
	if len(inliers) == 0 {
		return nil, math.MaxFloat64
	}
	return inliers, errSum / float64(len(inliers))
}

func reprojectionError(h [9]float64, src, dst Point) float64 {
	x, y := applyHomography(h, src.X, src.Y)
	dx := x - dst.X
	dy := y - dst.Y
	return math.Sqrt(dx*dx + dy*dy)
}

func sampleMatchIndices(rng *rand.Rand, total, count int) []int {
	chosen := make(map[int]struct{}, count)
	result := make([]int, 0, count)
	for len(result) < count {
		candidate := rng.Intn(total)
		if _, exists := chosen[candidate]; exists {
			continue
		}
		chosen[candidate] = struct{}{}
		result = append(result, candidate)
	}
	return result
}

func sobelX(gray *image.Gray, x, y int) float64 {
	return -float64(gray.GrayAt(x-1, y-1).Y) + float64(gray.GrayAt(x+1, y-1).Y) -
		2*float64(gray.GrayAt(x-1, y).Y) + 2*float64(gray.GrayAt(x+1, y).Y) -
		float64(gray.GrayAt(x-1, y+1).Y) + float64(gray.GrayAt(x+1, y+1).Y)
}

func sobelY(gray *image.Gray, x, y int) float64 {
	return -float64(gray.GrayAt(x-1, y-1).Y) - 2*float64(gray.GrayAt(x, y-1).Y) - float64(gray.GrayAt(x+1, y-1).Y) +
		float64(gray.GrayAt(x-1, y+1).Y) + 2*float64(gray.GrayAt(x, y+1).Y) + float64(gray.GrayAt(x+1, y+1).Y)
}

func isLocalPeak(values []float64, w, h, x, y, radius int) bool {
	center := values[y*w+x]
	for yy := maxInt(y-radius, 0); yy <= minInt(y+radius, h-1); yy++ {
		for xx := maxInt(x-radius, 0); xx <= minInt(x+radius, w-1); xx++ {
			if xx == x && yy == y {
				continue
			}
			if values[yy*w+xx] >= center {
				return false
			}
		}
	}
	return true
}

func tooCloseToExisting(features []autoFeature, point Point, minDist float64) bool {
	for _, feature := range features {
		if pointDistance(feature.Point, point) < minDist {
			return true
		}
	}
	return false
}

func pointDistance(a, b Point) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return math.Sqrt(dx*dx + dy*dy)
}

func clampUnit(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
