/*
©AngelaMos | 2026
tui_test.go

Model navigation, engine round-trips through the wizard, and view smoke tests
*/

package tui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/CarterPerez-dev/crypha/internal/engine"
	tea "github.com/charmbracelet/bubbletea"
)

func ready() Model {
	m := New()
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 44})
	return nm.(Model)
}

func step(m Model, msg tea.Msg) (Model, tea.Cmd) {
	nm, cmd := m.Update(msg)
	return nm.(Model), cmd
}

func keyEnter() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyEnter} }
func keyEsc() tea.KeyMsg   { return tea.KeyMsg{Type: tea.KeyEsc} }
func keyDown() tea.KeyMsg  { return tea.KeyMsg{Type: tea.KeyDown} }
func keyTab() tea.KeyMsg   { return tea.KeyMsg{Type: tea.KeyTab} }
func keySpace() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeySpace} }

func typeText(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func drain(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, drain(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

func finishRun(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	for _, msg := range drain(cmd) {
		m, _ = step(m, msg)
	}
	for i := 0; i < 60 && m.stage == stageRunning; i++ {
		m, _ = step(m, tickMsg{})
	}
	return m
}

func makePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: uint8(x + y), A: 0xFF})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func pickFormat(t *testing.T, m Model, name string) Model {
	t.Helper()
	for i := 0; i <= len(m.fmtPick.items); i++ {
		if m.fmtPick.selected().value == name {
			return m
		}
		m, _ = step(m, keyDown())
	}
	t.Fatalf("format %q not present in picker", name)
	return m
}

func TestHideRevealRoundTripPlaintext(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "out.png")
	cover := makePNG(t, 64, 64)
	const secret = "attack at dawn"

	m := ready()
	m, _ = step(m, keyEnter())
	if m.stage != stageFormat {
		t.Fatalf("after operation: stage = %v", m.stage)
	}
	m = pickFormat(t, m, "image")
	m, _ = step(m, keyEnter())
	if m.stage != stageCover {
		t.Fatalf("after format: stage = %v", m.stage)
	}
	m, _ = step(m, fileLoadedMsg{origin: stageCover, path: "cover.png", data: cover})
	if m.stage != stagePayload {
		t.Fatalf("after cover: stage = %v", m.stage)
	}
	m, _ = step(m, typeText(secret))
	m, _ = step(m, keyEnter())
	if m.stage != stageSecure {
		t.Fatalf("after payload: stage = %v", m.stage)
	}
	m, _ = step(m, keyEnter())
	if m.stage != stageSave {
		t.Fatalf("after secure: stage = %v", m.stage)
	}
	m.outPath.SetValue(out)
	m, _ = step(m, keyEnter())
	if m.stage != stageReview {
		t.Fatalf("after save: stage = %v", m.stage)
	}
	if !m.hideFits() {
		t.Fatalf("payload should fit a 64x64 cover")
	}
	m, cmd := step(m, keyEnter())
	if m.stage != stageRunning {
		t.Fatalf("after review: stage = %v", m.stage)
	}
	m = finishRun(t, m, cmd)
	if m.stage != stageResult {
		t.Fatalf("run did not finish: stage = %v", m.stage)
	}
	if m.engineErr != nil {
		t.Fatalf("hide errored: %v", m.engineErr)
	}
	if m.hideRes.Format != "image" {
		t.Fatalf("hide format = %q", m.hideRes.Format)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("stego file not written: %v", err)
	}

	stego, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read stego: %v", err)
	}

	r := ready()
	r, _ = step(r, keyDown())
	r, _ = step(r, keyEnter())
	if r.op != opReveal || r.stage != stageFormat {
		t.Fatalf("reveal setup: op=%v stage=%v", r.op, r.stage)
	}
	r, _ = step(r, keyEnter())
	if r.format != "" || r.stage != stageCover {
		t.Fatalf("reveal auto-detect setup: format=%q stage=%v", r.format, r.stage)
	}
	r, cmd = step(r, fileLoadedMsg{origin: stageCover, path: out, data: stego})
	if r.stage != stageRunning {
		t.Fatalf("reveal after stego: stage = %v", r.stage)
	}
	r = finishRun(t, r, cmd)
	if r.stage != stageResult {
		t.Fatalf("reveal did not finish: stage = %v", r.stage)
	}
	if r.engineErr != nil {
		t.Fatalf("reveal errored: %v", r.engineErr)
	}
	if got := string(r.revealRes.Data); got != secret {
		t.Fatalf("revealed %q, want %q", got, secret)
	}
}

