/*
©AngelaMos | 2026
image_test.go

Round-trip, rejection, capacity, and sniff tests for the LSB image carrier
*/

package image

import (
	"bytes"
	stdimage "image"
	"image/color"
	"image/color/palette"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/CarterPerez-dev/crypha/internal/carrier"
	"github.com/CarterPerez-dev/crypha/internal/payload"
	xbmp "golang.org/x/image/bmp"
)

const (
	coverWidth  = 8
	coverHeight = 8
	coverBytes  = 20
)

func nrgbaCover(w, h int) *stdimage.NRGBA {
	img := stdimage.NewNRGBA(stdimage.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += bytesPerPixel {
		img.Pix[i] = byte(i)
		img.Pix[i+1] = byte(i * 3)
		img.Pix[i+2] = byte(i * 7)
		img.Pix[i+3] = 0xFF
	}
	return img
}

func encodePNG(t *testing.T, img stdimage.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png cover: %v", err)
	}
	return buf.Bytes()
}

func encodeBMP(t *testing.T, img stdimage.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := xbmp.Encode(&buf, img); err != nil {
		t.Fatalf("encode bmp cover: %v", err)
	}
	return buf.Bytes()
}

func hideReveal(t *testing.T, cover, payload []byte) []byte {
	t.Helper()
	var stego bytes.Buffer
	if err := (imageCarrier{}).Hide(bytes.NewReader(cover), payload, &stego); err != nil {
		t.Fatalf("Hide: %v", err)
	}
	got, err := (imageCarrier{}).Reveal(bytes.NewReader(stego.Bytes()))
	if err != nil {
		t.Fatalf("Reveal: %v", err)
	}
	return got
}

func TestRoundTripPNG(t *testing.T) {
	cover := encodePNG(t, nrgbaCover(coverWidth, coverHeight))
	cases := []struct {
		name    string
		payload []byte
	}{
		{"single byte", []byte{0x42}},
		{"text", []byte("crypha")},
		{"full capacity", bytes.Repeat([]byte{0xAB}, coverBytes)},
		{"high bits set", bytes.Repeat([]byte{0xFF}, coverBytes)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := hideReveal(t, cover, tc.payload)
			if !bytes.Equal(got, tc.payload) {
				t.Fatalf("round-trip mismatch: got %x want %x", got, tc.payload)
			}
		})
	}
}

func TestRoundTripBMP(t *testing.T) {
	cover := encodeBMP(t, nrgbaCover(coverWidth, coverHeight))
	payload := []byte("bmp carrier")
	got := hideReveal(t, cover, payload)
	if !bytes.Equal(got, payload) {
		t.Fatalf("bmp round-trip mismatch: got %x want %x", got, payload)
	}
}

func TestRGBASourceSafety(t *testing.T) {
	rgba := stdimage.NewRGBA(stdimage.Rect(0, 0, coverWidth, coverHeight))
	for i := 0; i < len(rgba.Pix); i += bytesPerPixel {
		rgba.Pix[i] = byte(i * 5)
		rgba.Pix[i+1] = byte(i * 11)
		rgba.Pix[i+2] = byte(i * 13)
		rgba.Pix[i+3] = 0xFF
	}
	cover := encodePNG(t, rgba)
	payload := []byte("premultiply safe")
	got := hideReveal(t, cover, payload)
	if !bytes.Equal(got, payload) {
		t.Fatalf("rgba-source round-trip mismatch: got %x want %x", got, payload)
	}
}

func TestTransparentCoverAlphaUntouched(t *testing.T) {
	cover := stdimage.NewNRGBA(stdimage.Rect(0, 0, coverWidth, coverHeight))
	for i := 0; i < len(cover.Pix); i += bytesPerPixel {
		cover.Pix[i] = byte(i)
		cover.Pix[i+1] = byte(i * 2)
		cover.Pix[i+2] = byte(i * 4)
		cover.Pix[i+3] = byte(0x80 + i%64)
	}
	encoded := encodePNG(t, cover)

	var stego bytes.Buffer
	if err := (imageCarrier{}).Hide(bytes.NewReader(encoded), []byte("hi"), &stego); err != nil {
		t.Fatalf("Hide: %v", err)
	}
	decoded, _, err := stdimage.Decode(bytes.NewReader(stego.Bytes()))
	if err != nil {
		t.Fatalf("decode stego: %v", err)
	}
	out := toNRGBA(decoded)
	for i := 0; i < len(out.Pix); i += bytesPerPixel {
		if out.Pix[i+3] != cover.Pix[i+3] {
			t.Fatalf("alpha modified at pixel %d: got %d want %d", i/bytesPerPixel, out.Pix[i+3], cover.Pix[i+3])
		}
	}
}

