/*
©AngelaMos | 2026
qr_test.go

Differential tests against skip2 and gozxing plus round-trip, capacity, sniff, and registry checks
*/

package qr

import (
	"bytes"
	"errors"
	stdimage "image"
	"image/png"
	"strings"
	"testing"

	"github.com/CarterPerez-dev/crypha/internal/carrier"
	"github.com/CarterPerez-dev/crypha/internal/payload"
	gozxing "github.com/makiuchi-d/gozxing"
	gozxingqr "github.com/makiuchi-d/gozxing/qrcode"
	qrcode "github.com/skip2/go-qrcode"
)

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, errors.New("crypha/qr test: forced read error")
}

const testCover = "crypha"

func qrRandom(n, seed int) []byte {
	b := make([]byte, n)
	x := uint32(seed)*2654435761 + 1
	for i := range b {
		x = x*1664525 + 1013904223
		b[i] = byte(x >> 24)
	}
	return b
}

func skip2Clean(t *testing.T, cover string, version int) matrix {
	t.Helper()
	code, err := qrcode.NewWithForcedVersion(cover, version, qrcode.Highest)
	if err != nil {
		t.Fatalf("skip2 encode v%d: %v", version, err)
	}
	code.DisableBorder = true
	m, err := matrixFromBitmap(code.Bitmap(), version)
	if err != nil {
		t.Fatalf("matrixFromBitmap v%d: %v", version, err)
	}
	return m
}

func hideReveal(t *testing.T, cover string, payloadBytes []byte) []byte {
	t.Helper()
	var stego bytes.Buffer
	if err := (qrCarrier{}).Hide(strings.NewReader(cover), payloadBytes, &stego); err != nil {
		t.Fatalf("Hide: %v", err)
	}
	got, err := (qrCarrier{}).Reveal(bytes.NewReader(stego.Bytes()))
	if err != nil {
		t.Fatalf("Reveal: %v", err)
	}
	return got
}

func decodeWithGozxing(t *testing.T, pngBytes []byte) string {
	t.Helper()
	img, _, err := stdimage.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("decode stego png: %v", err)
	}
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		t.Fatalf("gozxing bitmap: %v", err)
	}
	res, err := gozxingqr.NewQRCodeReader().Decode(bmp, nil)
	if err != nil {
		t.Fatalf("gozxing decode: %v", err)
	}
	return res.GetText()
}

func TestFormatCodeKAT(t *testing.T) {
	cases := map[int]int{
		0:  0x5412,
		1:  0x5125,
		9:  0x72f3,
		16: 0x1689,
		31: 0x2bed,
	}
	for data, want := range cases {
		if got := formatCode(data); got != want {
			t.Fatalf("formatCode(%d): got %#x want %#x", data, got, want)
		}
	}
}

func TestCleanBlocksAreValidRSCodewords(t *testing.T) {
	for version := minSupportedVersion; version <= maxSupportedVersion; version++ {
		clean := skip2Clean(t, testCover, version)
		maskID, level, ok := parseFormat(clean)
		if !ok {
			t.Fatalf("v%d: parseFormat failed", version)
		}
		if level != ecLevelHigh {
			t.Fatalf("v%d: parsed level %d want %d", version, level, ecLevelHigh)
		}
		spec := versionTable[version]
		order := placementOrder(version, functionModules(version))
		if len(order) != spec.dataModules() {
			t.Fatalf("v%d: placement visited %d modules want %d", version, len(order), spec.dataModules())
		}
		serial := readSerial(clean, order, maskID, spec.totalCodewords())
		dataBlocks, ecBlocks, ok := spec.deinterleave(serial)
		if !ok {
			t.Fatalf("v%d: deinterleave failed", version)
		}
		for b := range dataBlocks {
			recv := append(append([]byte(nil), dataBlocks[b]...), ecBlocks[b]...)
			if !rsAllZero(rsSyndromes(recv, spec.ecPerBlock())) {
				t.Fatalf("v%d block %d: extracted codeword is not a valid RS codeword", version, b)
			}
		}
	}
}

