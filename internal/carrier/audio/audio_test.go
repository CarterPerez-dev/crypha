/*
©AngelaMos | 2026
audio_test.go

Round-trip, FLAC-in, rejection, capacity, and sniff tests for the LSB audio carrier
*/

package audio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"

	"github.com/CarterPerez-dev/crypha/internal/carrier"
	"github.com/CarterPerez-dev/crypha/internal/payload"
	goaudio "github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/mewkiz/flac"
	flacframe "github.com/mewkiz/flac/frame"
	"github.com/mewkiz/flac/meta"
)

const (
	sampleRate   = 44100
	coverSlots   = 1000
	coverBytes   = (coverSlots - lengthPrefixBits) / bitsPerByte
	roomySlots   = 8192
	rejectDepth  = 8
	minFLACBlock = 16
)

func pseudoSamples(n, seed int) []int {
	out := make([]int, n)
	x := uint32(seed)*2654435761 + 1
	for i := range out {
		x = x*1664525 + 1013904223
		out[i] = int(int16(x >> 16))
	}
	return out
}

func synthWAV(t *testing.T, samples []int, chans int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := encodeWAV(&buf, pcm{samples: samples, numChannels: chans, sampleRate: sampleRate}); err != nil {
		t.Fatalf("synth wav: %v", err)
	}
	return buf.Bytes()
}

func flacChannels(chans int) flacframe.Channels {
	if chans == 1 {
		return flacframe.ChannelsMono
	}
	return flacframe.ChannelsLR
}

func synthFLAC(t *testing.T, samples []int, chans, rate, bps int) []byte {
	t.Helper()
	perChan := len(samples) / chans
	declaredBlock := perChan
	if declaredBlock < minFLACBlock {
		declaredBlock = minFLACBlock
	}
	info := &meta.StreamInfo{
		SampleRate:    uint32(rate),
		NChannels:     uint8(chans),
		BitsPerSample: uint8(bps),
		NSamples:      uint64(perChan),
		BlockSizeMin:  uint16(declaredBlock),
		BlockSizeMax:  uint16(declaredBlock),
	}
	var out bytes.Buffer
	enc, err := flac.NewEncoder(&out, info)
	if err != nil {
		t.Fatalf("flac new encoder: %v", err)
	}
	enc.EnablePredictionAnalysis(false)

	if perChan > 0 {
		subs := make([]*flacframe.Subframe, chans)
		for ch := 0; ch < chans; ch++ {
			s := make([]int32, perChan)
			for i := 0; i < perChan; i++ {
				s[i] = int32(samples[i*chans+ch])
			}
			subs[ch] = &flacframe.Subframe{
				SubHeader: flacframe.SubHeader{Pred: flacframe.PredVerbatim},
				Samples:   s,
				NSamples:  perChan,
			}
		}
		f := &flacframe.Frame{
			Header: flacframe.Header{
				HasFixedBlockSize: true,
				BlockSize:         uint16(perChan),
				SampleRate:        uint32(rate),
				Channels:          flacChannels(chans),
				BitsPerSample:     uint8(bps),
			},
			Subframes: subs,
		}
		if err := enc.WriteFrame(f); err != nil {
			t.Fatalf("flac write frame: %v", err)
		}
	}
	if err := enc.Close(); err != nil {
		t.Fatalf("flac close: %v", err)
	}
	return out.Bytes()
}

func hideReveal(t *testing.T, cover, secret []byte) []byte {
	t.Helper()
	var stego bytes.Buffer
	if err := (audioCarrier{}).Hide(bytes.NewReader(cover), secret, &stego); err != nil {
		t.Fatalf("Hide: %v", err)
	}
	got, err := (audioCarrier{}).Reveal(bytes.NewReader(stego.Bytes()))
	if err != nil {
		t.Fatalf("Reveal: %v", err)
	}
	return got
}

func TestRoundTripWAV(t *testing.T) {
	cases := []struct {
		name    string
		chans   int
		payload []byte
	}{
		{"mono single byte", 1, []byte{0x42}},
		{"mono text", 1, []byte("crypha audio")},
		{"stereo text", 2, []byte("left and right channels")},
		{"high bits set", 1, bytes.Repeat([]byte{0xFF}, 40)},
		{"binary blob", 2, pseudoBytes(300, 9)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cover := synthWAV(t, pseudoSamples(roomySlots, 1), tc.chans)
			got := hideReveal(t, cover, tc.payload)
			if !bytes.Equal(got, tc.payload) {
				t.Fatalf("round-trip mismatch: got %x want %x", got, tc.payload)
			}
		})
	}
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

