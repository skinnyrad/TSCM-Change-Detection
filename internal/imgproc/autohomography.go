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
	autoHomographyMaxPoints = 160
	autoHomographyMinPoints = 4
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
	SrcIndex  int
	DstIndex  int
	Score     float64
	Distance  float64
	ReprojErr float64
}

// AutoDetectHomography finds 4-8 correspondence pairs that can seed manual warp review.
func AutoDetectHomography(before, after *image.NRGBA) (*AutoHomographyResult, error) {
	if before == nil || after == nil {
		return nil, fmt.Errorf("images not ready")
	}

	bScaled, bScaleX, bScaleY := downsampleWithScale(before, autoHomographyMaxDim)
	aScaled, aScaleX, aScaleY := downsampleWithScale(after, autoHomographyMaxDim)

	bGray := toGray(bScaled)
	aGray := toGray(aScaled)

	bFeatures := detectAutoFeatures(bGray, autoHomographyMaxPoints)
	aFeatures := detectAutoFeatures(aGray, autoHomographyMaxPoints)
	if len(bFeatures) < autoHomographyMinPoints || len(aFeatures) < autoHomographyMinPoints {
		return nil, fmt.Errorf("could not find enough distinctive points for auto alignment")
	}

	matches := matchAutoFeatures(bFeatures, aFeatures)
	if len(matches) < autoHomographyMinPoints {
		return nil, fmt.Errorf("could not find enough matching points for auto alignment")
	}

	inliers, avgErr, err := ransacAutoHomography(bFeatures, aFeatures, matches)
	if err != nil {
		return nil, err
	}
	if len(inliers) < autoHomographyMinPoints {
		return nil, fmt.Errorf("auto alignment confidence too low")
	}

	selected := selectSpreadMatches(bFeatures, matches, inliers, 8)
	if len(selected) < autoHomographyMinPoints {
		return nil, fmt.Errorf("auto alignment confidence too low")
	}

	pairs := make([]AutoPointPair, 0, len(selected))
	for _, idx := range selected {
		match := matches[idx]
		src := bFeatures[match.SrcIndex].Point
		dst := aFeatures[match.DstIndex].Point
		pairs = append(pairs, AutoPointPair{
			Src: Point{X: src.X * bScaleX, Y: src.Y * bScaleY},
			Dst: Point{X: dst.X * aScaleX, Y: dst.Y * aScaleY},
			Score: clampUnit((match.Score + match.ReprojErrScore()) / 2),
		})
	}

	confidence := clampUnit((float64(len(inliers))/float64(len(matches))*0.65) + (1/(1+avgErr))*0.35)
	return &AutoHomographyResult{
		Pairs:       pairs,
		Confidence:  confidence,
		MatchCount:  len(matches),
		InlierCount: len(inliers),
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

	maxResp := 0.0
	for _, resp := range responses {
		if resp > maxResp {
			maxResp = resp
		}
	}
	if maxResp <= 0 {
		return nil
	}
	threshold := maxResp * 0.08

	type candidate struct {
		x, y  int
		score float64
	}
	candidates := make([]candidate, 0, limit*4)
	for y := margin; y < h-margin; y++ {
		for x := margin; x < w-margin; x++ {
			resp := responses[y*w+x]
			if resp < threshold || !isLocalPeak(responses, w, h, x, y, 2) {
				continue
			}
			descriptor := makePatchDescriptor(gray, x, y, patchRadius)
			if descriptor == nil {
				continue
			}
			candidates = append(candidates, candidate{x: x, y: y, score: resp})
		}
	}

	sort.Slice(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })
	selected := make([]autoFeature, 0, minInt(limit, len(candidates)))
	minDist := math.Max(8, float64(minInt(w, h))/18)
	for _, candidate := range candidates {
		point := Point{X: float64(candidate.x), Y: float64(candidate.y)}
		if tooCloseToExisting(selected, point, minDist) {
			continue
		}
		descriptor := makePatchDescriptor(gray, candidate.x, candidate.y, patchRadius)
		if descriptor == nil {
			continue
		}
		selected = append(selected, autoFeature{Point: point, Score: candidate.score, Descriptor: descriptor})
		if len(selected) == limit {
			break
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
		if bestIdx == -1 || secondIdx == -1 || bestDist >= secondDist*0.92 {
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
	if len(matches) > 48 {
		matches = matches[:48]
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

func ransacAutoHomography(before, after []autoFeature, matches []autoMatch) ([]int, float64, error) {
	if len(matches) < autoHomographyMinPoints {
		return nil, 0, fmt.Errorf("could not find enough matching points for auto alignment")
	}

	rng := rand.New(rand.NewSource(7))
	bestInliers := make([]int, 0)
	bestAvgErr := math.MaxFloat64
	iterations := 320
	if len(matches) < 8 {
		iterations = 120
	}

	for i := 0; i < iterations; i++ {
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

		inliers, avgErr := scoreHomography(h, before, after, matches, 5.0)
		if len(inliers) > len(bestInliers) || (len(inliers) == len(bestInliers) && avgErr < bestAvgErr) {
			bestInliers = inliers
			bestAvgErr = avgErr
		}
	}

	if len(bestInliers) < autoHomographyMinPoints {
		return nil, 0, fmt.Errorf("auto alignment confidence too low")
	}

	srcPts := make([]Point, 0, len(bestInliers))
	dstPts := make([]Point, 0, len(bestInliers))
	for _, idx := range bestInliers {
		match := matches[idx]
		srcPts = append(srcPts, before[match.SrcIndex].Point)
		dstPts = append(dstPts, after[match.DstIndex].Point)
	}
	h, err := computeHomography(srcPts, dstPts)
	if err != nil {
		return nil, 0, fmt.Errorf("auto alignment confidence too low")
	}
	finalInliers, avgErr := scoreHomography(h, before, after, matches, 4.5)
	if len(finalInliers) < autoHomographyMinPoints {
		return nil, 0, fmt.Errorf("auto alignment confidence too low")
	}
	for _, idx := range finalInliers {
		match := &matches[idx]
		match.ReprojErr = reprojectionError(h, before[match.SrcIndex].Point, after[match.DstIndex].Point)
	}
	return finalInliers, avgErr, nil
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

func selectSpreadMatches(features []autoFeature, matches []autoMatch, inliers []int, limit int) []int {
	type rankedMatch struct {
		idx   int
		score float64
	}
	ranked := make([]rankedMatch, 0, len(inliers))
	for _, idx := range inliers {
		match := matches[idx]
		ranked = append(ranked, rankedMatch{idx: idx, score: clampUnit((match.Score + match.ReprojErrScore()) / 2)})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].idx < ranked[j].idx
		}
		return ranked[i].score > ranked[j].score
	})

	selected := make([]int, 0, minInt(limit, len(ranked)))
	minDist := 14.0
	for pass := 0; pass < 4 && len(selected) < autoHomographyMinPoints; pass++ {
		selected = selected[:0]
		currentMinDist := minDist / math.Pow(1.35, float64(pass))
		for _, candidate := range ranked {
			match := matches[candidate.idx]
			point := features[match.SrcIndex].Point
			if tooCloseMatch(features, matches, selected, point, currentMinDist) {
				continue
			}
			selected = append(selected, candidate.idx)
			if len(selected) == limit {
				return selected
			}
		}
	}
	if len(selected) == 0 {
		for _, candidate := range ranked {
			selected = append(selected, candidate.idx)
			if len(selected) == limit {
				break
			}
		}
	}
	return selected
}

func tooCloseMatch(features []autoFeature, matches []autoMatch, selected []int, point Point, minDist float64) bool {
	for _, idx := range selected {
		other := features[matches[idx].SrcIndex].Point
		if pointDistance(point, other) < minDist {
			return true
		}
	}
	return false
}

func (m autoMatch) ReprojErrScore() float64 {
	if m.ReprojErr == 0 {
		return 1
	}
	return 1 / (1 + m.ReprojErr)
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