/*
©AngelaMos | 2026
theme_test.go

Unit tests for the gradient engine, meter geometry, and pure view helpers
*/

package tui

import (
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

func TestRampLength(t *testing.T) {
	for _, n := range []int{0, 1, 2, 5, 30, 88} {
		if got := len(ramp(brandStops, n)); got != n {
			t.Fatalf("ramp(%d) length = %d", n, got)
		}
	}
}

func TestSpectralBarWidthInvariant(t *testing.T) {
	for _, w := range []int{1, 8, 24, 44} {
		for _, frac := range []float64{-0.5, 0, 0.33, 0.5, 1, 2.5} {
			bar := spectralBar(w, frac, capacityStops)
			if got := lipgloss.Width(bar); got != w {
				t.Fatalf("spectralBar(w=%d frac=%.2f) width = %d", w, frac, got)
			}
		}
	}
}

func TestGradientTextPreservesWidth(t *testing.T) {
	if got := lipgloss.Width(gradientText("crypha", brandStops, true)); got != 6 {
		t.Fatalf("gradientText width = %d, want 6", got)
	}
}

func TestWordmarkRowsAligned(t *testing.T) {
	lines := wordmarkLines()
	if len(lines) != wordmarkRows {
		t.Fatalf("wordmark has %d rows, want %d", len(lines), wordmarkRows)
	}
	width := utf8.RuneCountInString(lines[0])
	for i, ln := range lines {
		if got := utf8.RuneCountInString(ln); got != width {
			t.Fatalf("row %d width = %d, want %d", i, got, width)
		}
	}
}

func TestPickerWraps(t *testing.T) {
	p := newPicker([]pickItem{{title: "a"}, {title: "b"}, {title: "c"}})
	p.up()
	if p.cursor != 2 {
		t.Fatalf("up from 0 = %d, want 2", p.cursor)
	}
	p.down()
	if p.cursor != 0 {
		t.Fatalf("down from 2 = %d, want 0", p.cursor)
	}
}

func TestSecureFormFieldsAndToggle(t *testing.T) {
	f := newSecureForm()
	if len(f.fields()) != 2 {
		t.Fatalf("plaintext fields = %d, want 2", len(f.fields()))
	}
	f, _ = f.update(keySpace())
	if !f.encrypt {
		t.Fatalf("space did not enable encryption")
	}
	if len(f.fields()) != 5 {
		t.Fatalf("encrypted fields = %d, want 5", len(f.fields()))
	}
	f, _ = f.update(keyDown())
	f, _ = f.update(typeText("s3cret"))
	if string(f.passphrase()) != "s3cret" {
		t.Fatalf("passphrase = %q", f.passphrase())
	}
	if f.cipherValue() != "chacha20" {
		t.Fatalf("default cipher = %q", f.cipherValue())
	}
}

func TestSecureFormChoiceCycles(t *testing.T) {
	f := newSecureForm()
	f, _ = f.update(keySpace())
	f, _ = f.update(keyDown())
	f, _ = f.update(keyDown())
	if f.focusField() != secCipher {
		t.Fatalf("focus = %d, want secCipher", f.focusField())
	}
	before := f.cipherValue()
	f, _ = f.update(keySpace())
	if f.cipherValue() == before {
		t.Fatalf("cipher did not cycle from %q", before)
	}
}

func TestOutputExtensions(t *testing.T) {
	cases := map[string]string{"image": ".png", "audio": ".wav", "qr": ".png", "text": ".txt", "pdf": ".pdf"}
	for format, want := range cases {
		if got := outputExt(format); got != want {
			t.Fatalf("outputExt(%q) = %q, want %q", format, got, want)
		}
	}
}

func TestSuggestOutput(t *testing.T) {
	if got := suggestOutput("/home/x/photo.png", "image"); got != "photo.stego.png" {
		t.Fatalf("suggestOutput = %q", got)
	}
	if got := suggestOutput("", "audio"); got != "crypha.stego.wav" {
		t.Fatalf("suggestOutput empty = %q", got)
	}
}

func TestIsPrintable(t *testing.T) {
	if !isPrintable([]byte("hello\nworld\t!")) {
		t.Fatalf("printable text rejected")
	}
	if isPrintable([]byte{0x00, 0x01, 0xff}) {
		t.Fatalf("binary accepted as printable")
	}
}

func TestHexPreviewTruncates(t *testing.T) {
	data := make([]byte, 100)
	got := hexPreview(data, 4)
	if got != "00 00 00 00 ..." {
		t.Fatalf("hexPreview = %q", got)
	}
}

func TestFormatLabel(t *testing.T) {
	if formatLabel("", "") != "auto-detect" {
		t.Fatalf("empty format label")
	}
	if formatLabel("pdf", "append") != "pdf (append)" {
		t.Fatalf("technique label")
	}
	if formatLabel("image", "") != "image" {
		t.Fatalf("plain format label")
	}
}