func TestFLACInWAVOut(t *testing.T) {
	cases := []struct {
		name    string
		chans   int
		payload []byte
	}{
		{"mono", 1, []byte("decoded from flac, embedded, emitted as wav")},
		{"stereo", 2, pseudoBytes(200, 3)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cover := synthFLAC(t, pseudoSamples(roomySlots, 2), tc.chans, sampleRate, supportedBitDepth)

			var stego bytes.Buffer
			if err := (audioCarrier{}).Hide(bytes.NewReader(cover), tc.payload, &stego); err != nil {
				t.Fatalf("Hide flac cover: %v", err)
			}
			if !isWAV(stego.Bytes()) {
				t.Fatal("flac cover did not produce a WAV stego output")
			}
			got, err := (audioCarrier{}).Reveal(bytes.NewReader(stego.Bytes()))
			if err != nil {
				t.Fatalf("Reveal: %v", err)
			}
			if !bytes.Equal(got, tc.payload) {
				t.Fatalf("flac-in round-trip mismatch: got %x want %x", got, tc.payload)
			}
		})
	}
}

func TestCapacityBoundary(t *testing.T) {
	cover := synthWAV(t, pseudoSamples(coverSlots, 5), 1)

	atCap := bytes.Repeat([]byte{0x01}, coverBytes)
	if got := hideReveal(t, cover, atCap); !bytes.Equal(got, atCap) {
		t.Fatal("payload at exact capacity failed to round-trip")
	}

	overCap := bytes.Repeat([]byte{0x01}, coverBytes+1)
	err := (audioCarrier{}).Hide(bytes.NewReader(cover), overCap, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected capacity error for oversized payload")
	}
}

func TestCapacityReport(t *testing.T) {
	wavCover := synthWAV(t, pseudoSamples(coverSlots, 7), 1)
	got, err := (audioCarrier{}).Capacity(bytes.NewReader(wavCover))
	if err != nil {
		t.Fatalf("Capacity wav: %v", err)
	}
	if got != coverBytes {
		t.Fatalf("Capacity wav: got %d want %d", got, coverBytes)
	}

	flacCover := synthFLAC(t, pseudoSamples(coverSlots*2, 8), 2, sampleRate, supportedBitDepth)
	gotFlac, err := (audioCarrier{}).Capacity(bytes.NewReader(flacCover))
	if err != nil {
		t.Fatalf("Capacity flac: %v", err)
	}
	if want := capacityFromSlots(coverSlots * 2); gotFlac != want {
		t.Fatalf("Capacity flac: got %d want %d", gotFlac, want)
	}
}

func synthWAVCustom(t *testing.T, samples []int, chans, bitDepth, audioFormat int) []byte {
	t.Helper()
	ws := &memWriteSeeker{}
	enc := wav.NewEncoder(ws, sampleRate, bitDepth, chans, audioFormat)
	buf := &goaudio.IntBuffer{
		Format:         &goaudio.Format{NumChannels: chans, SampleRate: sampleRate},
		Data:           samples,
		SourceBitDepth: bitDepth,
	}
	if err := enc.Write(buf); err != nil {
		t.Fatalf("synth custom wav write: %v", err)
	}
	if err := enc.Close(); err != nil {
		t.Fatalf("synth custom wav close: %v", err)
	}
	return ws.buf
}

func TestHideRejectsNonPCMWAV(t *testing.T) {
	cover := synthWAVCustom(t, pseudoSamples(64, 1), 1, supportedBitDepth, 3)
	err := (audioCarrier{}).Hide(bytes.NewReader(cover), []byte("x"), &bytes.Buffer{})
	if err != ErrNotPCM {
		t.Fatalf("expected ErrNotPCM, got %v", err)
	}
}

func TestHideRejectsWrongBitDepthWAV(t *testing.T) {
	cover := synthWAVCustom(t, pseudoSamples(64, 1), 1, rejectDepth, wavFormatPCM)
	err := (audioCarrier{}).Hide(bytes.NewReader(cover), []byte("x"), &bytes.Buffer{})
	if err != ErrUnsupportedBitDepth {
		t.Fatalf("expected ErrUnsupportedBitDepth, got %v", err)
	}
}

func TestHideRejectsWrongBitDepthFLAC(t *testing.T) {
	cover := synthFLAC(t, pseudoSamples(64, 4), 1, sampleRate, rejectDepth)
	err := (audioCarrier{}).Hide(bytes.NewReader(cover), []byte("x"), &bytes.Buffer{})
	if err != ErrUnsupportedBitDepth {
		t.Fatalf("expected ErrUnsupportedBitDepth, got %v", err)
	}
}

