package imgproc

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
)

// EncodeBase64PNG encodes an image as a PNG (BestSpeed compression) and
// returns a data URI string suitable for use in an HTML <img src="..."> attribute.
func EncodeBase64PNG(img image.Image) (string, error) {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, img); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
