package imgproc

import (
	"image"
	"image/color"
	"testing"
)

func TestExifOrientation(t *testing.T) {
	// Minimal JPEG header with a big-endian EXIF block whose orientation is 6.
	tiff := []byte{'M', 'M', 0, 42, 0, 0, 0, 8, 0, 1, 0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, 6, 0, 0, 0, 0, 0, 0}
	seg := append([]byte("Exif\x00\x00"), tiff...)
	data := []byte{0xFF, 0xD8, 0xFF, 0xE1, byte((len(seg) + 2) >> 8), byte(len(seg) + 2)}
	data = append(data, seg...)
	if got := ExifOrientation(data); got != 6 {
		t.Fatalf("orientation = %d, want 6", got)
	}
	if got := ExifOrientation([]byte("not a jpeg")); got != 1 {
		t.Fatalf("non-jpeg orientation = %d, want 1", got)
	}
}

func TestApplyOrientationRotate90(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1)) // 2 wide, 1 tall
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	got := ApplyOrientation(img, 6) // rotate 90 CW → 1 wide, 2 tall, left pixel goes to top
	if got.Bounds().Dx() != 1 || got.Bounds().Dy() != 2 {
		t.Fatalf("bounds = %v", got.Bounds())
	}
	if got.NRGBAAt(0, 0).R != 255 {
		t.Fatalf("pixel did not land at top: %+v", got.NRGBAAt(0, 0))
	}
}
