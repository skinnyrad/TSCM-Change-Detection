package imgproc

import (
	"image"
	"testing"
)

func TestRegionsDiagonalIsOneRegion(t *testing.T) {
	m := image.NewGray(image.Rect(0, 0, 10, 10))
	m.Pix[1*m.Stride+1] = 255
	m.Pix[2*m.Stride+2] = 255 // diagonal neighbour: 8-connected
	m.Pix[8*m.Stride+8] = 255
	got := Regions(m)
	if len(got) != 2 {
		t.Fatalf("want 2 regions, got %d", len(got))
	}
	if got[0].Area != 2 || got[0].Box != image.Rect(1, 1, 3, 3) {
		t.Fatalf("unexpected first region: %+v", got[0])
	}
}
