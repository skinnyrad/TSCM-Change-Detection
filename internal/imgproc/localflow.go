package imgproc

import (
	"image"
	"math"
	"sort"
)

// RefineLocal removes residual non-rigid misalignment (parallax, lens
// distortion, homography error) by block-matching blurred grayscale images and
// warping before with the smoothed displacement field. Matching runs on heavily
// blurred data with a bias towards zero displacement and only searches a few
// pixels, so real objects (which move by more, or change appearance) are not
// "explained away" — the same reasoning as the blurred-flow step in the x-ray
// reference pipeline.
func RefineLocal(before, after *image.NRGBA, valid *image.Gray) (*image.NRGBA, *image.Gray) {
	const (
		scale  = 2
		block  = 24 // block edge at half resolution
		search = 4  // ± half-resolution pixels
		bias   = 0.35
	)
	bg, ag := prepGray(before, scale), prepGray(after, scale)
	w, h := bg.Bounds().Dx(), bg.Bounds().Dy()
	gw, gh := w/block, h/block
	if gw < 2 || gh < 2 {
		return before, valid
	}
	dxs, dys := make([]float64, gw*gh), make([]float64, gw*gh)
	for gy := 0; gy < gh; gy++ {
		for gx := 0; gx < gw; gx++ {
			x0, y0 := gx*block, gy*block
			// Skip flat blocks: matching them only fits noise.
			var mean, sq float64
			for y := y0; y < y0+block; y += 2 {
				for x := x0; x < x0+block; x += 2 {
					v := float64(ag.Pix[y*ag.Stride+x])
					mean += v
					sq += v * v
				}
			}
			n := float64(block * block / 4)
			if sq/n-(mean/n)*(mean/n) < 9 {
				continue
			}
			best, bdx, bdy := math.MaxFloat64, 0, 0
			for dy := -search; dy <= search; dy++ {
				for dx := -search; dx <= search; dx++ {
					var sad float64
					for y := y0; y < y0+block; y += 2 {
						sy := min(max(y-dy, 0), h-1)
						for x := x0; x < x0+block; x += 2 {
							sx := min(max(x-dx, 0), w-1)
							sad += math.Abs(float64(ag.Pix[y*ag.Stride+x]) - float64(bg.Pix[sy*bg.Stride+sx]))
						}
					}
					sad = sad/n + bias*math.Hypot(float64(dx), float64(dy))
					if sad < best {
						best, bdx, bdy = sad, dx, dy
					}
				}
			}
			dxs[gy*gw+gx], dys[gy*gw+gx] = float64(bdx*scale), float64(bdy*scale)
		}
	}
	dxs, dys = medianField(dxs, gw, gh), medianField(dys, gw, gh)

	bw, bh := before.Bounds().Dx(), before.Bounds().Dy()
	out := image.NewNRGBA(image.Rect(0, 0, bw, bh))
	nv := image.NewGray(image.Rect(0, 0, bw, bh))
	cell := float64(block * scale)
	for y := 0; y < bh; y++ {
		for x := 0; x < bw; x++ {
			fx, fy := (float64(x)/cell)-0.5, (float64(y)/cell)-0.5
			dx, dy := sampleField(dxs, gw, gh, fx, fy), sampleField(dys, gw, gh, fx, fy)
			sx, sy := float64(x)-dx, float64(y)-dy
			if sx < 0 || sy < 0 || sx > float64(bw-1) || sy > float64(bh-1) {
				continue
			}
			out.SetNRGBA(x, y, bilinearSample(before, sx, sy))
			if valid == nil {
				nv.Pix[y*nv.Stride+x] = 255
			} else {
				nv.Pix[y*nv.Stride+x] = valid.Pix[int(sy+0.5)*valid.Stride+int(sx+0.5)]
			}
		}
	}
	return out, nv
}

func medianField(f []float64, gw, gh int) []float64 {
	out := make([]float64, len(f))
	var win [9]float64
	for y := 0; y < gh; y++ {
		for x := 0; x < gw; x++ {
			n := 0
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					xx, yy := x+dx, y+dy
					if xx >= 0 && yy >= 0 && xx < gw && yy < gh {
						win[n] = f[yy*gw+xx]
						n++
					}
				}
			}
			s := win[:n]
			sort.Float64s(s)
			out[y*gw+x] = s[n/2]
		}
	}
	return out
}

func sampleField(f []float64, gw, gh int, fx, fy float64) float64 {
	fx = math.Max(0, math.Min(float64(gw-1), fx))
	fy = math.Max(0, math.Min(float64(gh-1), fy))
	x0, y0 := int(fx), int(fy)
	x1, y1 := min(x0+1, gw-1), min(y0+1, gh-1)
	tx, ty := fx-float64(x0), fy-float64(y0)
	top := f[y0*gw+x0]*(1-tx) + f[y0*gw+x1]*tx
	bot := f[y1*gw+x0]*(1-tx) + f[y1*gw+x1]*tx
	return top*(1-ty) + bot*ty
}
