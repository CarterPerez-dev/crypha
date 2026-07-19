/*
©AngelaMos | 2026
pdf_test.go

Per-technique round-trip, auto-detect, capacity, ordering, and sniff tests for the PDF carrier
*/

package pdf

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/CarterPerez-dev/crypha/internal/carrier"
	"github.com/CarterPerez-dev/crypha/internal/payload"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

const pageJSON = `{"pages": {"1": {"content": {}}}}`

var allTechniques = []Technique{TechniqueAttachment, TechniqueMetadata, TechniqueAppend}

func minimalPDF(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := api.Create(nil, strings.NewReader(pageJSON), &buf, newConfig()); err != nil {
		t.Fatalf("create demo pdf: %v", err)
	}
	return buf.Bytes()
}

func pseudoBytes(n, seed int) []byte {
	b := make([]byte, n)
	x := uint32(seed)*2654435761 + 1
	for i := range b {
		x = x*1664525 + 1013904223
		b[i] = byte(x >> 24)
	}
	return b
}

func hideWith(t *testing.T, tech Technique, cover, payload []byte) []byte {
	t.Helper()
	var stego bytes.Buffer
	if err := New(tech).Hide(bytes.NewReader(cover), payload, &stego); err != nil {
		t.Fatalf("Hide(%s): %v", tech, err)
	}
	return stego.Bytes()
}

func TestFixtureIsValidPDF(t *testing.T) {
	cover := minimalPDF(t)
	if !isPDF(cover) {
		t.Fatal("fixture is not a PDF")
	}
}

func TestRoundTripTechniques(t *testing.T) {
	cover := minimalPDF(t)
	payloads := []struct {
		name string
		data []byte
	}{
		{"short text", []byte("meet at the docks")},
		{"single byte", []byte{0x42}},
		{"binary blob", pseudoBytes(500, 7)},
	}
	for _, tech := range allTechniques {
		for _, pl := range payloads {
			t.Run(string(tech)+"/"+pl.name, func(t *testing.T) {
				stego := hideWith(t, tech, cover, pl.data)
				got, err := New(tech).Reveal(bytes.NewReader(stego))
				if err != nil {
					t.Fatalf("Reveal(%s): %v", tech, err)
				}
				if !bytes.Equal(got, pl.data) {
					t.Fatalf("%s round-trip mismatch: got %x want %x", tech, got, pl.data)
				}
			})
		}
	}
}

func TestRevealAutoDetect(t *testing.T) {
	cover := minimalPDF(t)
	detector, ok := carrier.Get(Format)
	if !ok {
		t.Fatal("pdf carrier not registered")
	}
	for _, tech := range allTechniques {
		t.Run(string(tech), func(t *testing.T) {
			payload := []byte("auto-detect me: " + string(tech))
			stego := hideWith(t, tech, cover, payload)
			got, err := detector.Reveal(bytes.NewReader(stego))
			if err != nil {
				t.Fatalf("auto-detect Reveal for %s: %v", tech, err)
			}
			if !bytes.Equal(got, payload) {
				t.Fatalf("auto-detect mismatch for %s: got %q want %q", tech, got, payload)
			}
		})
	}
}

