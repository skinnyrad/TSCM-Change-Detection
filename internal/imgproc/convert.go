package imgproc

import (
	"image"
	"image/color"
	"image/draw"
)

// ToNRGBA converts any image.Image to *image.NRGBA with (0,0) origin.
func ToNRGBA(src image.Image) *image.NRGBA {
	return toNRGBA(src)
}

// toNRGBA converts any image.Image to *image.NRGBA with (0,0) origin.
func toNRGBA(src image.Image) *image.NRGBA {
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return dst
}

// toGray converts an *image.NRGBA to *image.Gray using Rec. 601 luma weights
// (0.299R + 0.587G + 0.114B) to match OpenCV's COLOR_RGB2GRAY.
func toGray(img *image.NRGBA) *image.Gray {
	b := img.Bounds()
	out := image.NewGray(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c := img.NRGBAAt(x, y)
			luma := uint8(float64(c.R)*0.299 + float64(c.G)*0.587 + float64(c.B)*0.114 + 0.5)
			out.SetGray(x, y, color.Gray{Y: luma})
		}
	}
	return out
}

// grayToNRGBA converts *image.Gray to *image.NRGBA by replicating the gray
// value across all RGB channels with full opacity.
func grayToNRGBA(gray *image.Gray) *image.NRGBA {
	b := gray.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			v := gray.GrayAt(x, y).Y
			out.SetNRGBA(x, y, color.NRGBA{R: v, G: v, B: v, A: 255})
		}
	}
	return out
}

// nrgbaToGray converts *image.NRGBA back to *image.Gray by reading the R
// channel (valid when R=G=B, i.e. after round-tripping a gray image).
func nrgbaToGray(img *image.NRGBA) *image.Gray {
	b := img.Bounds()
	out := image.NewGray(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			out.SetGray(x, y, color.Gray{Y: img.NRGBAAt(x, y).R})
		}
	}
	return out
}

// rgbaToGray converts *image.RGBA to *image.Gray by reading the R channel
// (valid when R=G=B, i.e. after round-tripping a gray image through bild).
func rgbaToGray(img *image.RGBA) *image.Gray {
	b := img.Bounds()
	out := image.NewGray(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			out.SetGray(x, y, color.Gray{Y: img.RGBAAt(x, y).R})
		}
	}
	return out
}

// rgbaToNRGBA converts *image.RGBA to *image.NRGBA.
func rgbaToNRGBA(img *image.RGBA) *image.NRGBA {
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)
	return out
}
