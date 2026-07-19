/*
©AngelaMos | 2026
audio.go

LSB audio carrier for 16-bit PCM WAV covers, accepting FLAC covers decoded to WAV
*/

package audio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/CarterPerez-dev/crypha/internal/bitio"
	"github.com/CarterPerez-dev/crypha/internal/carrier"
	goaudio "github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/mewkiz/flac"
)

const (
	Format = "audio"

	bitsPerByte       = 8
	lengthPrefixBytes = 4
	lengthPrefixBits  = lengthPrefixBytes * bitsPerByte

	supportedBitDepth = 16
	wavFormatPCM      = 1

	riffTagBytes     = 4
	riffSizeBytes    = 4
	waveTagOffset    = riffTagBytes + riffSizeBytes
	waveTagBytes     = 4
	sniffHeaderBytes = waveTagOffset + waveTagBytes
)

var (
	riffTag = []byte("RIFF")
	waveTag = []byte("WAVE")
	flacTag = []byte("fLaC")
)

var (
	ErrEmptyPayload        = errors.New("crypha/audio: empty payload")
	ErrUnsupportedFormat   = errors.New("crypha/audio: cover must be a 16-bit PCM WAV or a FLAC file")
	ErrUnsupportedBitDepth = errors.New("crypha/audio: audio must be 16-bit, provide 16-bit PCM WAV or 16-bit FLAC")
	ErrNotPCM              = errors.New("crypha/audio: WAV must be uncompressed PCM")
	ErrNoSamples           = errors.New("crypha/audio: cover contains no audio samples")
	ErrPayloadTooLarge     = errors.New("crypha/audio: payload exceeds carrier capacity")
	ErrTooSmall            = errors.New("crypha/audio: audio is too small to contain a payload")
	ErrNoPayload           = errors.New("crypha/audio: no crypha payload found")
)

type pcm struct {
	samples     []int
	numChannels int
	sampleRate  int
}

type audioCarrier struct{}

func init() {
	carrier.Register(audioCarrier{})
}

func (audioCarrier) Format() string {
	return Format
}

func (audioCarrier) Hide(cover io.Reader, payload []byte, out io.Writer) error {
	if len(payload) == 0 {
		return ErrEmptyPayload
	}

	data, err := io.ReadAll(cover)
	if err != nil {
		return fmt.Errorf("crypha/audio: read cover: %w", err)
	}
	src, err := decodeCover(data)
	if err != nil {
		return err
	}

	framed := frame(payload)
	needBits := len(framed) * bitsPerByte
	if needBits > len(src.samples) {
		return fmt.Errorf("%w: need %d bytes, capacity is %d", ErrPayloadTooLarge, len(payload), capacityFromSlots(len(src.samples)))
	}

	reader := bitio.NewReader(framed)
	for slot := 0; slot < needBits; slot++ {
		bit, rerr := reader.ReadBit()
		if rerr != nil {
			return rerr
		}
		src.samples[slot] = (src.samples[slot] &^ 1) | int(bit)
	}

	return encodeWAV(out, src)
}

func (audioCarrier) Reveal(stego io.Reader) ([]byte, error) {
	data, err := io.ReadAll(stego)
	if err != nil {
		return nil, fmt.Errorf("crypha/audio: read stego: %w", err)
	}
	if !isWAV(data) {
		return nil, ErrUnsupportedFormat
	}

	src, err := decodeWAV(data)
	if err != nil {
		return nil, err
	}
	slots := len(src.samples)
	if slots < lengthPrefixBits {
		return nil, ErrTooSmall
	}

	length := binary.BigEndian.Uint32(readBits(src.samples, 0, lengthPrefixBits))
	maxPayload := capacityFromSlots(slots)
	if length == 0 || uint64(length) > uint64(maxPayload) {
		return nil, ErrNoPayload
	}

	payloadBits := int(length) * bitsPerByte
	return readBits(src.samples, lengthPrefixBits, payloadBits), nil
}

func (audioCarrier) Capacity(cover io.Reader) (int, error) {
	data, err := io.ReadAll(cover)
	if err != nil {
		return 0, fmt.Errorf("crypha/audio: read cover: %w", err)
	}
	src, err := decodeCover(data)
	if err != nil {
		return 0, err
	}
	return capacityFromSlots(len(src.samples)), nil
}

