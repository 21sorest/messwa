package dashboard

import (
	"bytes"
	"errors"
	"image"
	_ "image/jpeg" // UPI QR uploads
	_ "image/png"
	"io"
	"net/http"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	"rsc.io/qr"

	"messwa/internal/config"
)

// upiQRRoutes adds POST /api/upi-qr, which reads the UPI ID out of a UPI
// QR image (multipart, as qr) for the settings page. It stores nothing:
// the page puts the UPI ID in sell.upi, which saves with the rest. Any
// signed-in user may use it, so the welcome page can too.
func upiQRRoutes(mux *http.ServeMux) {
	mux.Handle("POST /api/upi-qr", signedIn(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxQR+64<<10)
		if err := r.ParseMultipartForm(maxQR + 64<<10); err != nil {
			jsonError(w, "the image is too large (5 MB at most)", http.StatusBadRequest)
			return
		}
		f, _, err := r.FormFile("qr")
		if err != nil {
			jsonError(w, "attach your UPI QR image", http.StatusBadRequest)
			return
		}
		defer f.Close()
		data, err := io.ReadAll(io.LimitReader(f, maxQR+1))
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		upi, err := readUPIQR(data)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, r, struct {
			UPI string `json:"upi"`
		}{upi})
	}))

	// GET /api/upi-qr-preview?upi=ID draws the QR a buyer would get for a
	// ₹50 meal, for the welcome page's sample chat.
	mux.Handle("GET /api/upi-qr-preview", signedIn(func(w http.ResponseWriter, r *http.Request) {
		upi, err := config.ParseUPI(r.URL.Query().Get("upi"))
		if err != nil || upi == "" {
			jsonError(w, "not a UPI ID", http.StatusBadRequest)
			return
		}
		code, err := qr.Encode(config.UPIPayURI(upi, previewAmount, "messwa preview"), qr.M)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "private, max-age=3600")
		w.Write(code.PNG())
	}))
}

// previewAmount is the price in the welcome page's sample chat.
const previewAmount = "50.00"

// readUPIQR finds the QR in a PNG or JPEG, such as a screenshot of your
// UPI app's "receive money" screen, and returns the UPI ID it pays.
func readUPIQR(data []byte) (string, error) {
	if len(data) > maxQR {
		return "", errors.New("the image is over 5 MB")
	}
	c, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") {
		return "", errors.New("the QR must be a PNG or JPEG image")
	}
	if c.Width > 8192 || c.Height > 8192 {
		return "", errors.New("the image is too big; crop it to the QR")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", errors.New("couldn't read that image")
	}
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", errors.New("couldn't read that image")
	}
	hints := map[gozxing.DecodeHintType]any{gozxing.DecodeHintType_TRY_HARDER: true}
	res, err := qrcode.NewQRCodeReader().Decode(bmp, hints)
	if err != nil {
		return "", errors.New("no QR found in that image; try a sharper screenshot, or type your UPI ID")
	}
	return config.ParseUPI(res.GetText())
}
