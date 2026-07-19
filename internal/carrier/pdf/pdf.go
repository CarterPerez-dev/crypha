/*
©AngelaMos | 2026
pdf.go

PDF carrier with three techniques: embedded-file attachment, Info-dict metadata keys, and raw append-after-EOF
*/

package pdf

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"time"

	"github.com/CarterPerez-dev/crypha/internal/carrier"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

type Technique string

const (
	TechniqueAttachment Technique = "attachment"
	TechniqueMetadata   Technique = "metadata"
	TechniqueAppend     Technique = "append"
)

const (
	Format = "pdf"

	lengthPrefixBytes = 4
	unboundedCapacity = math.MaxInt32

	attachmentID   = "crypha-payload.bin"
	attachmentDesc = "crypha embedded payload"

	metaKeyPrefix = "X-Crypha-Payload-"
	metaCountKey  = "X-Crypha-Payload-Count"
	metaChunkSize = 8192
)

var (
	pdfSignature = []byte("%PDF-")
	appendMagic  = []byte{0x43, 0x72, 0x79, 0x50}
	epoch        = time.Unix(0, 0)
)

var (
	ErrEmptyPayload      = errors.New("crypha/pdf: empty payload")
	ErrUnsupportedFormat = errors.New("crypha/pdf: cover must be a PDF")
	ErrNoPayload         = errors.New("crypha/pdf: no crypha payload found")
)

type pdfCarrier struct {
	technique Technique
}

func init() {
	model.ConfigPath = "disable"
	carrier.Register(pdfCarrier{technique: TechniqueAttachment})
}

func New(t Technique) carrier.Carrier {
	return pdfCarrier{technique: t}
}

func (pdfCarrier) Format() string {
	return Format
}

func (c pdfCarrier) Hide(cover io.Reader, payload []byte, out io.Writer) error {
	if len(payload) == 0 {
		return ErrEmptyPayload
	}

	data, err := io.ReadAll(cover)
	if err != nil {
		return fmt.Errorf("crypha/pdf: read cover: %w", err)
	}
	if !isPDF(data) {
		return ErrUnsupportedFormat
	}

	switch c.technique {
	case TechniqueMetadata:
		return hideMetadata(data, payload, out)
	case TechniqueAppend:
		return hideAppend(data, payload, out)
	default:
		return hideAttachment(data, payload, out)
	}
}

func (pdfCarrier) Reveal(stego io.Reader) ([]byte, error) {
	data, err := io.ReadAll(stego)
	if err != nil {
		return nil, fmt.Errorf("crypha/pdf: read stego: %w", err)
	}
	if !isPDF(data) {
		return nil, ErrUnsupportedFormat
	}

	if payload, ok := revealAttachment(data); ok && len(payload) > 0 {
		return payload, nil
	}
	if payload, ok := revealMetadata(data); ok && len(payload) > 0 {
		return payload, nil
	}
	if payload, ok := revealAppend(data); ok && len(payload) > 0 {
		return payload, nil
	}
	return nil, ErrNoPayload
}

func (pdfCarrier) Capacity(cover io.Reader) (int, error) {
	data, err := io.ReadAll(cover)
	if err != nil {
		return 0, fmt.Errorf("crypha/pdf: read cover: %w", err)
	}
	if !isPDF(data) {
		return 0, ErrUnsupportedFormat
	}
	return unboundedCapacity, nil
}

func (pdfCarrier) Sniff(stego io.ReadSeeker) bool {
	head := make([]byte, len(pdfSignature))
	if _, err := io.ReadFull(stego, head); err != nil {
		return false
	}
	return bytes.Equal(head, pdfSignature)
}

