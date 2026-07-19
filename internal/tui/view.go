/*
©AngelaMos | 2026
view.go

The render layer: layout composition and one render per wizard stage
*/

package tui

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

const (
	previewMaxLines = 6
	previewHexBytes = 32
	startingMessage = "\n  starting crypha ...\n"
	farewell        = "crypha out\n"
)

func (m Model) View() string {
	if m.quitting {
		return styleHint.Render(farewell)
	}
	if !m.ready {
		return startingMessage
	}
	inner := m.contentWidth()
	parts := []string{
		renderHeader(inner),
		"",
		renderStepper(m.stepLabels(), m.stepIndex()),
		"",
		m.stageView(inner),
	}
	if m.err != nil {
		parts = append(parts, "", styleError.Width(inner).Render(m.err.Error()))
	}
	parts = append(parts, "", renderFooter(m.footerHints()))
	framed := frameStyle(inner).Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
	return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, framed)
}

func (m Model) boxWidth() int {
	w := m.width - horizontalGutter
	if w > appMaxWidth {
		w = appMaxWidth
	}
	if w < appMinWidth {
		w = appMinWidth
	}
	return w
}

func (m Model) contentWidth() int {
	return m.boxWidth() - framePadX*2 - frameBorder*2
}

func (m Model) stageView(w int) string {
	switch m.stage {
	case stageOperation:
		return section("choose an operation", m.opPick.view(w))
	case stageFormat:
		return section(m.formatPrompt(), m.fmtPick.view(w))
	case stageTechnique:
		return section("choose a pdf technique", m.techPick.view(w))
	case stageCover:
		return section(m.coverPrompt(), m.files.View())
	case stagePayload:
		return m.payloadView()
	case stageSecure:
		return section("secure the payload", m.secure.view(w))
	case stageSave:
		return m.saveView()
	case stageReview:
		return m.reviewView(w)
	case stagePassphrase:
		return m.passphraseView()
	case stageRunning:
		return m.runningView()
	case stageResult:
		return m.resultView(w)
	}
	return ""
}

func (m Model) formatPrompt() string {
	if m.op == opReveal {
		return "choose a carrier, or auto-detect"
	}
	return "choose a carrier"
}

func (m Model) coverPrompt() string {
	if m.op == opReveal {
		return "select a stego file to inspect"
	}
	return "select a cover file"
}

func (m Model) payloadView() string {
	if m.payloadMode == payloadFile {
		body := lipgloss.JoinVertical(lipgloss.Left,
			styleHint.Render("choose a payload file  ·  tab or esc to type a message instead"),
			"",
			m.files.View())
		return section("payload from a file", body)
	}
	body := lipgloss.JoinVertical(lipgloss.Left,
		m.message.View(),
		"",
		styleHint.Render("enter continues  ·  tab loads a file instead"))
	return section("payload as an inline message", body)
}

func (m Model) saveView() string {
	body := lipgloss.JoinVertical(lipgloss.Left,
		m.outPath.View(),
		"",
		styleHint.Render("where to write the stego file  ·  enter continues"))
	return section("save the stego file as", body)
}

func (m Model) reviewView(w int) string {
	if m.op == opCapacity {
		title := "capacity for " + filepath.Base(m.coverPath)
		return section(title, renderCapacityTable(m.capRows, capBarWidth))
	}
	meter := renderHideMeter(len(m.payloadBytes), m.envelopeSize, m.capValue, m.capErr, meterBarWidth)
	body := lipgloss.JoinVertical(lipgloss.Left,
		m.hideSummary(),
		"",
		meter,
		"",
		styleHint.Render("enter to embed  ·  esc to go back"))
	return section("review", body)
}

