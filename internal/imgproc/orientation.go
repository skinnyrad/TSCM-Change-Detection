package imgproc

import (
	"encoding/binary"
	"image"
)

// ExifOrientation returns the EXIF orientation tag (1–8) of a JPEG, or 1 if the
// data is not a JPEG or has no orientation. Go's image decoders ignore this
// tag, so phone photos taken in portrait would otherwise load sideways.
func ExifOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}
	i := 2
	for i+4 <= len(data) {
		if data[i] != 0xFF {
			return 1
		}
		marker := data[i+1]
		if marker == 0xDA || marker == 0xD9 { // start of scan / end of image
			return 1
		}
		size := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
		if size < 2 || i+2+size > len(data) {
			return 1
		}
		if marker == 0xE1 && size >= 16 && string(data[i+4:i+10]) == "Exif\x00\x00" {
			return parseTiffOrientation(data[i+10 : i+2+size])
		}
		i += 2 + size
	}
	return 1
}

func parseTiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	off := int(bo.Uint32(t[4:8]))
	if off < 8 || off+2 > len(t) {
		return 1
	}
	n := int(bo.Uint16(t[off : off+2]))
	for e := 0; e < n; e++ {
		p := off + 2 + e*12
		if p+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[p:p+2]) == 0x0112 {
			if v := int(bo.Uint16(t[p+8 : p+10])); v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

// ApplyOrientation returns img transformed so it displays upright given its
// EXIF orientation value (1 = already upright).
func ApplyOrientation(img *image.NRGBA, o int) *image.NRGBA {
	if o <= 1 || o > 8 {
		return img
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	ow, oh := w, h
	if o >= 5 {
		ow, oh = h, w
	}
	out := image.NewNRGBA(image.Rect(0, 0, ow, oh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch o {
			case 2: // mirror horizontal
				dx, dy = w-1-x, y
			case 3: // rotate 180
				dx, dy = w-1-x, h-1-y
			case 4: // mirror vertical
				dx, dy = x, h-1-y
			case 5: // transpose
				dx, dy = y, x
			case 6: // rotate 90 CW
				dx, dy = h-1-y, x
			case 7: // transverse
				dx, dy = h-1-y, w-1-x
			case 8: // rotate 90 CCW
				dx, dy = y, w-1-x
			}
			copy(out.Pix[dy*out.Stride+dx*4:dy*out.Stride+dx*4+4], img.Pix[y*img.Stride+x*4:y*img.Stride+x*4+4])
		}
	}
	return out
}