func (audioCarrier) Sniff(stego io.ReadSeeker) bool {
	head := make([]byte, sniffHeaderBytes)
	if _, err := io.ReadFull(stego, head); err != nil {
		return false
	}
	return isWAV(head)
}

func decodeCover(data []byte) (pcm, error) {
	switch {
	case isWAV(data):
		return decodeWAV(data)
	case hasPrefix(data, flacTag):
		return decodeFLAC(data)
	default:
		return pcm{}, ErrUnsupportedFormat
	}
}

func decodeWAV(data []byte) (pcm, error) {
	dec := wav.NewDecoder(bytes.NewReader(data))
	dec.ReadInfo()
	if err := dec.Err(); err != nil {
		return pcm{}, fmt.Errorf("crypha/audio: decode wav: %w", err)
	}
	if dec.WavAudioFormat != wavFormatPCM {
		return pcm{}, ErrNotPCM
	}
	if dec.BitDepth != supportedBitDepth {
		return pcm{}, ErrUnsupportedBitDepth
	}

	buf, err := dec.FullPCMBuffer()
	if err != nil {
		return pcm{}, fmt.Errorf("crypha/audio: read wav samples: %w", err)
	}
	if len(buf.Data) == 0 {
		return pcm{}, ErrNoSamples
	}

	return pcm{
		samples:     buf.Data,
		numChannels: buf.Format.NumChannels,
		sampleRate:  buf.Format.SampleRate,
	}, nil
}

func decodeFLAC(data []byte) (pcm, error) {
	stream, err := flac.New(bytes.NewReader(data))
	if err != nil {
		return pcm{}, fmt.Errorf("crypha/audio: decode flac: %w", err)
	}
	defer func() { _ = stream.Close() }()

	if stream.Info.BitsPerSample != supportedBitDepth {
		return pcm{}, ErrUnsupportedBitDepth
	}

	numChannels := int(stream.Info.NChannels)
	samples := make([]int, 0, int(stream.Info.NSamples)*numChannels)
	for {
		f, ferr := stream.ParseNext()
		if ferr == io.EOF {
			break
		}
		if ferr != nil {
			return pcm{}, fmt.Errorf("crypha/audio: read flac frame: %w", ferr)
		}
		if len(f.Subframes) == 0 {
			continue
		}
		block := len(f.Subframes[0].Samples)
		for i := 0; i < block; i++ {
			for _, sub := range f.Subframes {
				samples = append(samples, int(sub.Samples[i]))
			}
		}
	}
	if len(samples) == 0 {
		return pcm{}, ErrNoSamples
	}

	return pcm{
		samples:     samples,
		numChannels: numChannels,
		sampleRate:  int(stream.Info.SampleRate),
	}, nil
}

func encodeWAV(out io.Writer, src pcm) error {
	ws := &memWriteSeeker{}
	enc := wav.NewEncoder(ws, src.sampleRate, supportedBitDepth, src.numChannels, wavFormatPCM)
	buf := &goaudio.IntBuffer{
		Format:         &goaudio.Format{NumChannels: src.numChannels, SampleRate: src.sampleRate},
		Data:           src.samples,
		SourceBitDepth: supportedBitDepth,
	}
	if err := enc.Write(buf); err != nil {
		return fmt.Errorf("crypha/audio: encode wav: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("crypha/audio: finalize wav: %w", err)
	}

	_, err := out.Write(ws.buf)
	return err
}

func capacityFromSlots(slots int) int {
	usable := slots - lengthPrefixBits
	if usable < 0 {
		return 0
	}
	return usable / bitsPerByte
}

func frame(payload []byte) []byte {
	framed := make([]byte, lengthPrefixBytes+len(payload))
	binary.BigEndian.PutUint32(framed, uint32(len(payload)))
	copy(framed[lengthPrefixBytes:], payload)
	return framed
}

func readBits(samples []int, startSlot, count int) []byte {
	writer := bitio.NewWriter()
	for slot := startSlot; slot < startSlot+count; slot++ {
		writer.WriteBit(byte(samples[slot] & 1))
	}
	return writer.Bytes()
}

func isWAV(data []byte) bool {
	return len(data) >= sniffHeaderBytes &&
		hasPrefix(data, riffTag) &&
		bytes.Equal(data[waveTagOffset:waveTagOffset+waveTagBytes], waveTag)
}

func hasPrefix(data, prefix []byte) bool {
	return len(data) >= len(prefix) && bytes.Equal(data[:len(prefix)], prefix)
}