func TestReadWriteSymmetry(t *testing.T) {
	for version := minSupportedVersion; version <= maxSupportedVersion; version++ {
		clean := skip2Clean(t, testCover, version)
		maskID, _, _ := parseFormat(clean)
		spec := versionTable[version]
		order := placementOrder(version, functionModules(version))
		serial := readSerial(clean, order, maskID, spec.totalCodewords())
		dataBlocks, ecBlocks, _ := spec.deinterleave(serial)

		rebuilt := clean.clone()
		writeSerial(rebuilt, order, maskID, spec.interleave(dataBlocks, ecBlocks))
		for y := 0; y < clean.size; y++ {
			for x := 0; x < clean.size; x++ {
				if rebuilt.grid[y][x] != clean.grid[y][x] {
					t.Fatalf("v%d: re-render differs at (%d,%d)", version, x, y)
				}
			}
		}
	}
}

func TestRoundTrip(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
	}{
		{"single byte", []byte{0x42}},
		{"embedded zeros", []byte{0x00, 0x00, 0xFF, 0x00, 0x7F}},
		{"tiny text", []byte("hi")},
		{"medium text", []byte("meet at the docks")},
		{"twenty four bytes", bytes.Repeat([]byte{0xAB}, 24)},
		{"twenty six bytes", bytes.Repeat([]byte{0x5A}, 26)},
		{"high bits", bytes.Repeat([]byte{0xFF}, 40)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := hideReveal(t, testCover, tc.payload)
			if !bytes.Equal(got, tc.payload) {
				t.Fatalf("round-trip mismatch: got %x want %x", got, tc.payload)
			}
		})
	}
}

func TestRandomBinaryRoundTrip(t *testing.T) {
	for _, size := range []int{1, 3, 12, 27, 44, 52} {
		payloadBytes := qrRandom(size, size*7+1)
		got := hideReveal(t, testCover, payloadBytes)
		if !bytes.Equal(got, payloadBytes) {
			t.Fatalf("random round-trip mismatch at size %d", size)
		}
	}
}

func TestStegoDecodesToCoverViaGozxing(t *testing.T) {
	covers := []string{"crypha", "https://angelamos.com", "HELLO WORLD 123"}
	for _, cover := range covers {
		var stego bytes.Buffer
		if err := (qrCarrier{}).Hide(strings.NewReader(cover), qrRandom(16, len(cover)), &stego); err != nil {
			t.Fatalf("Hide cover %q: %v", cover, err)
		}
		if got := decodeWithGozxing(t, stego.Bytes()); got != cover {
			t.Fatalf("gozxing decoded stego to %q, want cover %q", got, cover)
		}
	}
}

func TestPerVersionMaxCapacityRoundTripAndScan(t *testing.T) {
	for version := minSupportedVersion; version <= maxSupportedVersion; version++ {
		capBytes := versionTable[version].capacityBytes()
		if capBytes == 0 {
			continue
		}
		payloadBytes := qrRandom(capBytes, version*101+7)
		var stego bytes.Buffer
		if err := (qrCarrier{}).Hide(strings.NewReader(testCover), payloadBytes, &stego); err != nil {
			t.Fatalf("v%d Hide at capacity %d: %v", version, capBytes, err)
		}
		got, err := (qrCarrier{}).Reveal(bytes.NewReader(stego.Bytes()))
		if err != nil {
			t.Fatalf("v%d Reveal: %v", version, err)
		}
		if !bytes.Equal(got, payloadBytes) {
			t.Fatalf("v%d: round-trip mismatch at max capacity", version)
		}
		if scanned := decodeWithGozxing(t, stego.Bytes()); scanned != testCover {
			t.Fatalf("v%d: at full injection budget, gozxing decoded %q want cover %q", version, scanned, testCover)
		}
	}
}

func TestCleanQRHasNoPayload(t *testing.T) {
	clean := skip2Clean(t, testCover, 6)
	var buf bytes.Buffer
	if err := renderPNG(clean, &buf); err != nil {
		t.Fatalf("render clean: %v", err)
	}
	if _, err := (qrCarrier{}).Reveal(bytes.NewReader(buf.Bytes())); err != ErrNoPayload {
		t.Fatalf("clean QR reveal: got %v want ErrNoPayload", err)
	}
}

