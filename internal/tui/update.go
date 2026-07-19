/*
©AngelaMos | 2026
update.go

Message dispatch, per-stage key handling, and async engine result wiring
*/

package tui

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/CarterPerez-dev/crypha/internal/payload"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.onResize(msg)
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
		return m.onKey(msg)
	case fileLoadedMsg:
		return m.onFileLoaded(msg)
	case hideDoneMsg:
		return m.onHideDone(msg)
	case revealDoneMsg:
		return m.onRevealDone(msg)
	case savedMsg:
		return m.onSaved(msg)
	case tickMsg:
		return m.onTick()
	default:
		return m.forward(msg)
	}
}

func (m Model) onResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	m.ready = true
	var cmd tea.Cmd
	m.files, cmd = m.files.Update(msg)
	return m, cmd
}

func (m Model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.err = nil
	switch m.stage {
	case stageOperation:
		return m.keyOperation(msg)
	case stageFormat:
		return m.keyFormat(msg)
	case stageTechnique:
		return m.keyTechnique(msg)
	case stageCover:
		return m.keyCover(msg)
	case stagePayload:
		return m.keyPayload(msg)
	case stageSecure:
		return m.keySecure(msg)
	case stageSave:
		return m.keySave(msg)
	case stageReview:
		return m.keyReview(msg)
	case stagePassphrase:
		return m.keyPassphrase(msg)
	case stageResult:
		return m.keyResult(msg)
	}
	return m, nil
}

func (m Model) keyOperation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "up", "k":
		m.opPick.up()
	case "down", "j":
		m.opPick.down()
	case "enter":
		m.op = operation(m.opPick.cursor)
		return m.advance()
	}
	return m, nil
}

func (m Model) keyFormat(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		return m.back()
	case "up", "k":
		m.fmtPick.up()
	case "down", "j":
		m.fmtPick.down()
	case "enter":
		m.format = m.fmtPick.selected().value
		m.technique = ""
		return m.advance()
	}
	return m, nil
}

func (m Model) keyTechnique(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		return m.back()
	case "up", "k":
		m.techPick.up()
	case "down", "j":
		m.techPick.down()
	case "enter":
		m.technique = m.techPick.selected().value
		return m.advance()
	}
	return m, nil
}

func (m Model) keyCover(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		return m.back()
	}
	var cmd tea.Cmd
	m.files, cmd = m.files.Update(msg)
	if ok, path := m.files.DidSelectFile(msg); ok {
		return m, loadFileCmd(stageCover, path)
	}
	return m, cmd
}

func (m Model) keyPayload(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.payloadMode == payloadFile {
		switch msg.String() {
		case "esc", "tab":
			m.payloadMode = payloadMessage
			return m, m.message.Focus()
		}
		var cmd tea.Cmd
		m.files, cmd = m.files.Update(msg)
		if ok, path := m.files.DidSelectFile(msg); ok {
			return m, loadFileCmd(stagePayload, path)
		}
		return m, cmd
	}
	switch msg.String() {
	case "esc":
		return m.back()
	case "tab":
		m.payloadMode = payloadFile
		m.message.Blur()
		return m, m.files.Init()
	case "enter":
		if strings.TrimSpace(m.message.Value()) == "" {
			m.err = errNeedPayload
			return m, nil
		}
		m.payloadBytes = []byte(m.message.Value())
		m.payloadLabel = inlineLabel
		return m.advance()
	}
	var cmd tea.Cmd
	m.message, cmd = m.message.Update(msg)
	return m, cmd
}

func (m Model) keySecure(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m.back()
	case "enter":
		if m.secure.encrypt && len(m.secure.passphrase()) == 0 {
			m.err = errNeedPassphrase
			return m, nil
		}
		m.pass = m.secure.passphrase()
		return m.advance()
	}
	var cmd tea.Cmd
	m.secure, cmd = m.secure.update(msg)
	return m, cmd
}

func (m Model) keySave(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m.back()
	case "enter":
		if strings.TrimSpace(m.outPath.Value()) == "" {
			m.err = errNeedOutput
			return m, nil
		}
		return m.advance()
	}
	var cmd tea.Cmd
	m.outPath, cmd = m.outPath.Update(msg)
	return m, cmd
}

func (m Model) keyReview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		return m.back()
	case "n":
		return m.reset()
	}
	if m.op == opCapacity {
		return m, nil
	}
	if msg.String() == "enter" {
		if !m.hideFits() {
			m.err = errPayloadTooBig
			return m, nil
		}
		return m.advance()
	}
	return m, nil
}

