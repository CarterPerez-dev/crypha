/*
©AngelaMos | 2026
report.go

Human-readable tables and JSON rendering for crypha command output
*/

package report

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"runtime/debug"
	"strings"
	"text/tabwriter"

	"github.com/CarterPerez-dev/crypha/internal/config"
	"github.com/CarterPerez-dev/crypha/internal/engine"
	"github.com/CarterPerez-dev/crypha/internal/payload"
)

const (
	unboundedLabel = "unbounded"
	notApplicable  = "n/a"
	doesNotFit     = "does not fit"
	yesLabel       = "yes"
	noLabel        = "no"
	minCellWidth   = 0
	tabWidth       = 2
	padding        = 2
	padChar        = ' '
)

type formatLine struct {
	Format      string   `json:"format"`
	Cover       string   `json:"cover"`
	Output      string   `json:"output"`
	Techniques  []string `json:"techniques,omitempty"`
	Description string   `json:"description"`
}

type capacityLine struct {
	Format       string `json:"format"`
	Applicable   bool   `json:"applicable"`
	Capacity     int    `json:"capacity_bytes,omitempty"`
	MaxPlaintext int    `json:"max_plaintext_bytes,omitempty"`
	MaxEncrypted int    `json:"max_encrypted_bytes,omitempty"`
	Unbounded    bool   `json:"unbounded,omitempty"`
	Note         string `json:"note,omitempty"`
}

type hideLine struct {
	Format        string `json:"format"`
	Technique     string `json:"technique,omitempty"`
	PayloadBytes  int    `json:"payload_bytes"`
	EnvelopeBytes int    `json:"envelope_bytes"`
	Encrypted     bool   `json:"encrypted"`
	Compressed    bool   `json:"compressed"`
	Output        string `json:"output"`
}

type revealLine struct {
	Format    string `json:"format"`
	Bytes     int    `json:"bytes"`
	Encrypted bool   `json:"encrypted"`
	Output    string `json:"output,omitempty"`
	Data      string `json:"data,omitempty"`
}

type versionLine struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	GoVersion string `json:"go_version"`
}

type tableWriter struct {
	tw  *tabwriter.Writer
	err error
}

func newTableWriter(w io.Writer) *tableWriter {
	return &tableWriter{tw: tabwriter.NewWriter(w, minCellWidth, tabWidth, padding, padChar, 0)}
}

func (t *tableWriter) row(format string, a ...any) {
	if t.err != nil {
		return
	}
	_, t.err = fmt.Fprintf(t.tw, format, a...)
}

func (t *tableWriter) flush() error {
	if t.err != nil {
		return t.err
	}
	return t.tw.Flush()
}

func Formats(w io.Writer, jsonOut bool) error {
	cat := engine.Catalog()
	lines := make([]formatLine, 0, len(cat))
	for _, fi := range cat {
		d := config.FormatDetails[fi.Name]
		lines = append(lines, formatLine{
			Format:      fi.Name,
			Cover:       d.CoverInput,
			Output:      d.Output,
			Techniques:  fi.Techniques,
			Description: d.Blurb,
		})
	}
	if jsonOut {
		return writeJSON(w, lines)
	}
	tw := newTableWriter(w)
	tw.row("FORMAT\tCOVER\tOUTPUT\tOPTIONS\tDESCRIPTION\n")
	for _, l := range lines {
		tw.row("%s\t%s\t%s\t%s\t%s\n", l.Format, l.Cover, l.Output, options(l.Techniques), l.Description)
	}
	return tw.flush()
}

func Capacity(w io.Writer, rows []engine.CapacityRow, jsonOut bool) error {
	lines := make([]capacityLine, 0, len(rows))
	for _, r := range rows {
		lines = append(lines, capacityRow(r))
	}
	if jsonOut {
		return writeJSON(w, lines)
	}
	tw := newTableWriter(w)
	tw.row("FORMAT\tENVELOPE\tMAX PLAINTEXT\tMAX ENCRYPTED\tNOTE\n")
	for _, l := range lines {
		tw.row("%s\t%s\t%s\t%s\t%s\n", l.Format, capacityCell(l), plaintextCell(l), encryptedCell(l), l.Note)
	}
	return tw.flush()
}