func TestCapacityBoundary(t *testing.T) {
	atCap := bytes.Repeat([]byte{0x01}, maxCapacity())
	if got := hideReveal(t, testCover, atCap); !bytes.Equal(got, atCap) {
		t.Fatal("payload at exact capacity failed to round-trip")
	}
	over := bytes.Repeat([]byte{0x01}, maxCapacity()+1)
	err := (qrCarrier{}).Hide(strings.NewReader(testCover), over, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected capacity error for oversized payload")
	}
}

func TestCapacityReport(t *testing.T) {
	got, err := (qrCarrier{}).Capacity(strings.NewReader(testCover))
	if err != nil {
		t.Fatalf("Capacity: %v", err)
	}
	if got != maxCapacity() {
		t.Fatalf("Capacity: got %d want %d", got, maxCapacity())
	}
}

func TestCapacityTooLargeCoverIsZero(t *testing.T) {
	large := strings.Repeat("a", 200)
	got, err := (qrCarrier{}).Capacity(strings.NewReader(large))
	if err != nil {
		t.Fatalf("Capacity: %v", err)
	}
	if got != 0 {
		t.Fatalf("Capacity for a cover too large for any version: got %d want 0", got)
	}
}

func TestEmptyPayloadRejected(t *testing.T) {
	if err := (qrCarrier{}).Hide(strings.NewReader(testCover), nil, &bytes.Buffer{}); err != ErrEmptyPayload {
		t.Fatalf("expected ErrEmptyPayload, got %v", err)
	}
}

func TestEmptyCoverRejected(t *testing.T) {
	if err := (qrCarrier{}).Hide(strings.NewReader(""), []byte("x"), &bytes.Buffer{}); err != ErrCoverRequired {
		t.Fatalf("expected ErrCoverRequired, got %v", err)
	}
}

func TestCoverTooLargeRejected(t *testing.T) {
	huge := strings.Repeat("A", 4000)
	err := (qrCarrier{}).Hide(strings.NewReader(huge), []byte("x"), &bytes.Buffer{})
	if err != ErrCoverTooLarge {
		t.Fatalf("expected ErrCoverTooLarge, got %v", err)
	}
}

func TestEncryptedEnvelopeExceedsQRCapacity(t *testing.T) {
	env, err := payload.Pack([]byte("secret"), payload.Options{
		Passphrase: []byte("correct horse battery staple"),
		Cipher:     payload.CipherChaCha20,
		Strength:   payload.StrengthDefault,
	})
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if len(env) <= maxCapacity() {
		t.Fatalf("encrypted envelope is %d bytes, expected to exceed qr capacity %d", len(env), maxCapacity())
	}
	err = (qrCarrier{}).Hide(strings.NewReader(testCover), env, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected oversized encrypted envelope to be rejected")
	}
}

func TestUnencryptedEnvelopeThroughCarrier(t *testing.T) {
	secret := []byte("qr covert")
	env, err := payload.Pack(secret, payload.Options{Compress: true})
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	got := hideReveal(t, testCover, env)
	if !bytes.Equal(got, env) {
		t.Fatal("carrier did not return the exact envelope bytes")
	}
	plain, err := payload.Unpack(got, nil)
	if err != nil {
		t.Fatalf("Unpack: %v", err)
	}
	if !bytes.Equal(plain, secret) {
		t.Fatalf("end-to-end mismatch: got %q want %q", plain, secret)
	}
}

