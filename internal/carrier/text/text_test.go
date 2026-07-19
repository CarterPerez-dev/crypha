/*
©AngelaMos | 2026
text_test.go

Round-trip, incidental-zero-width, normalization, and framing tests for the text carrier
*/

package text

import (
	"bytes"
	"io"
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/CarterPerez-dev/crypha/internal/carrier"
	"github.com/CarterPerez-dev/crypha/internal/payload"
	"golang.org/x/text/unicode/norm"
)

func hide(t *testing.T, cover string, data []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := (textCarrier{}).Hide(strings.NewReader(cover), data, &out); err != nil {
		t.Fatalf("Hide: %v", err)
	}
	return out.Bytes()
}

func reveal(t *testing.T, stego []byte) []byte {
	t.Helper()
	got, err := (textCarrier{}).Reveal(bytes.NewReader(stego))
	if err != nil {
		t.Fatalf("Reveal: %v", err)
	}
	return got
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

func TestRoundTripCovers(t *testing.T) {
	cases := []struct {
		name  string
		cover string
	}{
		{"empty cover", ""},
		{"ascii", "the quick brown fox"},
		{"multiline", "line one\nline two\nline three\n"},
		{"unicode cover", "café naïve 你好 \U0001F600 text"},
		{"whitespace only", "   \t\n  "},
	}
	payloadBytes := []byte("attack at dawn")
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stego := hide(t, tc.cover, payloadBytes)
			got := reveal(t, stego)
			if !bytes.Equal(got, payloadBytes) {
				t.Fatalf("round-trip mismatch: got %q want %q", got, payloadBytes)
			}
		})
	}
}

func TestRandomBinaryRoundTrip(t *testing.T) {
	cover := "cover text that stays visible"
	for _, size := range []int{1, 7, 64, 1000} {
		data := pseudoRandom(size, size)
		got := reveal(t, hide(t, cover, data))
		if !bytes.Equal(got, data) {
			t.Fatalf("random round-trip mismatch at size %d", size)
		}
	}
}

func TestCoverStaysVisible(t *testing.T) {
	cover := "this text is unchanged"
	data := []byte("hidden")
	stego := hide(t, cover, data)

	if !bytes.HasPrefix(stego, []byte(cover)) {
		t.Fatal("cover bytes were altered")
	}
	suffix := stego[len(cover):]
	for _, r := range string(suffix) {
		if r != zeroRune && r != oneRune {
			t.Fatalf("appended data contains a non-carrier rune: U+%04X", r)
		}
	}

	visible := strings.Map(func(r rune) rune {
		if r == zeroRune || r == oneRune {
			return -1
		}
		return r
	}, string(stego))
	if visible != cover {
		t.Fatalf("stripping carrier runes did not restore the cover: got %q", visible)
	}
}

func TestIncidentalZeroWidthInCover(t *testing.T) {
	cover := "prefix" + string(zeroRune) + string(oneRune) + string(zeroRune) + "suffix"
	data := []byte("payload survives noise")
	got := reveal(t, hide(t, cover, data))
	if !bytes.Equal(got, data) {
		t.Fatalf("incidental zero-width broke extraction: got %q", got)
	}
}

func TestNFCNormalizationSurvival(t *testing.T) {
	cover := "café test cover"
	data := []byte("normalization is not a threat")
	stego := hide(t, cover, data)

	for _, form := range []norm.Form{norm.NFC, norm.NFD, norm.NFKC, norm.NFKD} {
		normalized := form.Bytes(stego)
		got, err := (textCarrier{}).Reveal(bytes.NewReader(normalized))
		if err != nil {
			t.Fatalf("Reveal after normalization: %v", err)
		}
		if !bytes.Equal(got, data) {
			t.Fatalf("payload lost through normalization form: got %q", got)
		}
	}
}

func TestCapacityUnbounded(t *testing.T) {
	got, err := (textCarrier{}).Capacity(strings.NewReader("anything"))
	if err != nil {
		t.Fatalf("Capacity: %v", err)
	}
	if got != math.MaxInt32 {
		t.Fatalf("Capacity: got %d want unbounded (%d)", got, math.MaxInt32)
	}
}

func TestEmptyPayloadRejected(t *testing.T) {
	var out bytes.Buffer
	if err := (textCarrier{}).Hide(strings.NewReader("cover"), nil, &out); err != ErrEmptyPayload {
		t.Fatalf("expected ErrEmptyPayload, got %v", err)
	}
}

func TestRevealNoPayload(t *testing.T) {
	cases := []struct {
		name  string
		stego string
	}{
		{"plain text", "just some ordinary text with no secrets"},
		{"empty", ""},
		{"incidental zero-width without magic", "a" + string(zeroRune) + string(oneRune) + "b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := (textCarrier{}).Reveal(strings.NewReader(tc.stego)); err != ErrNoPayload {
				t.Fatalf("expected ErrNoPayload, got %v", err)
			}
		})
	}
}

