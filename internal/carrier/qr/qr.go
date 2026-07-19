/*
©AngelaMos | 2026
qr.go

QR carrier that hides a payload as Reed-Solomon-correctable errors so scanners self-heal to the cover
*/

package qr

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/CarterPerez-dev/crypha/internal/carrier"
	qrcode "github.com/skip2/go-qrcode"
)

const Format = "qr"

var (
	ErrEmptyPayload    = errors.New("crypha/qr: empty payload")
	ErrCoverRequired   = errors.New("crypha/qr: qr cover text is required")
	ErrCoverTooLarge   = errors.New("crypha/qr: cover text too large for the supported qr versions")
	ErrPayloadTooLarge = errors.New("crypha/qr: payload exceeds carrier capacity")
	ErrNoPayload       = errors.New("crypha/qr: no crypha payload found")
	ErrNotQR           = errors.New("crypha/qr: stego is not a crypha qr image")
	errBadSymbol       = errors.New("crypha/qr: unexpected qr symbol geometry")
)

type qrCarrier struct{}

func init() {
	carrier.Register(qrCarrier{})
}

func (qrCarrier) Format() string {
	return Format
}

func (qrCarrier) Hide(cover io.Reader, payload []byte, out io.Writer) error {
	if len(payload) == 0 {
		return ErrEmptyPayload
	}
	coverText, err := io.ReadAll(cover)
	if err != nil {
		return fmt.Errorf("crypha/qr: read cover: %w", err)
	}
	if len(coverText) == 0 {
		return ErrCoverRequired
	}
	if len(payload) > maxCapacity() {
		return fmt.Errorf("%w: need %d bytes, max is %d", ErrPayloadTooLarge, len(payload), maxCapacity())
	}

	code, version, err := selectVersion(string(coverText), len(payload))
	if err != nil {
		return err
	}

	clean, err := matrixFromBitmap(code.Bitmap(), version)
	if err != nil {
		return err
	}

	maskID, level, ok := parseFormat(clean)
	if !ok || level != ecLevelHigh {
		return errBadSymbol
	}

	spec, _ := lookupVersion(version)
	isFunc := functionModules(version)
	order := placementOrder(version, isFunc)
	serial := readSerial(clean, order, maskID, spec.totalCodewords())

	dataBlocks, ecBlocks, ok := spec.deinterleave(serial)
	if !ok {
		return errBadSymbol
	}

	framed := frame(payload)
	if err := injectFramed(dataBlocks, spec, framed); err != nil {
		return err
	}

	stego := clean.clone()
	writeSerial(stego, order, maskID, spec.interleave(dataBlocks, ecBlocks))
	return renderPNG(stego, out)
}

func (qrCarrier) Reveal(stego io.Reader) ([]byte, error) {
	m, version, err := readGrid(stego)
	if err != nil {
		return nil, err
	}

	maskID, level, ok := parseFormat(m)
	if !ok || level != ecLevelHigh {
		return nil, ErrNoPayload
	}

	spec, _ := lookupVersion(version)
	isFunc := functionModules(version)
	order := placementOrder(version, isFunc)
	serial := readSerial(m, order, maskID, spec.totalCodewords())

	dataBlocks, ecBlocks, ok := spec.deinterleave(serial)
	if !ok {
		return nil, ErrNoPayload
	}

	framed, err := extractFramed(dataBlocks, ecBlocks, spec)
	if err != nil {
		return nil, err
	}
	return unframe(framed)
}

func (qrCarrier) Capacity(cover io.Reader) (int, error) {
	coverText, err := io.ReadAll(cover)
	if err != nil {
		return 0, fmt.Errorf("crypha/qr: read cover: %w", err)
	}
	if len(coverText) == 0 {
		return 0, ErrCoverRequired
	}
	best := 0
	for version := minSupportedVersion; version <= maxSupportedVersion; version++ {
		if _, err := qrcode.NewWithForcedVersion(string(coverText), version, qrcode.Highest); err != nil {
			continue
		}
		if c := versionTable[version].capacityBytes(); c > best {
			best = c
		}
	}
	return best, nil
}