func HideSummary(w io.Writer, res engine.HideResult, outPath string, jsonOut bool) error {
	line := hideLine{
		Format:        res.Format,
		Technique:     res.Technique,
		PayloadBytes:  res.PayloadBytes,
		EnvelopeBytes: res.EnvelopeBytes,
		Encrypted:     res.Encrypted,
		Compressed:    res.Compressed,
		Output:        outPath,
	}
	if jsonOut {
		return writeJSON(w, line)
	}
	tw := newTableWriter(w)
	tw.row("OUTPUT\t%s\n", line.Output)
	tw.row("FORMAT\t%s\n", formatCell(res.Format, res.Technique))
	tw.row("PAYLOAD\t%d bytes\n", line.PayloadBytes)
	tw.row("ENVELOPE\t%d bytes\n", line.EnvelopeBytes)
	tw.row("ENCRYPTED\t%s\n", boolLabel(line.Encrypted))
	tw.row("COMPRESSED\t%s\n", boolLabel(line.Compressed))
	return tw.flush()
}

func RevealStatus(w io.Writer, res engine.RevealResult, outPath string) error {
	_, err := fmt.Fprintf(w, "revealed %d bytes via %s -> %s\n", len(res.Data), res.Format, outPath)
	return err
}

func RevealJSON(w io.Writer, res engine.RevealResult, outPath string) error {
	line := revealLine{
		Format:    res.Format,
		Bytes:     len(res.Data),
		Encrypted: res.Encrypted,
		Output:    outPath,
	}
	if outPath == "" {
		line.Data = base64.StdEncoding.EncodeToString(res.Data)
	}
	return writeJSON(w, line)
}

func Version(w io.Writer, jsonOut bool) error {
	line := versionLine{
		Name:      config.BinaryName,
		Version:   config.Version,
		GoVersion: goVersion(),
	}
	if jsonOut {
		return writeJSON(w, line)
	}
	_, err := fmt.Fprintf(w, "%s %s (%s)\n", line.Name, line.Version, line.GoVersion)
	return err
}

func capacityRow(r engine.CapacityRow) capacityLine {
	l := capacityLine{Format: r.Format}
	if r.Err != nil {
		l.Note = reason(r.Err)
		return l
	}
	l.Applicable = true
	if r.Capacity >= math.MaxInt32 {
		l.Unbounded = true
		return l
	}
	l.Capacity = r.Capacity
	l.MaxPlaintext = clampZero(r.Capacity - payload.Overhead(false))
	l.MaxEncrypted = clampZero(r.Capacity - payload.Overhead(true))
	return l
}

func capacityCell(l capacityLine) string {
	switch {
	case !l.Applicable:
		return notApplicable
	case l.Unbounded:
		return unboundedLabel
	default:
		return fmt.Sprintf("%d", l.Capacity)
	}
}

func plaintextCell(l capacityLine) string {
	switch {
	case !l.Applicable:
		return notApplicable
	case l.Unbounded:
		return unboundedLabel
	default:
		return fmt.Sprintf("%d", l.MaxPlaintext)
	}
}

func encryptedCell(l capacityLine) string {
	switch {
	case !l.Applicable:
		return notApplicable
	case l.Unbounded:
		return unboundedLabel
	case l.MaxEncrypted <= 0:
		return doesNotFit
	default:
		return fmt.Sprintf("%d", l.MaxEncrypted)
	}
}

func options(techniques []string) string {
	if len(techniques) == 0 {
		return "-"
	}
	return strings.Join(techniques, " | ")
}

func formatCell(format, technique string) string {
	if technique == "" {
		return format
	}
	return format + " (" + technique + ")"
}

func boolLabel(b bool) string {
	if b {
		return yesLabel
	}
	return noLabel
}

func reason(err error) string {
	msg := err.Error()
	if i := strings.Index(msg, ": "); i >= 0 {
		return msg[i+2:]
	}
	return msg
}

func clampZero(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

func goVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.GoVersion != "" {
		return info.GoVersion
	}
	return "unknown"
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