func pseudoRandom(n, seed int) []byte {
	b := make([]byte, n)
	x := uint32(seed)*2654435761 + 1
	for i := range b {
		x = x*1664525 + 1013904223
		b[i] = byte(x >> 24)
	}
	return b
}

func TestRandomBinaryRoundTrip(t *testing.T) {
	cover := encodePNG(t, nrgbaCover(64, 64))
	for _, size := range []int{1, 17, 100, 500} {
		payload := pseudoRandom(size, size)
		got := hideReveal(t, cover, payload)
		if !bytes.Equal(got, payload) {
			t.Fatalf("random round-trip mismatch at size %d", size)
		}
	}
}

func TestPalettedRejected(t *testing.T) {
	pal := stdimage.NewPaletted(stdimage.Rect(0, 0, coverWidth, coverHeight), palette.WebSafe)
	for y := 0; y < coverHeight; y++ {
		for x := 0; x < coverWidth; x++ {
			pal.Set(x, y, color.RGBA{R: byte(x * 30), G: byte(y * 30), B: 0x40, A: 0xFF})
		}
	}
	cover := encodePNG(t, pal)
	err := (imageCarrier{}).Hide(bytes.NewReader(cover), []byte("x"), &bytes.Buffer{})
	if err != ErrPaletted {
		t.Fatalf("expected ErrPaletted, got %v", err)
	}
}

func TestSixteenBitRejected(t *testing.T) {
	img := stdimage.NewNRGBA64(stdimage.Rect(0, 0, coverWidth, coverHeight))
	for y := 0; y < coverHeight; y++ {
		for x := 0; x < coverWidth; x++ {
			img.Set(x, y, color.NRGBA64{R: 0x1234, G: 0x5678, B: 0x9ABC, A: 0xFFFF})
		}
	}
	cover := encodePNG(t, img)
	err := (imageCarrier{}).Hide(bytes.NewReader(cover), []byte("x"), &bytes.Buffer{})
	if err != Err16Bit {
		t.Fatalf("expected Err16Bit, got %v", err)
	}
}

func TestUnsupportedFormatRejected(t *testing.T) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, nrgbaCover(coverWidth, coverHeight), nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	err := (imageCarrier{}).Hide(bytes.NewReader(buf.Bytes()), []byte("x"), &bytes.Buffer{})
	if err != ErrUnsupportedFormat {
		t.Fatalf("expected ErrUnsupportedFormat, got %v", err)
	}
}

func TestEmptyPayloadRejected(t *testing.T) {
	cover := encodePNG(t, nrgbaCover(coverWidth, coverHeight))
	err := (imageCarrier{}).Hide(bytes.NewReader(cover), nil, &bytes.Buffer{})
	if err != ErrEmptyPayload {
		t.Fatalf("expected ErrEmptyPayload, got %v", err)
	}
}

func TestCapacityBoundary(t *testing.T) {
	cover := encodePNG(t, nrgbaCover(coverWidth, coverHeight))

	atCap := bytes.Repeat([]byte{0x01}, coverBytes)
	if got := hideReveal(t, cover, atCap); !bytes.Equal(got, atCap) {
		t.Fatal("payload at exact capacity failed to round-trip")
	}

	overCap := bytes.Repeat([]byte{0x01}, coverBytes+1)
	err := (imageCarrier{}).Hide(bytes.NewReader(cover), overCap, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected capacity error for oversized payload")
	}
}

func TestCapacityReport(t *testing.T) {
	cover := encodePNG(t, nrgbaCover(coverWidth, coverHeight))
	got, err := (imageCarrier{}).Capacity(bytes.NewReader(cover))
	if err != nil {
		t.Fatalf("Capacity: %v", err)
	}
	want := (coverWidth*coverHeight*channelsPerPixel - lengthPrefixBits) / bitsPerByte
	if got != want {
		t.Fatalf("Capacity: got %d want %d", got, want)
	}
	if want != coverBytes {
		t.Fatalf("test constant coverBytes stale: computed %d", want)
	}
}

func TestRevealNoPayload(t *testing.T) {
	clean := stdimage.NewNRGBA(stdimage.Rect(0, 0, coverWidth, coverHeight))
	for i := range clean.Pix {
		clean.Pix[i] = 0xFE
	}
	cover := encodePNG(t, clean)
	_, err := (imageCarrier{}).Reveal(bytes.NewReader(cover))
	if err != ErrNoPayload {
		t.Fatalf("expected ErrNoPayload on a cover with zeroed length bits, got %v", err)
	}
}