func (qrCarrier) Sniff(stego io.ReadSeeker) bool {
	m, _, err := readGrid(stego)
	if err != nil {
		return false
	}
	return hasFinderPatterns(m)
}

func selectVersion(coverText string, payloadLen int) (*qrcode.QRCode, int, error) {
	coverFits := false
	for version := minSupportedVersion; version <= maxSupportedVersion; version++ {
		spec := versionTable[version]
		code, err := qrcode.NewWithForcedVersion(coverText, version, qrcode.Highest)
		if err != nil {
			continue
		}
		coverFits = true
		if spec.capacityBytes() < payloadLen {
			continue
		}
		code.DisableBorder = true
		return code, version, nil
	}
	if !coverFits {
		return nil, 0, ErrCoverTooLarge
	}
	return nil, 0, fmt.Errorf("%w: need %d bytes", ErrPayloadTooLarge, payloadLen)
}

func matrixFromBitmap(bitmap [][]bool, version int) (matrix, error) {
	size := symbolSize(version)
	if len(bitmap) != size {
		return matrix{}, errBadSymbol
	}
	m := newMatrix(size)
	for y := 0; y < size; y++ {
		if len(bitmap[y]) != size {
			return matrix{}, errBadSymbol
		}
		copy(m.grid[y], bitmap[y])
	}
	return m, nil
}

func injectFramed(dataBlocks [][]byte, spec qrVersion, framed []byte) error {
	nb := spec.numBlocks()
	inject := spec.injectPerBlock()
	for t := 0; t < len(framed); t++ {
		block := t % nb
		slot := t / nb
		if slot >= inject || slot >= len(dataBlocks[block]) {
			return fmt.Errorf("%w: framed length %d", ErrPayloadTooLarge, len(framed))
		}
		dataBlocks[block][slot] ^= framed[t]
	}
	return nil
}

func extractFramed(dataBlocks, ecBlocks [][]byte, spec qrVersion) ([]byte, error) {
	nb := spec.numBlocks()
	inject := spec.injectPerBlock()
	ec := spec.ecPerBlock()

	clean := make([][]byte, nb)
	for b := 0; b < nb; b++ {
		recv := append(append([]byte(nil), dataBlocks[b]...), ecBlocks[b]...)
		corrected, err := rsDecode(recv, ec)
		if err != nil {
			return nil, ErrNoPayload
		}
		clean[b] = corrected[:len(dataBlocks[b])]
	}

	framed := make([]byte, nb*inject)
	for t := 0; t < len(framed); t++ {
		block := t % nb
		slot := t / nb
		framed[t] = clean[block][slot] ^ dataBlocks[block][slot]
	}
	return framed, nil
}

func frame(payload []byte) []byte {
	out := make([]byte, framePrefixBytes+len(payload))
	binary.BigEndian.PutUint32(out, uint32(len(payload)))
	copy(out[framePrefixBytes:], payload)
	return out
}

func unframe(framed []byte) ([]byte, error) {
	if len(framed) < framePrefixBytes {
		return nil, ErrNoPayload
	}
	length := binary.BigEndian.Uint32(framed)
	if length == 0 || uint64(length) > uint64(len(framed)-framePrefixBytes) {
		return nil, ErrNoPayload
	}
	out := make([]byte, length)
	copy(out, framed[framePrefixBytes:framePrefixBytes+int(length)])
	return out, nil
}

func hasFinderPatterns(m matrix) bool {
	return isFinder(m, 0, 0) &&
		isFinder(m, m.size-finderPatternSize, 0) &&
		isFinder(m, 0, m.size-finderPatternSize)
}

func isFinder(m matrix, ox, oy int) bool {
	if ox < 0 || oy < 0 || ox+finderPatternSize > m.size || oy+finderPatternSize > m.size {
		return false
	}
	for y := 0; y < finderPatternSize; y++ {
		for x := 0; x < finderPatternSize; x++ {
			border := x == 0 || x == finderPatternSize-1 || y == 0 || y == finderPatternSize-1
			center := x >= 2 && x <= 4 && y >= 2 && y <= 4
			if m.grid[oy+y][ox+x] != (border || center) {
				return false
			}
		}
	}
	return true
}

func maxCapacity() int {
	return versionTable[maxSupportedVersion].capacityBytes()
}
