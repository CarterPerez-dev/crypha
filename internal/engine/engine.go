/*
©AngelaMos | 2026
engine.go

The shared steganography engine that both frontends drive, wiring carriers to the payload envelope
*/

package engine

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/CarterPerez-dev/crypha/internal/carrier"
	_ "github.com/CarterPerez-dev/crypha/internal/carrier/all"
	"github.com/CarterPerez-dev/crypha/internal/carrier/pdf"
	"github.com/CarterPerez-dev/crypha/internal/payload"
)

var (
	ErrNoFormat          = errors.New("crypha: a carrier format is required")
	ErrUnknownFormat     = errors.New("crypha: unknown carrier format")
	ErrUnknownTechnique  = errors.New("crypha: unknown pdf technique")
	ErrTechniqueOnNonPDF = errors.New("crypha: technique only applies to the pdf format")
	ErrUndetected        = errors.New("crypha: no crypha payload detected; pass a format to force a carrier")
)

type HideRequest struct {
	Format    string
	Technique string
	Cover     io.Reader
	Payload   []byte
	Out       io.Writer
	Options   payload.Options
}

type HideResult struct {
	Format        string
	Technique     string
	PayloadBytes  int
	EnvelopeBytes int
	Encrypted     bool
	Compressed    bool
}

type RevealRequest struct {
	Format     string
	Stego      []byte
	Passphrase []byte
}

type RevealResult struct {
	Format    string
	Data      []byte
	Encrypted bool
}

type CapacityRow struct {
	Format   string
	Capacity int
	Err      error
}

type FormatInfo struct {
	Name       string
	Techniques []string
}

func ResolveCarrier(format, technique string) (carrier.Carrier, error) {
	if format == "" {
		return nil, ErrNoFormat
	}
	if format == pdf.Format {
		t := pdf.TechniqueAttachment
		if technique != "" {
			t = pdf.Technique(technique)
		}
		switch t {
		case pdf.TechniqueAttachment, pdf.TechniqueMetadata, pdf.TechniqueAppend:
			return pdf.New(t), nil
		default:
			return nil, fmt.Errorf("%w: %q", ErrUnknownTechnique, technique)
		}
	}
	if technique != "" {
		return nil, ErrTechniqueOnNonPDF
	}
	c, ok := carrier.Get(format)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownFormat, format)
	}
	return c, nil
}

func Hide(req HideRequest) (HideResult, error) {
	c, err := ResolveCarrier(req.Format, req.Technique)
	if err != nil {
		return HideResult{}, err
	}
	env, err := payload.Pack(req.Payload, req.Options)
	if err != nil {
		return HideResult{}, err
	}
	if err := c.Hide(req.Cover, env, req.Out); err != nil {
		return HideResult{}, err
	}
	return HideResult{
		Format:        c.Format(),
		Technique:     req.Technique,
		PayloadBytes:  len(req.Payload),
		EnvelopeBytes: len(env),
		Encrypted:     len(req.Options.Passphrase) > 0,
		Compressed:    req.Options.Compress,
	}, nil
}

func Reveal(req RevealRequest) (RevealResult, error) {
	c, env, err := locate(req.Format, req.Stego)
	if err != nil {
		return RevealResult{}, err
	}
	encrypted, err := payload.IsEncrypted(env)
	if err != nil {
		return RevealResult{Format: c.Format()}, err
	}
	if encrypted && len(req.Passphrase) == 0 {
		return RevealResult{Format: c.Format(), Encrypted: true}, payload.ErrPassphraseRequired
	}
	data, err := payload.Unpack(env, req.Passphrase)
	if err != nil {
		return RevealResult{Format: c.Format(), Encrypted: encrypted}, err
	}
	return RevealResult{Format: c.Format(), Data: data, Encrypted: encrypted}, nil
}

func Capacity(format string, cover io.Reader) (int, error) {
	c, err := ResolveCarrier(format, "")
	if err != nil {
		return 0, err
	}
	return c.Capacity(cover)
}

func CapacityAll(cover []byte) []CapacityRow {
	carriers := carrier.All()
	rows := make([]CapacityRow, 0, len(carriers))
	for _, c := range carriers {
		n, err := c.Capacity(bytes.NewReader(cover))
		rows = append(rows, CapacityRow{Format: c.Format(), Capacity: n, Err: err})
	}
	return rows
}

func Catalog() []FormatInfo {
	names := carrier.Formats()
	out := make([]FormatInfo, 0, len(names))
	for _, name := range names {
		out = append(out, FormatInfo{Name: name, Techniques: Techniques(name)})
	}
	return out
}

func Techniques(format string) []string {
	if format != pdf.Format {
		return nil
	}
	return []string{
		string(pdf.TechniqueAttachment),
		string(pdf.TechniqueMetadata),
		string(pdf.TechniqueAppend),
	}
}

func Overhead(encrypted bool) int {
	return payload.Overhead(encrypted)
}

func EnvelopeSize(data []byte, opts payload.Options) (int, error) {
	return payload.EnvelopeSize(data, opts)
}

func locate(format string, stego []byte) (carrier.Carrier, []byte, error) {
	if format != "" {
		c, err := ResolveCarrier(format, "")
		if err != nil {
			return nil, nil, err
		}
		env, err := c.Reveal(bytes.NewReader(stego))
		if err != nil {
			return nil, nil, err
		}
		return c, env, nil
	}
	return detect(stego)
}

func detect(stego []byte) (carrier.Carrier, []byte, error) {
	for _, c := range carrier.All() {
		if !c.Sniff(bytes.NewReader(stego)) {
			continue
		}
		env, err := c.Reveal(bytes.NewReader(stego))
		if err != nil {
			continue
		}
		if payload.Validate(env) == nil {
			return c, env, nil
		}
	}
	return nil, nil, ErrUndetected
}