func TestHideRejectsUnsupportedFormat(t *testing.T) {
	garbage := []byte("this is not audio at all, just prose")
	if err := (audioCarrier{}).Hide(bytes.NewReader(garbage), []byte("x"), &bytes.Buffer{}); err != ErrUnsupportedFormat {
		t.Fatalf("Hide garbage: got %v want ErrUnsupportedFormat", err)
	}
	if _, err := (audioCarrier{}).Capacity(bytes.NewReader(garbage)); err != ErrUnsupportedFormat {
		t.Fatalf("Capacity garbage: got %v want ErrUnsupportedFormat", err)
	}
}

func TestHideEmptyPayloadRejected(t *testing.T) {
	cover := synthWAV(t, pseudoSamples(coverSlots, 1), 1)
	if err := (audioCarrier{}).Hide(bytes.NewReader(cover), nil, &bytes.Buffer{}); err != ErrEmptyPayload {
		t.Fatalf("expected ErrEmptyPayload, got %v", err)
	}
}

func TestRevealRejectsNonWAV(t *testing.T) {
	flacData := synthFLAC(t, pseudoSamples(64, 2), 1, sampleRate, supportedBitDepth)
	if _, err := (audioCarrier{}).Reveal(bytes.NewReader(flacData)); err != ErrUnsupportedFormat {
		t.Fatalf("Reveal flac: got %v want ErrUnsupportedFormat", err)
	}
	if _, err := (audioCarrier{}).Reveal(bytes.NewReader([]byte("garbage"))); err != ErrUnsupportedFormat {
		t.Fatalf("Reveal garbage: got %v want ErrUnsupportedFormat", err)
	}
}

func TestRevealRejectsUndecodableWAV(t *testing.T) {
	cover := synthWAVCustom(t, pseudoSamples(64, 1), 1, supportedBitDepth, 3)
	if _, err := (audioCarrier{}).Reveal(bytes.NewReader(cover)); err != ErrNotPCM {
		t.Fatalf("Reveal float WAV: got %v want ErrNotPCM", err)
	}
}

func TestDecodeWAVTruncatedFmt(t *testing.T) {
	var b bytes.Buffer
	b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(0xFFFFFFFF))
	b.WriteString("WAVE")
	b.WriteString("fmt ")
	_ = binary.Write(&b, binary.LittleEndian, uint32(16))
	if err := (audioCarrier{}).Hide(bytes.NewReader(b.Bytes()), []byte("x"), &bytes.Buffer{}); err == nil {
		t.Fatal("expected decode error on a truncated fmt chunk")
	}
}

func TestRevealNoPayload(t *testing.T) {
	clean := make([]int, coverSlots)
	cover := synthWAV(t, clean, 1)
	if _, err := (audioCarrier{}).Reveal(bytes.NewReader(cover)); err != ErrNoPayload {
		t.Fatalf("expected ErrNoPayload on zeroed cover, got %v", err)
	}
}

func TestRevealTooSmall(t *testing.T) {
	cover := synthWAV(t, pseudoSamples(16, 1), 1)
	if _, err := (audioCarrier{}).Reveal(bytes.NewReader(cover)); err != ErrTooSmall {
		t.Fatalf("expected ErrTooSmall, got %v", err)
	}
}