func TestSniff(t *testing.T) {
	framed := hide(t, "cover", []byte("secret"))
	cases := []struct {
		name string
		data []byte
		want bool
	}{
		{"framed", framed, true},
		{"plain", []byte("nothing hidden here"), false},
		{"incidental zw no magic", []byte("x" + string(zeroRune) + string(oneRune) + "y"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (textCarrier{}).Sniff(bytes.NewReader(tc.data)); got != tc.want {
				t.Fatalf("Sniff(%s): got %v want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestOverheadRatio(t *testing.T) {
	data := []byte{0x00}
	stego := hide(t, "", data)
	runeCount := utf8.RuneCount(stego)
	wantRunes := (len(textMagic) + lengthBytes + len(data)) * bitsPerByte
	if runeCount != wantRunes {
		t.Fatalf("carrier-rune count: got %d want %d", runeCount, wantRunes)
	}
}

func TestEncryptedEnvelopeThroughCarrier(t *testing.T) {
	secret := []byte("the account number is 4815162342")
	envelope, err := payload.Pack(secret, payload.Options{
		Passphrase: []byte("open sesame"),
		Compress:   true,
		Cipher:     payload.CipherChaCha20,
		Strength:   payload.StrengthDefault,
	})
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}

	stego := hide(t, "innocuous cover message", envelope)
	recovered := reveal(t, stego)
	if !bytes.Equal(recovered, envelope) {
		t.Fatal("carrier did not return the exact envelope bytes")
	}

	plain, err := payload.Unpack(recovered, []byte("open sesame"))
	if err != nil {
		t.Fatalf("Unpack: %v", err)
	}
	if !bytes.Equal(plain, secret) {
		t.Fatalf("end-to-end mismatch: got %q want %q", plain, secret)
	}
}

type errReader struct{}

func (errReader) Read(_ []byte) (int, error) { return 0, io.ErrUnexpectedEOF }

type errWriter struct{}

func (errWriter) Write(_ []byte) (int, error) { return 0, io.ErrShortWrite }

func TestHideWriteError(t *testing.T) {
	if err := (textCarrier{}).Hide(strings.NewReader("cover"), []byte("x"), errWriter{}); err != io.ErrShortWrite {
		t.Fatalf("expected write error to propagate, got %v", err)
	}
}

func TestReadErrorsPropagate(t *testing.T) {
	if _, err := (textCarrier{}).Reveal(errReader{}); err == nil {
		t.Fatal("expected Reveal read error")
	}
	if (textCarrier{}).Sniff(readSeekerErr{}) {
		t.Fatal("Sniff should be false when the reader errors")
	}
}

type readSeekerErr struct{}

func (readSeekerErr) Read(_ []byte) (int, error)         { return 0, io.ErrUnexpectedEOF }
func (readSeekerErr) Seek(_ int64, _ int) (int64, error) { return 0, nil }

func zeroWidth(b []byte) string {
	var sb strings.Builder
	for _, bit := range bytesToBits(b) {
		sb.WriteRune(runeForBit(bit))
	}
	return sb.String()
}

func TestCorruptFramesRejected(t *testing.T) {
	cases := []struct {
		name  string
		stego string
	}{
		{"zero length field", zeroWidth(append(append([]byte{}, textMagic[:]...), 0, 0, 0, 0))},
		{"truncated length field", zeroWidth(textMagic[:]) + strings.Repeat(string(zeroRune), 10)},
		{"length exceeds payload bits", zeroWidth(append(append([]byte{}, textMagic[:]...), 0, 0, 0, 100)) + strings.Repeat(string(oneRune), bitsPerByte)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := (textCarrier{}).Reveal(strings.NewReader(tc.stego)); err != ErrNoPayload {
				t.Fatalf("expected ErrNoPayload, got %v", err)
			}
		})
	}
}

func TestNestedStegoReturnsAppendedFrame(t *testing.T) {
	first := []byte("first hidden layer")
	stegoA := hide(t, "public cover text", first)
	if got := reveal(t, stegoA); !bytes.Equal(got, first) {
		t.Fatalf("layer A: got %q want %q", got, first)
	}

	second := []byte("second layer wins")
	stegoB := hide(t, string(stegoA), second)
	if got := reveal(t, stegoB); !bytes.Equal(got, second) {
		t.Fatalf("nested reveal must return the appended frame: got %q want %q", got, second)
	}
}

func TestPayloadContainingMagicRoundTrips(t *testing.T) {
	data := append(append([]byte("head"), textMagic[:]...), []byte("tail after magic bytes")...)
	got := reveal(t, hide(t, "cover", data))
	if !bytes.Equal(got, data) {
		t.Fatalf("payload containing magic bytes corrupted: got %q want %q", got, data)
	}
}

func TestRegisteredInRegistry(t *testing.T) {
	c, ok := carrier.Get(Format)
	if !ok {
		t.Fatal("text carrier did not self-register")
	}
	if c.Format() != Format {
		t.Fatalf("registry returned wrong carrier: %s", c.Format())
	}
}
