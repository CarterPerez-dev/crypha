/*
©AngelaMos | 2026
image.go

LSB image carrier for PNG and 24-bit BMP covers via the mandatory NRGBA path
*/

package image

import (
	"encoding/binary"
	"errors"
	"fmt"
	stdimage "image"
	"image/draw"
	"image/png"
	"io"

	"github.com/CarterPerez-dev/crypha/internal/bitio"
	"github.com/CarterPerez-dev/crypha/internal/carrier"
	xbmp "golang.org/x/image/bmp"
)

const (
	Format = "image"

	formatPNG = "png"
	formatBMP = "bmp"

	bytesPerPixel     = 4
	channelsPerPixel  = 3
	bitsPerByte       = 8
	lengthPrefixBytes = 4
	lengthPrefixBits  = lengthPrefixBytes * bitsPerByte
)

var (
	pngSignature = []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	bmpSignature = []byte{'B', 'M'}
)

var (
	ErrEmptyPayload      = errors.New("crypha/image: empty payload")
	ErrUnsupportedFormat = errors.New("crypha/image: cover must be a PNG or 24-bit BMP")
	ErrPaletted          = errors.New("crypha/image: paletted images are not supported, provide a truecolor PNG or 24-bit BMP")
	Err16Bit             = errors.New("crypha/image: 16-bit images are not supported, provide an 8-bit truecolor image")
	ErrPayloadTooLarge   = errors.New("crypha/image: payload exceeds carrier capacity")
	ErrTooSmall          = errors.New("crypha/image: image is too small to contain a payload")
	ErrNoPayload         = errors.New("crypha/image: no crypha payload found")
)

type imageCarrier struct{}

func init() {
	carrier.Register(imageCarrier{})
}

func (imageCarrier) Format() string {
	return Format
}

func (imageCarrier) Hide(cover io.Reader, payload []byte, out io.Writer) error {
	if len(payload) == 0 {
		return ErrEmptyPayload
	}

	src, format, err := stdimage.Decode(cover)
	if err != nil {
		return fmt.Errorf("crypha/image: decode cover: %w", err)
	}
	if format != formatPNG && format != formatBMP {
		return ErrUnsupportedFormat
	}
	if err := rejectLossy(src); err != nil {
		return err
	}

	dst := toNRGBA(src)
	slots := channelSlots(dst)
	framed := frame(payload)
	needBits := len(framed) * bitsPerByte
	if needBits > slots {
		return fmt.Errorf("%w: need %d bytes, capacity is %d", ErrPayloadTooLarge, len(payload), capacityFromSlots(slots))
	}

	reader := bitio.NewReader(framed)
	for slot := 0; slot < needBits; slot++ {
		bit, rerr := reader.ReadBit()
		if rerr != nil {
			return rerr
		}
		off := pixOffset(slot)
		dst.Pix[off] = (dst.Pix[off] &^ 1) | bit
	}

	switch format {
	case formatPNG:
		return png.Encode(out, dst)
	default:
		return xbmp.Encode(out, dst)
	}
}

func (imageCarrier) Reveal(stego io.Reader) ([]byte, error) {
	src, format, err := stdimage.Decode(stego)
	if err != nil {
		return nil, fmt.Errorf("crypha/image: decode stego: %w", err)
	}
	if format != formatPNG && format != formatBMP {
		return nil, ErrUnsupportedFormat
	}
	if err := rejectLossy(src); err != nil {
		return nil, err
	}

	img := toNRGBA(src)
	slots := channelSlots(img)
	if slots < lengthPrefixBits {
		return nil, ErrTooSmall
	}

	length := binary.BigEndian.Uint32(readBits(img, 0, lengthPrefixBits))
	maxPayload := capacityFromSlots(slots)
	if length == 0 || uint64(length) > uint64(maxPayload) {
		return nil, ErrNoPayload
	}

	payloadBits := int(length) * bitsPerByte
	return readBits(img, lengthPrefixBits, payloadBits), nil
}

func (imageCarrier) Capacity(cover io.Reader) (int, error) {
	cfg, format, err := stdimage.DecodeConfig(cover)
	if err != nil {
		return 0, fmt.Errorf("crypha/image: decode cover config: %w", err)
	}
	if format != formatPNG && format != formatBMP {
		return 0, ErrUnsupportedFormat
	}
	slots := cfg.Width * cfg.Height * channelsPerPixel
	return capacityFromSlots(slots), nil
}

func (imageCarrier) Sniff(stego io.ReadSeeker) bool {
	head := make([]byte, len(pngSignature))
	if _, err := io.ReadFull(stego, head); err != nil {
		return false
	}
	if bytesHavePrefix(head, pngSignature) {
		return true
	}
	return bytesHavePrefix(head, bmpSignature)
}

func rejectLossy(src stdimage.Image) error {
	switch src.(type) {
	case *stdimage.Paletted:
		return ErrPaletted
	case *stdimage.RGBA64, *stdimage.NRGBA64, *stdimage.Gray16, *stdimage.CMYK:
		return Err16Bit
	default:
		return nil
	}
}

func toNRGBA(src stdimage.Image) *stdimage.NRGBA {
	if n, ok := src.(*stdimage.NRGBA); ok && n.Rect.Min == (stdimage.Point{}) {
		return n
	}
	b := src.Bounds()
	dst := stdimage.NewNRGBA(stdimage.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return dst
}

func channelSlots(img *stdimage.NRGBA) int {
	return img.Rect.Dx() * img.Rect.Dy() * channelsPerPixel
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

func pixOffset(slot int) int {
	return bytesPerPixel*(slot/channelsPerPixel) + (slot % channelsPerPixel)
}

func readBits(img *stdimage.NRGBA, startSlot, count int) []byte {
	writer := bitio.NewWriter()
	for slot := startSlot; slot < startSlot+count; slot++ {
		off := pixOffset(slot)
		writer.WriteBit(img.Pix[off] & 1)
	}
	return writer.Bytes()
}

func bytesHavePrefix(b, prefix []byte) bool {
	if len(b) < len(prefix) {
		return false
	}
	for i := range prefix {
		if b[i] != prefix[i] {
			return false
		}
	}
	return true
}