func TestSniff(t *testing.T) {
	wavCover := synthWAV(t, pseudoSamples(coverSlots, 1), 1)
	flacCover := synthFLAC(t, pseudoSamples(64, 1), 1, sampleRate, supportedBitDepth)
	cases := []struct {
		name string
		data []byte
		want bool
	}{
		{"wav", wavCover, true},
		{"flac not stego", flacCover, false},
		{"random", []byte("not audio, definitely"), false},
		{"short", []byte("RIFF"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (audioCarrier{}).Sniff(bytes.NewReader(tc.data)); got != tc.want {
				t.Fatalf("Sniff(%s): got %v want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestEncryptedEnvelopeThroughCarrier(t *testing.T) {
	secret := []byte("the drop is behind the third locker")
	envelope, err := payload.Pack(secret, payload.Options{
		Passphrase: []byte("correct horse battery staple"),
		Compress:   true,
		Cipher:     payload.CipherChaCha20,
		Strength:   payload.StrengthDefault,
	})
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}

	cover := synthWAV(t, pseudoSamples(roomySlots, 4), 2)
	var stego bytes.Buffer
	if err := (audioCarrier{}).Hide(bytes.NewReader(cover), envelope, &stego); err != nil {
		t.Fatalf("Hide envelope: %v", err)
	}

	recovered, err := (audioCarrier{}).Reveal(bytes.NewReader(stego.Bytes()))
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

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, errors.New("forced read failure")
}

func TestReadErrorsPropagate(t *testing.T) {
	if err := (audioCarrier{}).Hide(errReader{}, []byte("x"), &bytes.Buffer{}); err == nil {
		t.Fatal("Hide: expected read error")
	}
	if _, err := (audioCarrier{}).Reveal(errReader{}); err == nil {
		t.Fatal("Reveal: expected read error")
	}
	if _, err := (audioCarrier{}).Capacity(errReader{}); err == nil {
		t.Fatal("Capacity: expected read error")
	}
}

func TestDecodeWAVZeroSamples(t *testing.T) {
	cover := synthWAV(t, nil, 1)
	if err := (audioCarrier{}).Hide(bytes.NewReader(cover), []byte("x"), &bytes.Buffer{}); err != ErrNoSamples {
		t.Fatalf("expected ErrNoSamples on empty WAV, got %v", err)
	}
}

func TestDecodeWAVMalformedHeader(t *testing.T) {
	cover := []byte("RIFF\x04\x00\x00\x00WAVE")
	err := (audioCarrier{}).Hide(bytes.NewReader(cover), []byte("x"), &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected decode error on a RIFF/WAVE file with no fmt chunk")
	}
}

func TestDecodeFLACZeroFrames(t *testing.T) {
	cover := synthFLAC(t, nil, 1, sampleRate, supportedBitDepth)
	if err := (audioCarrier{}).Hide(bytes.NewReader(cover), []byte("x"), &bytes.Buffer{}); err != ErrNoSamples {
		t.Fatalf("expected ErrNoSamples on frameless FLAC, got %v", err)
	}
}

func TestDecodeFLACGarbage(t *testing.T) {
	cover := append([]byte("fLaC"), pseudoBytes(64, 11)...)
	if err := (audioCarrier{}).Hide(bytes.NewReader(cover), []byte("x"), &bytes.Buffer{}); err == nil {
		t.Fatal("expected decode error on fLaC-tagged garbage")
	}
}

func TestDecodeFLACTruncatedFrame(t *testing.T) {
	full := synthFLAC(t, pseudoSamples(roomySlots, 6), 1, sampleRate, supportedBitDepth)
	truncated := full[:len(full)-16]
	if err := (audioCarrier{}).Hide(bytes.NewReader(truncated), []byte("x"), &bytes.Buffer{}); err == nil {
		t.Fatal("expected decode error on a FLAC truncated mid-frame")
	}
}

func TestCapacityTinyCoverIsZero(t *testing.T) {
	cover := synthWAV(t, pseudoSamples(16, 1), 1)
	got, err := (audioCarrier{}).Capacity(bytes.NewReader(cover))
	if err != nil {
		t.Fatalf("Capacity tiny: %v", err)
	}
	if got != 0 {
		t.Fatalf("Capacity tiny: got %d want 0", got)
	}
}

func TestMemWriteSeeker(t *testing.T) {
	m := &memWriteSeeker{}
	if _, err := m.Write([]byte("hello world")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if pos, err := m.Seek(6, io.SeekStart); err != nil || pos != 6 {
		t.Fatalf("SeekStart: pos=%d err=%v", pos, err)
	}
	if pos, err := m.Seek(2, io.SeekCurrent); err != nil || pos != 8 {
		t.Fatalf("SeekCurrent: pos=%d err=%v", pos, err)
	}
	if pos, err := m.Seek(-5, io.SeekEnd); err != nil || pos != 6 {
		t.Fatalf("SeekEnd: pos=%d err=%v", pos, err)
	}
	if _, err := m.Seek(-1, io.SeekStart); err != errNegativeSeek {
		t.Fatalf("negative seek: got %v want errNegativeSeek", err)
	}
	if _, err := m.Seek(0, 99); err != errInvalidWhence {
		t.Fatalf("bad whence: got %v want errInvalidWhence", err)
	}
	if _, err := m.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("reset seek: %v", err)
	}
	if _, err := m.Write([]byte("HELLO")); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	if !bytes.Equal(m.buf, []byte("HELLO world")) {
		t.Fatalf("overwrite mismatch: %q", m.buf)
	}
}

func TestRegisteredInRegistry(t *testing.T) {
	c, ok := carrier.Get(Format)
	if !ok {
		t.Fatal("audio carrier did not self-register")
	}
	if c.Format() != Format {
		t.Fatalf("registry returned wrong carrier: %s", c.Format())
	}
}
