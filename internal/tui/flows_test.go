/*
©AngelaMos | 2026
flows_test.go

Coverage for the file-mode payload, save flow, reset, and command plumbing
*/

package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type nopMsg struct{}

func hideStego(t *testing.T, secret string) []byte {
	t.Helper()
	out := filepath.Join(t.TempDir(), "s.png")
	cover := makePNG(t, 64, 64)
	m := ready()
	m, _ = step(m, keyEnter())
	m = pickFormat(t, m, "image")
	m, _ = step(m, keyEnter())
	m, _ = step(m, fileLoadedMsg{origin: stageCover, path: "c.png", data: cover})
	m, _ = step(m, typeText(secret))
	m, _ = step(m, keyEnter())
	m, _ = step(m, keyEnter())
	m.outPath.SetValue(out)
	m, _ = step(m, keyEnter())
	m, cmd := step(m, keyEnter())
	m = finishRun(t, m, cmd)
	if m.engineErr != nil {
		t.Fatalf("hide errored: %v", m.engineErr)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read stego: %v", err)
	}
	return data
}

func TestInitReturnsCmd(t *testing.T) {
	if New().Init() == nil {
		t.Fatalf("Init returned nil")
	}
}

func TestLoadFileCmd(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(p, []byte("hi"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	msg, ok := loadFileCmd(stageCover, p)().(fileLoadedMsg)
	if !ok || msg.err != nil || string(msg.data) != "hi" || msg.origin != stageCover {
		t.Fatalf("loadFileCmd = %+v ok=%v", msg, ok)
	}
	miss, _ := loadFileCmd(stageCover, filepath.Join(t.TempDir(), "nope"))().(fileLoadedMsg)
	if miss.err == nil {
		t.Fatalf("expected an error for a missing file")
	}
}

func TestPayloadFileMode(t *testing.T) {
	cover := makePNG(t, 48, 48)
	m := ready()
	m, _ = step(m, keyEnter())
	m = pickFormat(t, m, "image")
	m, _ = step(m, keyEnter())
	m, _ = step(m, fileLoadedMsg{origin: stageCover, path: "c.png", data: cover})
	if m.stage != stagePayload {
		t.Fatalf("stage = %v", m.stage)
	}
	m, _ = step(m, keyTab())
	if m.payloadMode != payloadFile {
		t.Fatalf("tab did not switch to file mode")
	}
	m, _ = step(m, fileLoadedMsg{origin: stagePayload, path: "/x/secret.bin", data: []byte("payloaddata")})
	if m.stage != stageSecure {
		t.Fatalf("after payload file: stage = %v", m.stage)
	}
	if m.payloadLabel != "secret.bin" {
		t.Fatalf("payload label = %q", m.payloadLabel)
	}
	if string(m.payloadBytes) != "payloaddata" {
		t.Fatalf("payload bytes = %q", m.payloadBytes)
	}
}

func TestRevealSaveFlow(t *testing.T) {
	stego := hideStego(t, "save me")
	saveTo := filepath.Join(t.TempDir(), "recovered.txt")

	r := ready()
	r, _ = step(r, keyDown())
	r, _ = step(r, keyEnter())
	r, _ = step(r, keyEnter())
	r, cmd := step(r, fileLoadedMsg{origin: stageCover, path: "s.png", data: stego})
	r = finishRun(t, r, cmd)
	if r.stage != stageResult || r.engineErr != nil {
		t.Fatalf("reveal: stage=%v err=%v", r.stage, r.engineErr)
	}
	if !r.canSaveReveal() {
		t.Fatalf("revealed payload should be saveable")
	}
	r, _ = step(r, typeText("s"))
	if !r.saving {
		t.Fatalf("s did not enter saving mode")
	}
	r.outPath.SetValue(saveTo)
	r, cmd = step(r, keyEnter())
	for _, msg := range drain(cmd) {
		r, _ = step(r, msg)
	}
	if r.savedAt != saveTo {
		t.Fatalf("savedAt = %q, want %q", r.savedAt, saveTo)
	}
	got, err := os.ReadFile(saveTo)
	if err != nil || string(got) != "save me" {
		t.Fatalf("saved file = %q err=%v", got, err)
	}
}

func TestResetOnNewRun(t *testing.T) {
	m := ready()
	m.stage = stageResult
	m, _ = step(m, typeText("n"))
	if m.stage != stageOperation {
		t.Fatalf("new run did not reset, stage = %v", m.stage)
	}
}

func TestCoverKeyForwarding(t *testing.T) {
	m := ready()
	m, _ = step(m, keyEnter())
	m = pickFormat(t, m, "image")
	m, _ = step(m, keyEnter())
	if m.stage != stageCover {
		t.Fatalf("stage = %v", m.stage)
	}
	browse, _ := step(m, keyDown())
	if browse.stage != stageCover {
		t.Fatalf("browsing changed the stage to %v", browse.stage)
	}
	back, _ := step(m, keyEsc())
	if back.stage != stageFormat {
		t.Fatalf("esc from cover went to %v", back.stage)
	}
}

func TestCapacityReviewKeys(t *testing.T) {
	cover := makePNG(t, 32, 32)
	m := ready()
	m, _ = step(m, keyDown())
	m, _ = step(m, keyDown())
	m, _ = step(m, keyEnter())
	m, _ = step(m, fileLoadedMsg{origin: stageCover, path: "c.png", data: cover})
	if m.stage != stageReview {
		t.Fatalf("stage = %v", m.stage)
	}
	back, _ := step(m, keyEsc())
	if back.stage != stageCover {
		t.Fatalf("esc from capacity review went to %v", back.stage)
	}
	fresh, _ := step(m, typeText("n"))
	if fresh.stage != stageOperation {
		t.Fatalf("n from capacity review went to %v", fresh.stage)
	}
}

func TestForwardNonKeyMsgs(t *testing.T) {
	for _, s := range []stage{stageCover, stagePayload, stageSecure, stageSave, stagePassphrase, stageResult} {
		m := ready()
		m.stage = s
		m.saving = s == stageResult
		m, _ = step(m, nopMsg{})
		if got := m.View(); got == "" {
			t.Fatalf("empty view after nop msg at stage %v", s)
		}
	}
}

func TestPassPromptsResetOnCoverLoad(t *testing.T) {
	m := ready()
	m.op = opReveal
	m.stage = stageCover
	m.passPrompts = 2
	m, _ = step(m, fileLoadedMsg{origin: stageCover, path: "x", data: []byte("not a stego")})
	if m.passPrompts != 0 {
		t.Fatalf("passPrompts = %d after loading a fresh file, want 0", m.passPrompts)
	}
}

func TestHideFitsUsesCompressedSize(t *testing.T) {
	cover := makePNG(t, 16, 16)
	big := strings.Repeat("A", 200)

	m := ready()
	m, _ = step(m, keyEnter())
	m = pickFormat(t, m, "image")
	m, _ = step(m, keyEnter())
	m, _ = step(m, fileLoadedMsg{origin: stageCover, path: "c.png", data: cover})
	m, _ = step(m, typeText(big))
	m, _ = step(m, keyEnter())
	m, _ = step(m, keyDown())
	m, _ = step(m, keySpace())
	if !m.secure.compress {
		t.Fatalf("compression not enabled")
	}
	m, _ = step(m, keyEnter())
	m.outPath.SetValue(filepath.Join(t.TempDir(), "o.png"))
	m, _ = step(m, keyEnter())
	if m.stage != stageReview {
		t.Fatalf("stage = %v", m.stage)
	}
	if len(big)+14 <= m.capValue {
		t.Fatalf("precondition broken: raw payload already fits (cap=%d)", m.capValue)
	}
	if !m.hideFits() {
		t.Fatalf("compressible payload should fit: envelope=%d cap=%d", m.envelopeSize, m.capValue)
	}
}