func TestSniff(t *testing.T) {
	pngCover := encodePNG(t, nrgbaCover(coverWidth, coverHeight))
	bmpCover := encodeBMP(t, nrgbaCover(coverWidth, coverHeight))
	cases := []struct {
		name string
		data []byte
		want bool
	}{
		{"png", pngCover, true},
		{"bmp", bmpCover, true},
		{"random", []byte("not an image file at all"), false},
		{"short", []byte{0x89, 'P'}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (imageCarrier{}).Sniff(bytes.NewReader(tc.data)); got != tc.want {
				t.Fatalf("Sniff(%s): got %v want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestRevealRejectsUnsupportedAndGarbage(t *testing.T) {
	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, nrgbaCover(coverWidth, coverHeight), nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	if _, err := (imageCarrier{}).Reveal(bytes.NewReader(jpg.Bytes())); err != ErrUnsupportedFormat {
		t.Fatalf("Reveal jpeg: got %v want ErrUnsupportedFormat", err)
	}
	if _, err := (imageCarrier{}).Reveal(bytes.NewReader([]byte("garbage"))); err == nil {
		t.Fatal("Reveal garbage: expected decode error")
	}
}

func TestRevealRejectsLossy(t *testing.T) {
	pal := stdimage.NewPaletted(stdimage.Rect(0, 0, coverWidth, coverHeight), palette.WebSafe)
	for y := 0; y < coverHeight; y++ {
		for x := 0; x < coverWidth; x++ {
			pal.Set(x, y, color.RGBA{R: byte(x * 30), G: byte(y * 30), B: 0x40, A: 0xFF})
		}
	}
	if _, err := (imageCarrier{}).Reveal(bytes.NewReader(encodePNG(t, pal))); err != ErrPaletted {
		t.Fatalf("Reveal paletted: got %v want ErrPaletted", err)
	}

	wide := stdimage.NewNRGBA64(stdimage.Rect(0, 0, coverWidth, coverHeight))
	wide.Set(0, 0, color.NRGBA64{R: 0x1234, G: 0x5678, B: 0x9ABC, A: 0xFFFF})
	if _, err := (imageCarrier{}).Reveal(bytes.NewReader(encodePNG(t, wide))); err != Err16Bit {
		t.Fatalf("Reveal 16-bit: got %v want Err16Bit", err)
	}
}

func TestRevealTooSmall(t *testing.T) {
	cover := encodePNG(t, nrgbaCover(1, 1))
	if _, err := (imageCarrier{}).Reveal(bytes.NewReader(cover)); err != ErrTooSmall {
		t.Fatalf("Reveal 1x1: got %v want ErrTooSmall", err)
	}
}

func TestCapacityRejectsUnsupportedAndTiny(t *testing.T) {
	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, nrgbaCover(coverWidth, coverHeight), nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	if _, err := (imageCarrier{}).Capacity(bytes.NewReader(jpg.Bytes())); err != ErrUnsupportedFormat {
		t.Fatalf("Capacity jpeg: got %v want ErrUnsupportedFormat", err)
	}
	if _, err := (imageCarrier{}).Capacity(bytes.NewReader([]byte("garbage"))); err == nil {
		t.Fatal("Capacity garbage: expected decode error")
	}
	tiny := encodePNG(t, nrgbaCover(1, 1))
	got, err := (imageCarrier{}).Capacity(bytes.NewReader(tiny))
	if err != nil {
		t.Fatalf("Capacity 1x1: %v", err)
	}
	if got != 0 {
		t.Fatalf("Capacity 1x1: got %d want 0", got)
	}
}

func TestEncryptedEnvelopeThroughCarrier(t *testing.T) {
	secret := []byte("meet at the docks at midnight")
	envelope, err := payload.Pack(secret, payload.Options{
		Passphrase: []byte("correct horse battery staple"),
		Compress:   true,
		Cipher:     payload.CipherChaCha20,
		Strength:   payload.StrengthDefault,
	})
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}

	cover := encodePNG(t, nrgbaCover(96, 96))
	var stego bytes.Buffer
	if err := (imageCarrier{}).Hide(bytes.NewReader(cover), envelope, &stego); err != nil {
		t.Fatalf("Hide envelope: %v", err)
	}

	recovered, err := (imageCarrier{}).Reveal(bytes.NewReader(stego.Bytes()))
	if err != nil {
		t.Fatalf("Reveal envelope: %v", err)
	}
	if !bytes.Equal(recovered, envelope) {
		t.Fatal("carrier did not return the exact envelope bytes")
	}

	plain, err := payload.Unpack(recovered, []byte("correct horse battery staple"))
	if err != nil {
		t.Fatalf("Unpack: %v", err)
	}
	if !bytes.Equal(plain, secret) {
		t.Fatalf("end-to-end mismatch: got %q want %q", plain, secret)
	}
}

func TestRegisteredInRegistry(t *testing.T) {
	c, ok := carrier.Get(Format)
	if !ok {
		t.Fatal("image carrier did not self-register")
	}
	if c.Format() != Format {
		t.Fatalf("registry returned wrong carrier: %s", c.Format())
	}
}
