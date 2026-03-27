package imgproc

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
)

// EncodeBase64PNG encodes an image as a PNG and returns a data URI string
// suitable for use directly in an HTML <img src="..."> attribute.
func EncodeBase64PNG(img image.Image) (string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
