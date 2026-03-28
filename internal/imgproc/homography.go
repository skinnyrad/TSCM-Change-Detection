package imgproc

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

// Point is a 2D image coordinate in pixel space.
type Point struct{ X, Y float64 }

// mat3 is a row-major 3×3 matrix.
type mat3 [9]float64

// WarpPerspective warps src so that srcPts map to dstPts using a perspective
// transform. The output image is outW×outH (the dimensions of the "after" image).
// Uses inverse mapping with bilinear interpolation. Accepts 4–8 point pairs.
func WarpPerspective(src *image.NRGBA, srcPts, dstPts []Point, outW, outH int) (*image.NRGBA, error) {
	// Inverse map: H that takes output (dst) coords back to source coords.
	hinv, err := computeHomography(dstPts, srcPts)
	if err != nil {
		return nil, err
	}

	sb := src.Bounds()
	sw, sh := float64(sb.Dx()), float64(sb.Dy())
	out := image.NewNRGBA(image.Rect(0, 0, outW, outH))

	for py := 0; py < outH; py++ {
		for px := 0; px < outW; px++ {
			sx, sy := applyHomography(hinv, float64(px), float64(py))
			if sx >= 0 && sx < sw && sy >= 0 && sy < sh {
				out.SetNRGBA(px, py, bilinearSample(src, sx, sy))
			} else {
				out.SetNRGBA(px, py, color.NRGBA{A: 255})
			}
		}
	}
	return out, nil
}

// computeHomography solves for the 3×3 homography H that maps src[i]→dst[i].
// Uses Hartley normalization for numerical stability, then DLT via normal
// equations (supporting 4–8 pairs), then denormalizes the result.
func computeHomography(src, dst []Point) ([9]float64, error) {
	n := len(src)
	if n < 4 {
		return [9]float64{}, fmt.Errorf("need at least 4 point pairs, got %d", n)
	}

	// Hartley normalization: center and scale each point set so the mean
	// distance from origin is √2. This keeps DLT matrix values near ±1,
	// preventing precision loss from large pixel coordinates (~4000px).
	Ts, normSrc := normalizationTransform(src)
	Td, normDst := normalizationTransform(dst)

	// Build (2n)×8 design matrix A and RHS vector b from normalized points.
	// Row 2i:   [x, y, 1, 0, 0, 0, -x'x, -x'y] · h = x'
	// Row 2i+1: [0, 0, 0, x, y, 1, -y'x, -y'y] · h = y'
	rows := 2 * n
	Amat := make([][8]float64, rows)
	bvec := make([]float64, rows)
	for i := 0; i < n; i++ {
		x, y := normSrc[i].X, normSrc[i].Y
		xp, yp := normDst[i].X, normDst[i].Y
		Amat[2*i] = [8]float64{x, y, 1, 0, 0, 0, -xp * x, -xp * y}
		bvec[2*i] = xp
		Amat[2*i+1] = [8]float64{0, 0, 0, x, y, 1, -yp * x, -yp * y}
		bvec[2*i+1] = yp
	}

	// Normal equations: (AᵀA)·h = Aᵀb, forming the 8×9 augmented matrix.
	var aug [8][9]float64
	for i := 0; i < 8; i++ {
		for j := 0; j < 8; j++ {
			for k := 0; k < rows; k++ {
				aug[i][j] += Amat[k][i] * Amat[k][j]
			}
		}
		for k := 0; k < rows; k++ {
			aug[i][8] += Amat[k][i] * bvec[k]
		}
	}

	h, err := gaussianElimination(aug)
	if err != nil {
		return [9]float64{}, err
	}

	// H in normalized coordinates (h[8]=1 by construction).
	Hn := mat3{h[0], h[1], h[2], h[3], h[4], h[5], h[6], h[7], 1}

	// Denormalize: H = Td⁻¹ · Hn · Ts
	H := mat3Mul(mat3Mul(mat3Inv(Td), Hn), Ts)

	// Scale so H[8] = 1 (homogeneous normalization).
	if math.Abs(H[8]) > 1e-15 {
		for i := range H {
			H[i] /= H[8]
		}
	}

	return [9]float64(H), nil
}