func hideAttachment(cover, payload []byte, out io.Writer) error {
	conf := newConfig()
	ctx, err := api.ReadValidateAndOptimize(bytes.NewReader(cover), conf)
	if err != nil {
		return fmt.Errorf("crypha/pdf: read cover: %w", err)
	}
	att := model.Attachment{
		Reader:   bytes.NewReader(payload),
		ID:       attachmentID,
		FileName: attachmentID,
		Desc:     attachmentDesc,
		ModTime:  &epoch,
	}
	if err := ctx.AddAttachment(att, false); err != nil {
		return fmt.Errorf("crypha/pdf: attach payload: %w", err)
	}
	if err := api.Write(ctx, out, conf); err != nil {
		return fmt.Errorf("crypha/pdf: write pdf: %w", err)
	}
	return nil
}

func revealAttachment(stego []byte) ([]byte, bool) {
	conf := newConfig()
	attachments, err := api.ExtractAttachmentsRaw(bytes.NewReader(stego), "", nil, conf)
	if err != nil {
		return nil, false
	}
	for _, att := range attachments {
		if att.ID != attachmentID && att.FileName != attachmentID {
			continue
		}
		payload, rerr := io.ReadAll(att)
		if rerr != nil {
			return nil, false
		}
		return payload, true
	}
	return nil, false
}

func hideMetadata(cover, payload []byte, out io.Writer) error {
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	props := map[string]string{}
	count := 0
	for offset := 0; offset < len(encoded); offset += metaChunkSize {
		end := offset + metaChunkSize
		if end > len(encoded) {
			end = len(encoded)
		}
		props[metaKeyPrefix+strconv.Itoa(count)] = encoded[offset:end]
		count++
	}
	props[metaCountKey] = strconv.Itoa(count)

	if err := api.AddProperties(bytes.NewReader(cover), out, props, newConfig()); err != nil {
		return fmt.Errorf("crypha/pdf: write metadata: %w", err)
	}
	return nil
}

func revealMetadata(stego []byte) ([]byte, bool) {
	props, err := api.Properties(bytes.NewReader(stego), newConfig())
	if err != nil {
		return nil, false
	}
	countStr, ok := props[metaCountKey]
	if !ok {
		return nil, false
	}
	count, err := strconv.Atoi(countStr)
	if err != nil || count <= 0 {
		return nil, false
	}

	var encoded bytes.Buffer
	for i := 0; i < count; i++ {
		chunk, ok := props[metaKeyPrefix+strconv.Itoa(i)]
		if !ok {
			return nil, false
		}
		encoded.WriteString(chunk)
	}

	payload, err := base64.RawURLEncoding.DecodeString(encoded.String())
	if err != nil {
		return nil, false
	}
	return payload, true
}

func hideAppend(cover, payload []byte, out io.Writer) error {
	if _, err := out.Write(cover); err != nil {
		return err
	}
	if _, err := out.Write(appendMagic); err != nil {
		return err
	}
	if _, err := out.Write(payload); err != nil {
		return err
	}
	var lenField [lengthPrefixBytes]byte
	binary.BigEndian.PutUint32(lenField[:], uint32(len(payload)))
	_, err := out.Write(lenField[:])
	return err
}

func revealAppend(stego []byte) ([]byte, bool) {
	trailer := len(appendMagic) + lengthPrefixBytes
	if len(stego) < trailer {
		return nil, false
	}
	payloadLen := binary.BigEndian.Uint32(stego[len(stego)-lengthPrefixBytes:])
	if uint64(len(appendMagic))+uint64(payloadLen)+uint64(lengthPrefixBytes) > uint64(len(stego)) {
		return nil, false
	}
	magicStart := len(stego) - lengthPrefixBytes - int(payloadLen) - len(appendMagic)
	if !bytes.Equal(stego[magicStart:magicStart+len(appendMagic)], appendMagic) {
		return nil, false
	}
	payloadStart := magicStart + len(appendMagic)
	return stego[payloadStart : payloadStart+int(payloadLen)], true
}

func newConfig() *model.Configuration {
	conf := model.NewDefaultConfiguration()
	conf.ValidationMode = model.ValidationRelaxed
	return conf
}

func isPDF(data []byte) bool {
	return bytes.HasPrefix(data, pdfSignature)
}
