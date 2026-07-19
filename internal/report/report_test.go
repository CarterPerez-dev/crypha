/*
©AngelaMos | 2026
report_test.go

Table and JSON rendering tests for command output
*/

package report

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/CarterPerez-dev/crypha/internal/config"
	"github.com/CarterPerez-dev/crypha/internal/engine"
)

func TestFormatsHuman(t *testing.T) {
	var buf bytes.Buffer
	if err := Formats(&buf, false); err != nil {
		t.Fatalf("Formats: %v", err)
	}
	out := buf.String()
	for _, f := range []string{"image", "audio", "qr", "text", "pdf"} {
		if !strings.Contains(out, f) {
			t.Errorf("formats output missing %q", f)
		}
	}
	if !strings.Contains(out, "attachment") {
		t.Error("pdf technique options not listed")
	}
}

func TestFormatsJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := Formats(&buf, true); err != nil {
		t.Fatalf("Formats: %v", err)
	}
	if !json.Valid(buf.Bytes()) {
		t.Fatal("formats json is invalid")
	}
	var lines []formatLine
	if err := json.Unmarshal(buf.Bytes(), &lines); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(lines) != 5 {
		t.Fatalf("got %d formats, want 5", len(lines))
	}
}

func TestCapacityHuman(t *testing.T) {
	rows := []engine.CapacityRow{
		{Format: "qr", Capacity: 52},
		{Format: "pdf", Capacity: math.MaxInt32},
		{Format: "text", Err: errors.New("crypha/text: cover too small")},
	}
	var buf bytes.Buffer
	if err := Capacity(&buf, rows, false); err != nil {
		t.Fatalf("Capacity: %v", err)
	}
	out := buf.String()
	for _, want := range []string{doesNotFit, unboundedLabel, notApplicable, "cover too small", "38"} {
		if !strings.Contains(out, want) {
			t.Errorf("capacity output missing %q\n%s", want, out)
		}
	}
}

func TestCapacityJSON(t *testing.T) {
	rows := []engine.CapacityRow{
		{Format: "qr", Capacity: 52},
		{Format: "pdf", Capacity: math.MaxInt32},
		{Format: "text", Err: errors.New("crypha/text: cover too small")},
	}
	var buf bytes.Buffer
	if err := Capacity(&buf, rows, true); err != nil {
		t.Fatalf("Capacity: %v", err)
	}
	var lines []capacityLine
	if err := json.Unmarshal(buf.Bytes(), &lines); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	byFormat := map[string]capacityLine{}
	for _, l := range lines {
		byFormat[l.Format] = l
	}
	if got := byFormat["qr"]; got.MaxPlaintext != 38 || got.MaxEncrypted != 0 {
		t.Errorf("qr line = %+v, want plaintext 38 encrypted 0", got)
	}
	if !byFormat["pdf"].Unbounded {
		t.Error("pdf should be unbounded")
	}
	if byFormat["text"].Applicable {
		t.Error("rejected cover should be inapplicable")
	}
}

func TestHideSummary(t *testing.T) {
	res := engine.HideResult{
		Format:        "image",
		PayloadBytes:  12,
		EnvelopeBytes: 80,
		Encrypted:     true,
	}
	var buf bytes.Buffer
	if err := HideSummary(&buf, res, "out.png", false); err != nil {
		t.Fatalf("HideSummary: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"image", "12", "80", "out.png", yesLabel} {
		if !strings.Contains(out, want) {
			t.Errorf("hide summary missing %q\n%s", want, out)
		}
	}
}

func TestVersion(t *testing.T) {
	var buf bytes.Buffer
	if err := Version(&buf, false); err != nil {
		t.Fatalf("Version: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, config.BinaryName) || !strings.Contains(out, config.Version) {
		t.Errorf("version output = %q", out)
	}
}

func TestRevealJSON(t *testing.T) {
	res := engine.RevealResult{Format: "image", Data: []byte("hidden bytes"), Encrypted: true}

	var toStdout bytes.Buffer
	if err := RevealJSON(&toStdout, res, ""); err != nil {
		t.Fatalf("RevealJSON: %v", err)
	}
	var line revealLine
	if err := json.Unmarshal(toStdout.Bytes(), &line); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(line.Data)
	if err != nil || string(decoded) != "hidden bytes" {
		t.Fatalf("data = %q err %v", decoded, err)
	}
	if line.Bytes != len(res.Data) || !line.Encrypted || line.Format != "image" {
		t.Fatalf("line = %+v", line)
	}

	var toFile bytes.Buffer
	if err := RevealJSON(&toFile, res, "out.bin"); err != nil {
		t.Fatalf("RevealJSON with file: %v", err)
	}
	var fileLine revealLine
	if err := json.Unmarshal(toFile.Bytes(), &fileLine); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if fileLine.Data != "" {
		t.Error("data should be omitted when writing to a file")
	}
	if fileLine.Output != "out.bin" {
		t.Errorf("output = %q, want out.bin", fileLine.Output)
	}
}
