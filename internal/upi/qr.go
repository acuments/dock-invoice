package upi

import (
	"fmt"

	qrcode "github.com/skip2/go-qrcode"
)

// PNG encodes uri as a QR code image. pixelSize is the width/height in
// pixels. Low error correction keeps the module grid sparse enough to scan
// reliably when printed on an invoice.
func PNG(uri string, pixelSize int) ([]byte, error) {
	if uri == "" {
		return nil, fmt.Errorf("upi: empty payment URI")
	}
	if pixelSize <= 0 {
		pixelSize = 256
	}
	q, err := qrcode.New(uri, qrcode.Low)
	if err != nil {
		return nil, err
	}
	return q.PNG(pixelSize)
}
