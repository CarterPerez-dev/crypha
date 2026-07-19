/*
©AngelaMos | 2026
engine_test.go

Resolution, round-trip, auto-detect, and capacity tests for the shared engine
*/

package engine

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/CarterPerez-dev/crypha/internal/carrier/pdf"
	"github.com/CarterPerez-dev/crypha/internal/payload"
)

const (
	qrCoverText = "crypha qr cover"
	textCover   = "the quick brown fox jumps over the lazy dog"
)

func pngCover(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: uint8(x + y), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func hideBytes(t *testing.T, format string, cover, plaintext []byte, opts payload.Options) []byte {
	t.Helper()
	var out bytes.Buffer
	if _, err := Hide(HideRequest{
		Format:  format,
		Cover:   bytes.NewReader(cover),
		Payload: plaintext,
		Out:     &out,
		Options: opts,
	}); err != nil {
		t.Fatalf("Hide(%s): %v", format, err)
	}
	return out.Bytes()
}

func TestResolveCarrier(t *testing.T) {
	cases := []struct {
		name      string
		format    string
		technique string
		wantErr   error
		wantFmt   string
	}{
		{"image", "image", "", nil, "image"},
		{"pdf default", "pdf", "", nil, "pdf"},
		{"pdf attachment", "pdf", "attachment", nil, "pdf"},
		{"pdf metadata", "pdf", "metadata", nil, "pdf"},
		{"pdf append", "pdf", "append", nil, "pdf"},
		{"pdf bad technique", "pdf", "nonsense", ErrUnknownTechnique, ""},
		{"technique on non-pdf", "image", "attachment", ErrTechniqueOnNonPDF, ""},
		{"unknown format", "nope", "", ErrUnknownFormat, ""},
		{"empty format", "", "", ErrNoFormat, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := ResolveCarrier(tc.format, tc.technique)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if c.Format() != tc.wantFmt {
				t.Fatalf("format = %q, want %q", c.Format(), tc.wantFmt)
			}
		})
	}
}

func TestPlaintextRoundTrip(t *testing.T) {
	cases := []struct {
		format string
		cover  []byte
	}{
		{"text", []byte(textCover)},
		{"image", pngCover(t, 64, 64)},
		{"qr", []byte(qrCoverText)},
	}
	secret := []byte("meet at dawn")
	for _, tc := range cases {
		t.Run(tc.format, func(t *testing.T) {
			stego := hideBytes(t, tc.format, tc.cover, secret, payload.Options{})
			res, err := Reveal(RevealRequest{Format: tc.format, Stego: stego})
			if err != nil {
				t.Fatalf("Reveal: %v", err)
			}
			if !bytes.Equal(res.Data, secret) {
				t.Fatalf("data = %q, want %q", res.Data, secret)
			}
			if res.Encrypted {
				t.Fatal("plaintext payload reported as encrypted")
			}
		})
	}
}

func TestEncryptedRoundTrip(t *testing.T) {
	cover := pngCover(t, 64, 64)
	secret := []byte("classified")
	pass := []byte("correct horse battery staple")
	stego := hideBytes(t, "image", cover, secret, payload.Options{Passphrase: pass})

	res, err := Reveal(RevealRequest{Format: "image", Stego: stego, Passphrase: pass})
	if err != nil {
		t.Fatalf("Reveal: %v", err)
	}
	if !bytes.Equal(res.Data, secret) {
		t.Fatalf("data = %q, want %q", res.Data, secret)
	}
	if !res.Encrypted {
		t.Fatal("encrypted payload reported as plaintext")
	}

	if _, err := Reveal(RevealRequest{Format: "image", Stego: stego}); !errors.Is(err, payload.ErrPassphraseRequired) {
		t.Fatalf("missing passphrase err = %v, want ErrPassphraseRequired", err)
	}
	if _, err := Reveal(RevealRequest{Format: "image", Stego: stego, Passphrase: []byte("wrong")}); !errors.Is(err, payload.ErrDecrypt) {
		t.Fatalf("wrong passphrase err = %v, want ErrDecrypt", err)
	}
}

func TestAutoDetect(t *testing.T) {
	secret := []byte("hi")
	cases := []struct {
		format string
		cover  []byte
	}{
		{"text", []byte(textCover)},
		{"image", pngCover(t, 64, 64)},
		{"qr", []byte(qrCoverText)},
	}
	for _, tc := range cases {
		t.Run(tc.format, func(t *testing.T) {
			stego := hideBytes(t, tc.format, tc.cover, secret, payload.Options{})
			res, err := Reveal(RevealRequest{Stego: stego})
			if err != nil {
				t.Fatalf("auto-detect Reveal: %v", err)
			}
			if res.Format != tc.format {
				t.Fatalf("detected %q, want %q", res.Format, tc.format)
			}
			if !bytes.Equal(res.Data, secret) {
				t.Fatalf("data = %q, want %q", res.Data, secret)
			}
		})
	}
}

func TestAutoDetectQRNotShadowedByImage(t *testing.T) {
	stego := hideBytes(t, "qr", []byte(qrCoverText), []byte("hi"), payload.Options{})
	res, err := Reveal(RevealRequest{Stego: stego})
	if err != nil {
		t.Fatalf("Reveal: %v", err)
	}
	if res.Format != "qr" {
		t.Fatalf("a qr PNG resolved to %q; the image carrier shadowed qr", res.Format)
	}
}

func TestAutoDetectUndetected(t *testing.T) {
	if _, err := Reveal(RevealRequest{Stego: []byte("just some plain bytes, not a carrier")}); !errors.Is(err, ErrUndetected) {
		t.Fatalf("err = %v, want ErrUndetected", err)
	}
}

func TestCatalogAndTechniques(t *testing.T) {
	cat := Catalog()
	if len(cat) != 5 {
		t.Fatalf("catalog has %d formats, want 5", len(cat))
	}
	for _, fi := range cat {
		if fi.Name == pdf.Format {
			if len(fi.Techniques) != 3 {
				t.Fatalf("pdf techniques = %v, want 3", fi.Techniques)
			}
		} else if fi.Techniques != nil {
			t.Fatalf("%s has techniques %v, want none", fi.Name, fi.Techniques)
		}
	}
}

func TestCapacityAll(t *testing.T) {
	rows := CapacityAll(pngCover(t, 64, 64))
	var imageRow *CapacityRow
	for i := range rows {
		if rows[i].Format == "image" {
			imageRow = &rows[i]
		}
	}
	if imageRow == nil {
		t.Fatal("no image row in capacity table")
	}
	if imageRow.Err != nil {
		t.Fatalf("image capacity err: %v", imageRow.Err)
	}
	if imageRow.Capacity <= 0 {
		t.Fatalf("image capacity = %d, want positive", imageRow.Capacity)
	}
}

func TestCapacitySingleFormat(t *testing.T) {
	n, err := Capacity("image", bytes.NewReader(pngCover(t, 32, 32)))
	if err != nil {
		t.Fatalf("Capacity: %v", err)
	}
	if n <= 0 {
		t.Fatalf("capacity = %d, want positive", n)
	}
}
