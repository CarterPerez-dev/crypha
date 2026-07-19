/*
©AngelaMos | 2026
secure.go

The encryption sub-form: a focusable list of toggle, choice, and secret controls
*/

package tui

import (
	"strings"

	"github.com/CarterPerez-dev/crypha/internal/payload"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type secField int

const (
	secEncrypt secField = iota
	secPass
	secCipher
	secStrength
	secCompress
)

var (
	cipherChoices   = []string{string(payload.CipherChaCha20), string(payload.CipherAES256GCM)}
	strengthChoices = []string{string(payload.StrengthDefault), string(payload.StrengthHigh)}
)

type secureForm struct {
	encrypt  bool
	pass     textinput.Model
	cipher   int
	strength int
	compress bool
	focus    int
}

func newSecureForm() secureForm {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "passphrase"
	ti.EchoMode = textinput.EchoPassword
	ti.CharLimit = passLimit
	ti.Width = passFieldWidth
	return secureForm{pass: ti}
}

func (f secureForm) fields() []secField {
	if f.encrypt {
		return []secField{secEncrypt, secPass, secCipher, secStrength, secCompress}
	}
	return []secField{secEncrypt, secCompress}
}

func (f secureForm) focusField() secField {
	fs := f.fields()
	i := f.focus
	if i < 0 {
		i = 0
	}
	if i >= len(fs) {
		i = len(fs) - 1
	}
	return fs[i]
}

func (f *secureForm) move(delta int) {
	n := len(f.fields())
	f.focus += delta
	if f.focus < 0 {
		f.focus = 0
	}
	if f.focus >= n {
		f.focus = n - 1
	}
	f.syncFocus()
}

func (f *secureForm) syncFocus() tea.Cmd {
	if f.focusField() == secPass {
		return f.pass.Focus()
	}
	f.pass.Blur()
	return nil
}

func (f secureForm) cipherValue() string {
	return cipherChoices[f.cipher]
}

func (f secureForm) strengthValue() string {
	return strengthChoices[f.strength]
}

func (f secureForm) passphrase() []byte {
	if !f.encrypt {
		return nil
	}
	v := f.pass.Value()
	if v == "" {
		return nil
	}
	return []byte(v)
}

func (f secureForm) update(msg tea.Msg) (secureForm, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		f.pass, cmd = f.pass.Update(msg)
		return f, cmd
	}
	switch km.String() {
	case "up", "shift+tab":
		cmd := f.moveWith(-1)
		return f, cmd
	case "down", "tab":
		cmd := f.moveWith(1)
		return f, cmd
	}
	switch f.focusField() {
	case secEncrypt:
		if isToggleKey(km) {
			f.encrypt = !f.encrypt
			f.syncFocus()
		}
	case secCompress:
		if isToggleKey(km) {
			f.compress = !f.compress
		}
	case secCipher:
		f.cipher = cycleChoice(f.cipher, len(cipherChoices), km)
	case secStrength:
		f.strength = cycleChoice(f.strength, len(strengthChoices), km)
	case secPass:
		var cmd tea.Cmd
		f.pass, cmd = f.pass.Update(msg)
		return f, cmd
	}
	return f, nil
}

func (f *secureForm) moveWith(delta int) tea.Cmd {
	f.move(delta)
	return f.syncFocus()
}

func isToggleKey(km tea.KeyMsg) bool {
	switch km.String() {
	case " ", "left", "right":
		return true
	}
	return false
}

func cycleChoice(idx, n int, km tea.KeyMsg) int {
	switch km.String() {
	case "left":
		return (idx - 1 + n) % n
	case "right", " ":
		return (idx + 1) % n
	}
	return idx
}

func (f secureForm) view(width int) string {
	rows := make([]string, 0, len(f.fields()))
	for i, fld := range f.fields() {
		rows = append(rows, f.fieldRow(fld, i == f.focus))
	}
	note := styleHint.Width(width).Render("up/down move  ·  space or left/right change  ·  enter continues")
	return lipgloss.JoinVertical(lipgloss.Left, lipgloss.JoinVertical(lipgloss.Left, rows...), "", note)
}

func (f secureForm) fieldRow(fld secField, focused bool) string {
	prefix := "  "
	labelStyle := styleLabel
	if focused {
		prefix = styleCursor.Render(accentBar) + " "
		labelStyle = styleValueBold
	}
	label, control := f.labelControl(fld)
	return prefix + labelStyle.Width(labelWidth).Render(label) + control
}

func (f secureForm) labelControl(fld secField) (string, string) {
	switch fld {
	case secEncrypt:
		return "encrypt", toggleControl(f.encrypt)
	case secPass:
		return "passphrase", f.pass.View()
	case secCipher:
		return "cipher", choiceControl(cipherChoices, f.cipher)
	case secStrength:
		return "key strength", choiceControl(strengthChoices, f.strength)
	case secCompress:
		return "compress", toggleControl(f.compress)
	}
	return "", ""
}

func toggleControl(on bool) string {
	return pill("off", !on) + " " + pill("on", on)
}

func choiceControl(choices []string, idx int) string {
	parts := make([]string, len(choices))
	for i, c := range choices {
		parts[i] = pill(c, i == idx)
	}
	return strings.Join(parts, " ")
}

func pill(text string, active bool) string {
	if active {
		return badge(text, hexViolet)
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(hexFaint)).Padding(0, 1).Render(text)
}