func (m Model) keyPassphrase(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.revPass.Blur()
		cmd := m.enter(stageCover)
		return m, cmd
	case "enter":
		return m, m.enter(stageRunning)
	}
	var cmd tea.Cmd
	m.revPass, cmd = m.revPass.Update(msg)
	return m, cmd
}

func (m Model) keyResult(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.saving {
		return m.keyResultSaving(msg)
	}
	switch msg.String() {
	case "q", "esc", "enter":
		m.quitting = true
		return m, tea.Quit
	case "n":
		return m.reset()
	case "s":
		if m.canSaveReveal() {
			m.saving = true
			m.outPath.SetValue(m.suggestReveal())
			return m, m.outPath.Focus()
		}
	}
	return m, nil
}

func (m Model) keyResultSaving(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.saving = false
		m.outPath.Blur()
		return m, nil
	case "enter":
		if strings.TrimSpace(m.outPath.Value()) == "" {
			m.err = errNeedOutput
			return m, nil
		}
		return m, saveCmd(strings.TrimSpace(m.outPath.Value()), m.revealRes.Data)
	}
	var cmd tea.Cmd
	m.outPath, cmd = m.outPath.Update(msg)
	return m, cmd
}

func (m Model) onFileLoaded(msg fileLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		return m, nil
	}
	switch msg.origin {
	case stageCover:
		m.coverPath = msg.path
		m.coverBytes = msg.data
		m.passPrompts = 0
		return m.advance()
	case stagePayload:
		if len(msg.data) == 0 {
			m.err = errEmptyFile
			return m, nil
		}
		m.payloadBytes = msg.data
		m.payloadLabel = filepath.Base(msg.path)
		m.payloadMode = payloadMessage
		return m.advance()
	}
	return m, nil
}

func (m Model) onHideDone(msg hideDoneMsg) (tea.Model, tea.Cmd) {
	m.engineDone = true
	if msg.err != nil {
		m.engineErr = msg.err
	} else {
		m.hideRes = msg.res
		m.stego = msg.data
		m.outputAt = m.outPath.Value()
	}
	return m.maybeFinish()
}

func (m Model) onRevealDone(msg revealDoneMsg) (tea.Model, tea.Cmd) {
	m.engineDone = true
	m.revealRes = msg.res
	m.engineErr = msg.err
	return m.maybeFinish()
}

func (m Model) onSaved(msg savedMsg) (tea.Model, tea.Cmd) {
	m.saving = false
	m.outPath.Blur()
	if msg.err != nil {
		m.err = msg.err
		return m, nil
	}
	m.savedAt = msg.path
	return m, nil
}

func (m Model) onTick() (tea.Model, tea.Cmd) {
	if m.stage != stageRunning {
		return m, nil
	}
	m.animFrac += animStep
	if m.animFrac >= 1 {
		m.animFrac = 1
		return m.maybeFinish()
	}
	return m, tick()
}

func (m Model) maybeFinish() (tea.Model, tea.Cmd) {
	if m.stage != stageRunning || !m.engineDone || m.animFrac < 1 {
		return m, nil
	}
	if m.op == opReveal && needsPassphrase(m.engineErr) && m.passPrompts < maxRevealPrompts {
		m.passPrompts++
		m.revPass.Reset()
		m.stage = stagePassphrase
		return m, m.revPass.Focus()
	}
	m.stage = stageResult
	return m, nil
}

func (m Model) forward(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.stage {
	case stageCover:
		m.files, cmd = m.files.Update(msg)
	case stagePayload:
		if m.payloadMode == payloadFile {
			m.files, cmd = m.files.Update(msg)
		} else {
			m.message, cmd = m.message.Update(msg)
		}
	case stageSecure:
		m.secure, cmd = m.secure.update(msg)
	case stageSave:
		m.outPath, cmd = m.outPath.Update(msg)
	case stagePassphrase:
		m.revPass, cmd = m.revPass.Update(msg)
	case stageResult:
		if m.saving {
			m.outPath, cmd = m.outPath.Update(msg)
		}
	}
	return m, cmd
}

func (m Model) canSaveReveal() bool {
	return m.op == opReveal && m.engineErr == nil && len(m.revealRes.Data) > 0
}

func needsPassphrase(err error) bool {
	return errors.Is(err, payload.ErrPassphraseRequired) || errors.Is(err, payload.ErrDecrypt)
}