func TestHideRevealRoundTripEncrypted(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "enc.png")
	cover := makePNG(t, 64, 64)
	const secret = "the eagle lands at noon"
	const pass = "correct horse battery staple"

	m := ready()
	m, _ = step(m, keyEnter())
	m = pickFormat(t, m, "image")
	m, _ = step(m, keyEnter())
	m, _ = step(m, fileLoadedMsg{origin: stageCover, path: "cover.png", data: cover})
	m, _ = step(m, typeText(secret))
	m, _ = step(m, keyEnter())
	if m.stage != stageSecure {
		t.Fatalf("stage = %v", m.stage)
	}
	m, _ = step(m, keySpace())
	if !m.secure.encrypt {
		t.Fatalf("encrypt toggle did not engage")
	}
	m, _ = step(m, keyDown())
	m, _ = step(m, typeText(pass))
	m, _ = step(m, keyEnter())
	if m.stage != stageSave {
		t.Fatalf("after secure: stage = %v", m.stage)
	}
	if len(m.pass) == 0 {
		t.Fatalf("passphrase not captured")
	}
	m.outPath.SetValue(out)
	m, _ = step(m, keyEnter())
	m, cmd := step(m, keyEnter())
	m = finishRun(t, m, cmd)
	if m.engineErr != nil {
		t.Fatalf("encrypted hide errored: %v", m.engineErr)
	}
	if !m.hideRes.Encrypted {
		t.Fatalf("result not marked encrypted")
	}

	stego, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read stego: %v", err)
	}

	r := ready()
	r, _ = step(r, keyDown())
	r, _ = step(r, keyEnter())
	r = pickFormat(t, r, "image")
	r, _ = step(r, keyEnter())
	if r.format != "image" {
		t.Fatalf("reveal format = %q", r.format)
	}
	r, cmd = step(r, fileLoadedMsg{origin: stageCover, path: out, data: stego})
	r = finishRun(t, r, cmd)
	if r.stage != stagePassphrase {
		t.Fatalf("encrypted reveal should prompt for a passphrase, stage = %v", r.stage)
	}
	r, _ = step(r, typeText(pass))
	r, cmd = step(r, keyEnter())
	if r.stage != stageRunning {
		t.Fatalf("after passphrase: stage = %v", r.stage)
	}
	r = finishRun(t, r, cmd)
	if r.stage != stageResult || r.engineErr != nil {
		t.Fatalf("encrypted reveal failed: stage=%v err=%v", r.stage, r.engineErr)
	}
	if got := string(r.revealRes.Data); got != secret {
		t.Fatalf("revealed %q, want %q", got, secret)
	}
	if !r.revealRes.Encrypted {
		t.Fatalf("revealed payload not marked encrypted")
	}
}

func TestWrongPassphraseReprompts(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "wp.png")
	cover := makePNG(t, 64, 64)

	m := ready()
	m, _ = step(m, keyEnter())
	m = pickFormat(t, m, "image")
	m, _ = step(m, keyEnter())
	m, _ = step(m, fileLoadedMsg{origin: stageCover, path: "c.png", data: cover})
	m, _ = step(m, typeText("classified"))
	m, _ = step(m, keyEnter())
	m, _ = step(m, keySpace())
	m, _ = step(m, keyDown())
	m, _ = step(m, typeText("realpass"))
	m, _ = step(m, keyEnter())
	m.outPath.SetValue(out)
	m, _ = step(m, keyEnter())
	m, cmd := step(m, keyEnter())
	m = finishRun(t, m, cmd)
	if m.engineErr != nil {
		t.Fatalf("hide errored: %v", m.engineErr)
	}
	stego, _ := os.ReadFile(out)

	r := ready()
	r, _ = step(r, keyDown())
	r, _ = step(r, keyEnter())
	r = pickFormat(t, r, "image")
	r, _ = step(r, keyEnter())
	r, cmd = step(r, fileLoadedMsg{origin: stageCover, path: out, data: stego})
	r = finishRun(t, r, cmd)
	if r.stage != stagePassphrase {
		t.Fatalf("stage = %v", r.stage)
	}
	r, _ = step(r, typeText("wrongpass"))
	r, cmd = step(r, keyEnter())
	r = finishRun(t, r, cmd)
	if r.stage != stagePassphrase {
		t.Fatalf("wrong passphrase should re-prompt, stage = %v", r.stage)
	}
	if r.passPrompts < 2 {
		t.Fatalf("passPrompts = %d, want >= 2", r.passPrompts)
	}
}