func TestMetadataChunking(t *testing.T) {
	cover := minimalPDF(t)
	payload := pseudoBytes(metaChunkSize, 3)

	stego := hideWith(t, TechniqueMetadata, cover, payload)
	got, err := New(TechniqueMetadata).Reveal(bytes.NewReader(stego))
	if err != nil {
		t.Fatalf("Reveal: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("multi-chunk metadata round-trip mismatch")
	}
}

func TestAppendStrippedByPdfcpuRewrite(t *testing.T) {
	cover := minimalPDF(t)
	appendStego := hideWith(t, TechniqueAppend, cover, []byte("fragile append payload"))
	if _, ok := revealAppend(appendStego); !ok {
		t.Fatal("append payload not present before rewrite")
	}

	rewritten := hideWith(t, TechniqueAttachment, appendStego, []byte("new attachment"))
	if _, ok := revealAppend(rewritten); ok {
		t.Fatal("expected pdfcpu rewrite to strip the trailing append payload")
	}
	if _, ok := revealAttachment(rewritten); !ok {
		t.Fatal("attachment payload should survive the rewrite")
	}
}

func TestCapacityUnbounded(t *testing.T) {
	cover := minimalPDF(t)
	got, err := New(TechniqueAttachment).Capacity(bytes.NewReader(cover))
	if err != nil {
		t.Fatalf("Capacity: %v", err)
	}
	if got != unboundedCapacity {
		t.Fatalf("Capacity: got %d want %d", got, unboundedCapacity)
	}
}

func TestSniff(t *testing.T) {
	cover := minimalPDF(t)
	cases := []struct {
		name string
		data []byte
		want bool
	}{
		{"pdf", cover, true},
		{"random", []byte("not a pdf at all, just text"), false},
		{"short", []byte("%PD"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := New(TechniqueAttachment).Sniff(bytes.NewReader(tc.data)); got != tc.want {
				t.Fatalf("Sniff(%s): got %v want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestRejectNonPDF(t *testing.T) {
	garbage := []byte("this is plainly not a pdf document")
	if err := New(TechniqueAttachment).Hide(bytes.NewReader(garbage), []byte("x"), &bytes.Buffer{}); err != ErrUnsupportedFormat {
		t.Fatalf("Hide non-pdf: got %v want ErrUnsupportedFormat", err)
	}
	if _, err := New(TechniqueAttachment).Reveal(bytes.NewReader(garbage)); err != ErrUnsupportedFormat {
		t.Fatalf("Reveal non-pdf: got %v want ErrUnsupportedFormat", err)
	}
	if _, err := New(TechniqueAttachment).Capacity(bytes.NewReader(garbage)); err != ErrUnsupportedFormat {
		t.Fatalf("Capacity non-pdf: got %v want ErrUnsupportedFormat", err)
	}
}

func TestEmptyPayloadRejected(t *testing.T) {
	cover := minimalPDF(t)
	if err := New(TechniqueAttachment).Hide(bytes.NewReader(cover), nil, &bytes.Buffer{}); err != ErrEmptyPayload {
		t.Fatalf("expected ErrEmptyPayload, got %v", err)
	}
}

func TestRevealNoPayload(t *testing.T) {
	cover := minimalPDF(t)
	if _, err := New(TechniqueAttachment).Reveal(bytes.NewReader(cover)); err != ErrNoPayload {
		t.Fatalf("expected ErrNoPayload on a clean PDF, got %v", err)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, errors.New("forced read failure")
}

func TestReadErrorsPropagate(t *testing.T) {
	if err := New(TechniqueAttachment).Hide(errReader{}, []byte("x"), &bytes.Buffer{}); err == nil {
		t.Fatal("Hide: expected read error")
	}
	if _, err := New(TechniqueAttachment).Reveal(errReader{}); err == nil {
		t.Fatal("Reveal: expected read error")
	}
	if _, err := New(TechniqueAttachment).Capacity(errReader{}); err == nil {
		t.Fatal("Capacity: expected read error")
	}
}

func TestEncryptedEnvelopeThroughCarrier(t *testing.T) {
	cover := minimalPDF(t)
	secret := []byte("the account number is 4417 1234 5678 9012")
	envelope, err := payload.Pack(secret, payload.Options{
		Passphrase: []byte("correct horse battery staple"),
		Compress:   true,
		Cipher:     payload.CipherChaCha20,
		Strength:   payload.StrengthDefault,
	})
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}

	for _, tech := range allTechniques {
		t.Run(string(tech), func(t *testing.T) {
			stego := hideWith(t, tech, cover, envelope)
			recovered, err := New(tech).Reveal(bytes.NewReader(stego))
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
		})
	}
}

func TestCorruptPDFRejectedByTechniques(t *testing.T) {
	corrupt := []byte("%PDF-1.7\nnot a real pdf body, no xref, no objects\n%%EOF")
	if err := New(TechniqueAttachment).Hide(bytes.NewReader(corrupt), []byte("x"), &bytes.Buffer{}); err == nil {
		t.Fatal("Hide attachment on corrupt PDF: expected error")
	}
	if err := New(TechniqueMetadata).Hide(bytes.NewReader(corrupt), []byte("x"), &bytes.Buffer{}); err == nil {
		t.Fatal("Hide metadata on corrupt PDF: expected error")
	}
	if _, err := New(TechniqueAttachment).Reveal(bytes.NewReader(corrupt)); err != ErrNoPayload {
		t.Fatalf("Reveal corrupt PDF: got %v want ErrNoPayload", err)
	}
}

func TestForeignAttachmentIgnored(t *testing.T) {
	cover := minimalPDF(t)
	conf := newConfig()
	ctx, err := api.ReadValidateAndOptimize(bytes.NewReader(cover), conf)
	if err != nil {
		t.Fatalf("read cover: %v", err)
	}
	other := model.Attachment{
		Reader:   bytes.NewReader([]byte("someone else's file")),
		ID:       "other.txt",
		FileName: "other.txt",
		ModTime:  &epoch,
	}
	if err := ctx.AddAttachment(other, false); err != nil {
		t.Fatalf("add foreign attachment: %v", err)
	}
	var buf bytes.Buffer
	if err := api.Write(ctx, &buf, conf); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, ok := revealAttachment(buf.Bytes()); ok {
		t.Fatal("revealAttachment must ignore a non-crypha attachment")
	}
}

type errWriter struct {
	failAt int
	count  int
}

func (w *errWriter) Write(p []byte) (int, error) {
	w.count++
	if w.count > w.failAt {
		return 0, errors.New("forced write failure")
	}
	return len(p), nil
}

func TestAppendWriteErrorsPropagate(t *testing.T) {
	cover := minimalPDF(t)
	for failAt := 0; failAt < 4; failAt++ {
		if err := hideAppend(cover, []byte("payload"), &errWriter{failAt: failAt}); err == nil {
			t.Fatalf("hideAppend failAt=%d: expected write error", failAt)
		}
	}
}

func TestAttachmentWriteErrorPropagates(t *testing.T) {
	cover := minimalPDF(t)
	if err := hideAttachment(cover, []byte("payload"), &errWriter{failAt: 0}); err == nil {
		t.Fatal("hideAttachment: expected write error")
	}
}

func metaStego(t *testing.T, cover []byte, props map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := api.AddProperties(bytes.NewReader(cover), &buf, props, newConfig()); err != nil {
		t.Fatalf("add properties: %v", err)
	}
	return buf.Bytes()
}

func TestMetadataTamperedRejected(t *testing.T) {
	cover := minimalPDF(t)
	cases := []struct {
		name  string
		props map[string]string
	}{
		{"non-numeric count", map[string]string{metaCountKey: "not-a-number"}},
		{"zero count", map[string]string{metaCountKey: "0"}},
		{"missing chunk", map[string]string{metaCountKey: "1"}},
		{"invalid base64", map[string]string{metaCountKey: "1", metaKeyPrefix + "0": "@@@not-base64@@@"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stego := metaStego(t, cover, tc.props)
			if _, ok := revealMetadata(stego); ok {
				t.Fatalf("revealMetadata must reject %s", tc.name)
			}
		})
	}
}

func TestEmptyMetadataYieldsNoPayload(t *testing.T) {
	cover := minimalPDF(t)
	stego := metaStego(t, cover, map[string]string{metaCountKey: "0"})
	if _, err := New(TechniqueAttachment).Reveal(bytes.NewReader(stego)); err != ErrNoPayload {
		t.Fatalf("count=0 metadata: got %v want ErrNoPayload", err)
	}
}

func TestEmptyMetadataDoesNotMaskAppend(t *testing.T) {
	cover := minimalPDF(t)
	secret := []byte("the real secret rides under a spurious empty-metadata key")
	metaPart := metaStego(t, cover, map[string]string{metaCountKey: "0"})

	var stego bytes.Buffer
	if err := hideAppend(metaPart, secret, &stego); err != nil {
		t.Fatalf("hideAppend: %v", err)
	}
	got, err := New(TechniqueAttachment).Reveal(bytes.NewReader(stego.Bytes()))
	if err != nil {
		t.Fatalf("Reveal: %v", err)
	}
	if !bytes.Equal(got, secret) {
		t.Fatalf("empty metadata masked the append payload: got %q", got)
	}
}

func TestRevealAppendEdgeCases(t *testing.T) {
	if _, ok := revealAppend([]byte("tiny")); ok {
		t.Fatal("revealAppend on too-short input must return false")
	}

	cover := minimalPDF(t)
	payload := []byte("magic mismatch probe")
	stego := hideWith(t, TechniqueAppend, cover, payload)
	magicPos := len(stego) - lengthPrefixBytes - len(payload) - len(appendMagic)
	corrupt := append([]byte{}, stego...)
	corrupt[magicPos] ^= 0xFF
	if _, ok := revealAppend(corrupt); ok {
		t.Fatal("revealAppend must reject a mismatched magic")
	}
}

func TestRegisteredInRegistry(t *testing.T) {
	c, ok := carrier.Get(Format)
	if !ok {
		t.Fatal("pdf carrier did not self-register")
	}
	if c.Format() != Format {
		t.Fatalf("registry returned wrong carrier: %s", c.Format())
	}
}