func TestSniff(t *testing.T) {
	var stego bytes.Buffer
	if err := (qrCarrier{}).Hide(strings.NewReader(testCover), []byte("payload"), &stego); err != nil {
		t.Fatalf("Hide: %v", err)
	}

	plain := stdimage.NewGray(stdimage.Rect(0, 0, 200, 200))
	for i := range plain.Pix {
		plain.Pix[i] = 0xFF
	}
	var notQR bytes.Buffer
	if err := png.Encode(&notQR, plain); err != nil {
		t.Fatalf("encode plain png: %v", err)
	}

	cases := []struct {
		name string
		data []byte
		want bool
	}{
		{"stego qr", stego.Bytes(), true},
		{"blank png", notQR.Bytes(), false},
		{"garbage", []byte("not an image at all"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (qrCarrier{}).Sniff(bytes.NewReader(tc.data)); got != tc.want {
				t.Fatalf("Sniff(%s): got %v want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestRevealRejectsGarbage(t *testing.T) {
	if _, err := (qrCarrier{}).Reveal(bytes.NewReader([]byte("garbage"))); err == nil {
		t.Fatal("expected decode error for garbage input")
	}
	plain := stdimage.NewGray(stdimage.Rect(0, 0, 200, 200))
	for i := range plain.Pix {
		plain.Pix[i] = 0xFF
	}
	var blank bytes.Buffer
	if err := png.Encode(&blank, plain); err != nil {
		t.Fatalf("encode blank: %v", err)
	}
	if _, err := (qrCarrier{}).Reveal(bytes.NewReader(blank.Bytes())); err == nil {
		t.Fatal("blank png reveal: expected an error, got nil")
	}
}

func TestRegisteredInRegistry(t *testing.T) {
	c, ok := carrier.Get(Format)
	if !ok {
		t.Fatal("qr carrier did not self-register")
	}
	if c.Format() != Format {
		t.Fatalf("registry returned wrong carrier: %s", c.Format())
	}
}

func TestReadErrorsPropagate(t *testing.T) {
	if err := (qrCarrier{}).Hide(errReader{}, []byte("x"), &bytes.Buffer{}); err == nil {
		t.Fatal("Hide: expected cover read error")
	}
	if _, err := (qrCarrier{}).Capacity(errReader{}); err == nil {
		t.Fatal("Capacity: expected cover read error")
	}
	if _, err := (qrCarrier{}).Reveal(errReader{}); err == nil {
		t.Fatal("Reveal: expected stego read error")
	}
}

func TestUnframeRejectsBadLength(t *testing.T) {
	if _, err := unframe([]byte{0, 1}); err != ErrNoPayload {
		t.Fatalf("short frame: got %v want ErrNoPayload", err)
	}
	if _, err := unframe([]byte{0, 0, 0, 0}); err != ErrNoPayload {
		t.Fatalf("zero-length frame: got %v want ErrNoPayload", err)
	}
	if _, err := unframe([]byte{0, 0, 0, 10, 1, 2, 3}); err != ErrNoPayload {
		t.Fatalf("overlong length prefix: got %v want ErrNoPayload", err)
	}
}

func TestMatrixFromBitmapRejectsWrongSize(t *testing.T) {
	tooSmall := make([][]bool, 10)
	if _, err := matrixFromBitmap(tooSmall, 1); err != errBadSymbol {
		t.Fatalf("wrong height: got %v want errBadSymbol", err)
	}
	raggedRows := make([][]bool, symbolSize(1))
	for i := range raggedRows {
		raggedRows[i] = make([]bool, 5)
	}
	if _, err := matrixFromBitmap(raggedRows, 1); err != errBadSymbol {
		t.Fatalf("wrong width: got %v want errBadSymbol", err)
	}
}

func TestInjectFramedRejectsOverflow(t *testing.T) {
	spec := versionTable[2]
	dataBlocks := [][]byte{make([]byte, spec.groups[0].data)}
	oversized := make([]byte, spec.numBlocks()*spec.injectPerBlock()+1)
	if err := injectFramed(dataBlocks, spec, oversized); !errors.Is(err, ErrPayloadTooLarge) {
		t.Fatalf("overflow inject: got %v want ErrPayloadTooLarge", err)
	}
}

func TestSniffRejectsNonFinderImage(t *testing.T) {
	size := symbolSize(1)
	dim := (size + 2*quietZoneModules) * modulePixels
	white := stdimage.NewGray(stdimage.Rect(0, 0, dim, dim))
	for i := range white.Pix {
		white.Pix[i] = 0xFF
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, white); err != nil {
		t.Fatalf("encode white qr-sized png: %v", err)
	}
	if (qrCarrier{}).Sniff(bytes.NewReader(buf.Bytes())) {
		t.Fatal("Sniff should reject a qr-sized image with no finder patterns")
	}
}
