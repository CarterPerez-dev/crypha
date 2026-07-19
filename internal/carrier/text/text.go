/*
©AngelaMos | 2026
text.go

Zero-width Unicode text carrier that appends an invisible framed payload after cover text
*/

package text

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"unicode/utf8"

	"github.com/CarterPerez-dev/crypha/internal/bitio"
	"github.com/CarterPerez-dev/crypha/internal/carrier"
)

const (
	Format = "text"

	zeroRune rune = 0x200B
	oneRune  rune = 0x2060

	bitsPerByte = 8
	lengthBytes = 4
	lengthBits  = lengthBytes * bitsPerByte

	unboundedCapacity = math.MaxInt32
)

var (
	textMagic     = [4]byte{0x7A, 0x57, 0x43, 0x52}
	textMagicBits = bytesToBits(textMagic[:])
)

var (
	ErrEmptyPayload = errors.New("crypha/text: empty payload")
	ErrNoPayload    = errors.New("crypha/text: no crypha payload found")
)

type textCarrier struct{}

func init() {
	carrier.Register(textCarrier{})
}

func (textCarrier) Format() string {
	return Format
}

func (textCarrier) Hide(cover io.Reader, payload []byte, out io.Writer) error {
	if len(payload) == 0 {
		return ErrEmptyPayload
	}

	coverBytes, err := io.ReadAll(cover)
	if err != nil {
		return fmt.Errorf("crypha/text: read cover: %w", err)
	}

	frame := make([]byte, 0, len(textMagic)+lengthBytes+len(payload))
	frame = append(frame, textMagic[:]...)
	var lenField [lengthBytes]byte
	binary.BigEndian.PutUint32(lenField[:], uint32(len(payload)))
	frame = append(frame, lenField[:]...)
	frame = append(frame, payload...)

	zw := make([]byte, 0, len(frame)*bitsPerByte*utf8.UTFMax)
	reader := bitio.NewReader(frame)
	for {
		bit, rerr := reader.ReadBit()
		if rerr != nil {
			break
		}
		zw = utf8.AppendRune(zw, runeForBit(bit))
	}

	if _, err := out.Write(coverBytes); err != nil {
		return err
	}
	_, err = out.Write(zw)
	return err
}

func (textCarrier) Reveal(stego io.Reader) ([]byte, error) {
	data, err := io.ReadAll(stego)
	if err != nil {
		return nil, fmt.Errorf("crypha/text: read stego: %w", err)
	}
	payload, ok := findFrame(extractBits(data))
	if !ok {
		return nil, ErrNoPayload
	}
	return payload, nil
}

func (textCarrier) Capacity(_ io.Reader) (int, error) {
	return unboundedCapacity, nil
}

func (textCarrier) Sniff(stego io.ReadSeeker) bool {
	data, err := io.ReadAll(stego)
	if err != nil {
		return false
	}
	_, ok := findFrame(extractBits(data))
	return ok
}

func runeForBit(bit byte) rune {
	if bit == 1 {
		return oneRune
	}
	return zeroRune
}

func findFrame(bits []byte) ([]byte, bool) {
	for start := 0; start+len(textMagicBits) <= len(bits); start++ {
		if !matchAt(bits, textMagicBits, start) {
			continue
		}
		if payload, ok := parseFrame(bits, start+len(textMagicBits)); ok {
			return payload, true
		}
	}
	return nil, false
}

func extractBits(data []byte) []byte {
	bits := make([]byte, 0, len(data))
	for _, r := range string(data) {
		switch r {
		case zeroRune:
			bits = append(bits, 0)
		case oneRune:
			bits = append(bits, 1)
		}
	}
	return bits
}

func bytesToBits(b []byte) []byte {
	out := make([]byte, 0, len(b)*bitsPerByte)
	for _, by := range b {
		for shift := bitsPerByte - 1; shift >= 0; shift-- {
			out = append(out, (by>>uint(shift))&1)
		}
	}
	return out
}

func bitsToBytes(bits []byte) []byte {
	writer := bitio.NewWriter()
	for _, bit := range bits {
		writer.WriteBit(bit)
	}
	return writer.Bytes()
}

func matchAt(haystack, needle []byte, at int) bool {
	if at+len(needle) > len(haystack) {
		return false
	}
	for i := range needle {
		if haystack[at+i] != needle[i] {
			return false
		}
	}
	return true
}

func parseFrame(bits []byte, headerStart int) ([]byte, bool) {
	if headerStart+lengthBits > len(bits) {
		return nil, false
	}
	length := binary.BigEndian.Uint32(bitsToBytes(bits[headerStart : headerStart+lengthBits]))
	if length == 0 {
		return nil, false
	}
	payloadStart := headerStart + lengthBits
	remaining := len(bits) - payloadStart
	if uint64(length)*bitsPerByte != uint64(remaining) {
		return nil, false
	}
	return bitsToBytes(bits[payloadStart:]), true
}