// normalizationTransform computes the Hartley normalization matrix T and the
// transformed point set. T centers points at the origin and scales so the
// mean distance from origin is √2.
func normalizationTransform(pts []Point) (mat3, []Point) {
	n := float64(len(pts))

	var cx, cy float64
	for _, p := range pts {
		cx += p.X
		cy += p.Y
	}
	cx /= n
	cy /= n

	var dist float64
	for _, p := range pts {
		dx, dy := p.X-cx, p.Y-cy
		dist += math.Sqrt(dx*dx + dy*dy)
	}
	dist /= n
	if dist < 1e-10 {
		dist = 1
	}
	s := math.Sqrt2 / dist

	// T = [ s,  0, -s·cx ]
	//     [ 0,  s, -s·cy ]
	//     [ 0,  0,   1   ]
	T := mat3{s, 0, -s * cx, 0, s, -s * cy, 0, 0, 1}

	norm := make([]Point, len(pts))
	for i, p := range pts {
		norm[i] = Point{X: s * (p.X - cx), Y: s * (p.Y - cy)}
	}
	return T, norm
}

func mat3Mul(a, b mat3) mat3 {
	var c mat3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			for k := 0; k < 3; k++ {
				c[i*3+j] += a[i*3+k] * b[k*3+j]
			}
		}
	}
	return c
}

func mat3Inv(m mat3) mat3 {
	a, b, c := m[0], m[1], m[2]
	d, e, f := m[3], m[4], m[5]
	g, h, k := m[6], m[7], m[8]
	det := a*(e*k-f*h) - b*(d*k-f*g) + c*(d*h-e*g)
	if math.Abs(det) < 1e-15 {
		return mat3{1, 0, 0, 0, 1, 0, 0, 0, 1}
	}
	inv := 1.0 / det
	return mat3{
		(e*k - f*h) * inv, (c*h - b*k) * inv, (b*f - c*e) * inv,
		(f*g - d*k) * inv, (a*k - c*g) * inv, (c*d - a*f) * inv,
		(d*h - e*g) * inv, (b*g - a*h) * inv, (a*e - b*d) * inv,
	}
}

// gaussianElimination solves the 8×8 system encoded in the 8×9 augmented
// matrix using partial pivoting. Returns the 8 solution values.
func gaussianElimination(A [8][9]float64) ([8]float64, error) {
	const n = 8
	for col := 0; col < n; col++ {
		maxRow, maxVal := col, math.Abs(A[col][col])
		for row := col + 1; row < n; row++ {
			if v := math.Abs(A[row][col]); v > maxVal {
				maxVal, maxRow = v, row
			}
		}
		if maxVal < 1e-10 {
			return [8]float64{}, fmt.Errorf("degenerate point configuration: avoid collinear or coincident points")
		}
		A[col], A[maxRow] = A[maxRow], A[col]

		for row := col + 1; row < n; row++ {
			if A[row][col] == 0 {
				continue
			}
			factor := A[row][col] / A[col][col]
			for k := col; k <= n; k++ {
				A[row][k] -= factor * A[col][k]
			}
		}
	}

	var x [8]float64
	for i := n - 1; i >= 0; i-- {
		x[i] = A[i][n]
		for j := i + 1; j < n; j++ {
			x[i] -= A[i][j] * x[j]
		}
		x[i] /= A[i][i]
	}
	return x, nil
}

// applyHomography applies the 3×3 homography h to point (x,y).
func applyHomography(h [9]float64, x, y float64) (float64, float64) {
	w := h[6]*x + h[7]*y + h[8]
	return (h[0]*x + h[1]*y + h[2]) / w,
		(h[3]*x + h[4]*y + h[5]) / w
}

// bilinearSample samples img at sub-pixel position (x, y).
func bilinearSample(img *image.NRGBA, x, y float64) color.NRGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	x0 := int(math.Floor(x))
	y0 := int(math.Floor(y))
	x1 := x0 + 1
	y1 := y0 + 1
	fx := x - float64(x0)
	fy := y - float64(y0)

	clampX := func(v int) int {
		if v < 0 {
			return 0
		}
		if v >= w {
			return w - 1
		}
		return v
	}
	clampY := func(v int) int {
		if v < 0 {
			return 0
		}
		if v >= h {
			return h - 1
		}
		return v
	}

	c00 := img.NRGBAAt(clampX(x0), clampY(y0))
	c10 := img.NRGBAAt(clampX(x1), clampY(y0))
	c01 := img.NRGBAAt(clampX(x0), clampY(y1))
	c11 := img.NRGBAAt(clampX(x1), clampY(y1))

	lerp := func(a, b uint8, t float64) uint8 {
		return uint8(float64(a)*(1-t)+float64(b)*t + 0.5)
	}

	return color.NRGBA{
		R: lerp(lerp(c00.R, c10.R, fx), lerp(c01.R, c11.R, fx), fy),
		G: lerp(lerp(c00.G, c10.G, fx), lerp(c01.G, c11.G, fx), fy),
		B: lerp(lerp(c00.B, c10.B, fx), lerp(c01.B, c11.B, fx), fy),
		A: 255,
	}
}