func TestCapacityFlow(t *testing.T) {
	cover := makePNG(t, 48, 48)
	m := ready()
	m, _ = step(m, keyDown())
	m, _ = step(m, keyDown())
	m, _ = step(m, keyEnter())
	if m.op != opCapacity || m.stage != stageCover {
		t.Fatalf("capacity setup: op=%v stage=%v", m.op, m.stage)
	}
	m, _ = step(m, fileLoadedMsg{origin: stageCover, path: "cover.png", data: cover})
	if m.stage != stageReview {
		t.Fatalf("after cover: stage = %v", m.stage)
	}
	if len(m.capRows) == 0 {
		t.Fatalf("capacity rows not built")
	}
	var haveImage bool
	for _, r := range m.capRows {
		if r.format == "image" {
			haveImage = true
			if !r.applicable || r.maxPlaintext <= 0 {
				t.Fatalf("image row = %+v", r)
			}
		}
	}
	if !haveImage {
		t.Fatalf("no image row in capacity report")
	}
}

func TestPdfTechniqueBranch(t *testing.T) {
	m := ready()
	m, _ = step(m, keyEnter())
	m = pickFormat(t, m, "pdf")
	m, _ = step(m, keyEnter())
	if m.format != "pdf" || m.stage != stageTechnique {
		t.Fatalf("pdf branch: format=%q stage=%v", m.format, m.stage)
	}
	m, _ = step(m, keyEnter())
	if m.technique != "attachment" || m.stage != stageCover {
		t.Fatalf("technique branch: technique=%q stage=%v", m.technique, m.stage)
	}
}

func TestTechniqueClearedOnFormatChange(t *testing.T) {
	m := ready()
	m, _ = step(m, keyEnter())
	m = pickFormat(t, m, "pdf")
	m, _ = step(m, keyEnter())
	if m.stage != stageTechnique {
		t.Fatalf("stage = %v", m.stage)
	}
	m, _ = step(m, keyDown())
	m, _ = step(m, keyEnter())
	if m.technique == "" {
		t.Fatalf("technique should be set after choosing one")
	}
	m, _ = step(m, keyEsc())
	m, _ = step(m, keyEsc())
	if m.stage != stageFormat {
		t.Fatalf("back to format: stage = %v", m.stage)
	}
	m = pickFormat(t, m, "image")
	m, _ = step(m, keyEnter())
	if m.technique != "" {
		t.Fatalf("technique not cleared after switching to image: %q", m.technique)
	}
	if m.stage != stageCover {
		t.Fatalf("stage = %v", m.stage)
	}
}

func TestBackNavigation(t *testing.T) {
	m := ready()
	m, _ = step(m, keyEnter())
	if m.stage != stageFormat {
		t.Fatalf("stage = %v", m.stage)
	}
	m, _ = step(m, keyEsc())
	if m.stage != stageOperation {
		t.Fatalf("back did not return to operation: %v", m.stage)
	}
}

func TestEmptyMessageBlocks(t *testing.T) {
	cover := makePNG(t, 32, 32)
	m := ready()
	m, _ = step(m, keyEnter())
	m, _ = step(m, keyEnter())
	m, _ = step(m, fileLoadedMsg{origin: stageCover, path: "c.png", data: cover})
	m, _ = step(m, keyEnter())
	if m.stage != stagePayload {
		t.Fatalf("empty message should not advance, stage = %v", m.stage)
	}
	if m.err == nil {
		t.Fatalf("expected a validation error")
	}
}

func TestViewAllStagesNoPanic(t *testing.T) {
	cover := makePNG(t, 16, 16)
	stages := []stage{
		stageOperation, stageFormat, stageTechnique, stageCover, stagePayload,
		stageSecure, stageSave, stageReview, stagePassphrase, stageRunning, stageResult,
	}
	for _, op := range []operation{opHide, opReveal, opCapacity} {
		for _, s := range stages {
			for _, w := range []int{50, 100} {
				m := ready()
				m.width = w
				m.op = op
				m.stage = s
				m.format = "image"
				m.coverPath = "cover.png"
				m.payloadBytes = []byte("preview")
				m.payloadLabel = inlineLabel
				m.capValue = 4096
				m.envelopeSize = 200
				m.capRows = buildCapRows(engine.CapacityAll(cover))
				m.fmtPick = newPicker(formatItems(op == opReveal))
				m.techPick = newPicker(techniqueItems("pdf"))
				m.revealRes = engine.RevealResult{Format: "image", Data: []byte("hi")}
				m.hideRes = engine.HideResult{Format: "image", PayloadBytes: 7, EnvelopeBytes: 21}
				if got := m.View(); got == "" {
					t.Fatalf("empty view at op=%d stage=%d", op, s)
				}
			}
		}
	}
}

func TestQuitFromResult(t *testing.T) {
	m := ready()
	m.stage = stageResult
	_, cmd := step(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatalf("q on result should return a quit command")
	}
}