func (m Model) hideSummary() string {
	lines := []string{
		kv("carrier", formatLabel(m.format, m.technique)),
		kv("payload", fmt.Sprintf("%s (%d B)", m.payloadLabel, len(m.payloadBytes))),
		kv("security", m.securitySummary()),
		kv("output", m.outPath.Value()),
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m Model) securitySummary() string {
	var parts []string
	if m.encrypted() {
		parts = append(parts, badge("encrypted", hexViolet),
			styleHint.Render(m.secure.cipherValue()+" · "+m.secure.strengthValue()))
	} else {
		parts = append(parts, styleHint.Render("plaintext"))
	}
	if m.secure.compress {
		parts = append(parts, badge("compressed", hexBlue))
	}
	return strings.Join(parts, " ")
}

func (m Model) runningView() string {
	verb := "embedding payload"
	if m.op == opReveal {
		verb = "revealing payload"
	}
	bar := spectralBar(embedBarWidth, m.animFrac, brandStops)
	pct := int(math.Round(m.animFrac * fullPercent))
	body := lipgloss.JoinVertical(lipgloss.Left,
		bar+"  "+styleValueBold.Render(fmt.Sprintf("%d%%", pct)),
		"",
		styleHint.Render(verb+" ..."))
	return section(verb, body)
}

func (m Model) passphraseView() string {
	prompt := "this payload is encrypted; enter the passphrase to unlock it"
	if m.passPrompts > 1 {
		prompt = "that passphrase did not work, try again"
	}
	body := lipgloss.JoinVertical(lipgloss.Left,
		styleWarn.Render(prompt),
		"",
		m.revPass.View(),
		"",
		styleHint.Render("enter to unlock  ·  esc to cancel"))
	return section("unlock", body)
}

func (m Model) resultView(w int) string {
	if m.engineErr != nil {
		return section("failed", styleError.Width(w).Render(reasonText(m.engineErr)))
	}
	if m.op == opReveal {
		return m.revealResultView(w)
	}
	return m.hideResultView()
}

func (m Model) hideResultView() string {
	r := m.hideRes
	lines := []string{
		kvStyled("status", "payload hidden", styleSuccess),
		kv("carrier", formatLabel(r.Format, r.Technique)),
		kv("payload", fmt.Sprintf("%d B", r.PayloadBytes)),
		kv("envelope", fmt.Sprintf("%d B", r.EnvelopeBytes)),
		kv("security", boolBadges(r.Encrypted, r.Compressed)),
		kv("output", m.outputAt),
	}
	body := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinVertical(lipgloss.Left, lines...),
		"",
		styleHint.Render("n new run  ·  q quit"))
	return section("done", body)
}

func (m Model) revealResultView(w int) string {
	data := m.revealRes.Data
	lines := []string{
		kvStyled("status", "payload revealed", styleSuccess),
		kv("carrier", m.revealRes.Format),
		kv("encrypted", boolText(m.revealRes.Encrypted)),
		kv("size", fmt.Sprintf("%d B", len(data))),
	}
	if m.savedAt != "" {
		lines = append(lines, kvStyled("saved", m.savedAt, styleSuccess))
	}
	segments := []string{
		lipgloss.JoinVertical(lipgloss.Left, lines...),
		"",
		revealPreview(data, w),
	}
	if m.saving {
		segments = append(segments, "",
			styleLabel.Render("save as"),
			m.outPath.View(),
			styleHint.Render("enter to write  ·  esc to cancel"))
	} else {
		segments = append(segments, "", styleHint.Render(m.revealHints()))
	}
	return section("revealed", lipgloss.JoinVertical(lipgloss.Left, segments...))
}

func (m Model) revealHints() string {
	if m.canSaveReveal() {
		return "s save to file  ·  n new run  ·  q quit"
	}
	return "n new run  ·  q quit"
}

func section(title, body string) string {
	return lipgloss.JoinVertical(lipgloss.Left, sectionTitle(title), "", body)
}

func revealPreview(data []byte, w int) string {
	if len(data) == 0 {
		return styleHint.Render("(empty payload)")
	}
	if isPrintable(data) {
		text := lipgloss.NewStyle().Foreground(lipgloss.Color(hexBright)).Width(w).MaxHeight(previewMaxLines).Render(string(data))
		return lipgloss.JoinVertical(lipgloss.Left, styleLabel.Render("message"), text)
	}
	return lipgloss.JoinVertical(lipgloss.Left, styleLabel.Render("hex preview"), styleValue.Width(w).Render(hexPreview(data, previewHexBytes)))
}

func formatLabel(format, technique string) string {
	if format == "" {
		return "auto-detect"
	}
	if technique == "" {
		return format
	}
	return format + " (" + technique + ")"
}

func boolText(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func boolBadges(encrypted, compressed bool) string {
	var parts []string
	if encrypted {
		parts = append(parts, badge("encrypted", hexViolet))
	} else {
		parts = append(parts, styleHint.Render("plaintext"))
	}
	if compressed {
		parts = append(parts, badge("compressed", hexBlue))
	}
	return strings.Join(parts, " ")
}

func isPrintable(data []byte) bool {
	if !utf8.Valid(data) {
		return false
	}
	for _, r := range string(data) {
		if r == '\n' || r == '\t' || r == '\r' {
			continue
		}
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

func hexPreview(data []byte, n int) string {
	truncated := false
	if len(data) > n {
		data = data[:n]
		truncated = true
	}
	s := fmt.Sprintf("% x", data)
	if truncated {
		s += " ..."
	}
	return s
}
